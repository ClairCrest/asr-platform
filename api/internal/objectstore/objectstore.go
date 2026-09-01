// Package objectstore wraps the MinIO (S3-compatible) client used to
// presign audio uploads and to remove objects when a job is deleted.
package objectstore

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const presignTTL = 15 * time.Minute

type Client struct {
	mc     *minio.Client // internal endpoint: real operations (bucket checks, deletes)
	signer *minio.Client // public endpoint: signs URLs only, never dials out
	bucket string
}

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	// PublicEndpoint is the host presigned URLs are signed against — see
	// config.Config.S3PublicEndpoint for why this can differ from
	// Endpoint. Presigning is a pure local signature computation with no
	// network round trip, so a client built against an unreachable-from-
	// here endpoint works fine for it.
	PublicEndpoint string
}

// New connects to MinIO and ensures the configured bucket exists, creating
// it if this is a fresh instance.
// minioRegion is MinIO's own default server region (used when
// MINIO_REGION/MINIO_SITE_REGION isn't set, true of every deployment in
// this repo). Pinning it on the client avoids minio-go silently doing a
// GetBucketLocation lookup the first time it needs a region to sign a
// request — a real network call that, on the presigning client, targets
// S3PublicEndpoint and fails outright when that host isn't reachable
// from wherever the API is actually running (e.g. inside a pod, where
// "localhost" means the pod itself, not the host the Ingress answers on).
const minioRegion = "us-east-1"

func New(ctx context.Context, cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: minioRegion,
	})
	if err != nil {
		return nil, fmt.Errorf("objectstore: create client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("objectstore: check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: minioRegion}); err != nil {
			return nil, fmt.Errorf("objectstore: create bucket: %w", err)
		}
	}

	signer := mc
	if cfg.PublicEndpoint != cfg.Endpoint {
		signer, err = minio.New(cfg.PublicEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure: cfg.UseSSL,
			Region: minioRegion,
		})
		if err != nil {
			return nil, fmt.Errorf("objectstore: create presigning client: %w", err)
		}
	}

	return &Client{mc: mc, signer: signer, bucket: cfg.Bucket}, nil
}

// PresignPutURL returns a time-limited URL the client can PUT an audio file
// to directly, without the API proxying the upload body.
func (c *Client) PresignPutURL(ctx context.Context, objectKey string) (string, error) {
	u, err := c.signer.PresignedPutObject(ctx, c.bucket, objectKey, presignTTL)
	if err != nil {
		return "", fmt.Errorf("objectstore: presign put: %w", err)
	}
	return u.String(), nil
}

// PresignGetURL returns a time-limited URL the dashboard's audio player
// can stream the source file from directly, without the API proxying the
// audio body.
func (c *Client) PresignGetURL(ctx context.Context, objectKey string) (string, error) {
	u, err := c.signer.PresignedGetObject(ctx, c.bucket, objectKey, presignTTL, nil)
	if err != nil {
		return "", fmt.Errorf("objectstore: presign get: %w", err)
	}
	return u.String(), nil
}

// Ping verifies the configured bucket is reachable, for use by /readyz.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("objectstore: ping: %w", err)
	}
	return nil
}

// DeleteObject removes objectKey from the bucket. Called when a job is
// soft-deleted to reclaim storage.
func (c *Client) DeleteObject(ctx context.Context, objectKey string) error {
	if err := c.mc.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("objectstore: delete object: %w", err)
	}
	return nil
}
