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
	mc     *minio.Client
	bucket string
}

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// New connects to MinIO and ensures the configured bucket exists, creating
// it if this is a fresh instance.
func New(ctx context.Context, cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("objectstore: create client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("objectstore: check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("objectstore: create bucket: %w", err)
		}
	}

	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// PresignPutURL returns a time-limited URL the client can PUT an audio file
// to directly, without the API proxying the upload body.
func (c *Client) PresignPutURL(ctx context.Context, objectKey string) (string, error) {
	u, err := c.mc.PresignedPutObject(ctx, c.bucket, objectKey, presignTTL)
	if err != nil {
		return "", fmt.Errorf("objectstore: presign put: %w", err)
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
