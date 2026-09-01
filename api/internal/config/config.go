// Package config loads configuration from environment variables and fails
// fast at startup when a required variable is missing or malformed.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIPort     string
	MetricsPort string

	DatabaseURL string

	RedisAddr string

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool
	// S3PublicEndpoint is the host presigned upload/download URLs are
	// signed against. It must be reachable by whoever holds the URL — a
	// browser, not the API pod — which differs from S3Endpoint whenever
	// MinIO isn't itself exposed on the same address the API reaches it
	// at internally (true of every deploy/k8s environment, where the API
	// talks to MinIO over the cluster network but a browser can only
	// reach it through the Ingress). Defaults to S3Endpoint, which is
	// correct for docker-compose local dev where both are the same host.
	S3PublicEndpoint string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// CORSAllowedOrigins lets the browser-based dashboard, served from a
	// different origin than the API (nginx container vs API container in
	// every environment except a shared reverse proxy), make requests here.
	CORSAllowedOrigins []string
}

// Load reads and validates every environment variable the API needs.
// It returns an error naming the first missing or invalid variable rather
// than letting the service start in a half-configured state.
func Load() (Config, error) {
	var cfg Config
	var err error

	if cfg.APIPort, err = requireEnv("API_PORT"); err != nil {
		return Config{}, err
	}
	if cfg.MetricsPort, err = requireEnv("METRICS_PORT"); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL, err = requireEnv("DATABASE_URL"); err != nil {
		return Config{}, err
	}
	if cfg.RedisAddr, err = requireEnv("REDIS_ADDR"); err != nil {
		return Config{}, err
	}
	if cfg.S3Endpoint, err = requireEnv("S3_ENDPOINT"); err != nil {
		return Config{}, err
	}
	if cfg.S3AccessKey, err = requireEnv("S3_ACCESS_KEY"); err != nil {
		return Config{}, err
	}
	if cfg.S3SecretKey, err = requireEnv("S3_SECRET_KEY"); err != nil {
		return Config{}, err
	}
	if cfg.S3Bucket, err = requireEnv("S3_BUCKET"); err != nil {
		return Config{}, err
	}
	if cfg.S3UseSSL, err = requireEnvBool("S3_USE_SSL"); err != nil {
		return Config{}, err
	}
	cfg.S3PublicEndpoint = optionalEnv("S3_PUBLIC_ENDPOINT", cfg.S3Endpoint)
	if cfg.JWTSecret, err = requireEnv("JWT_SECRET"); err != nil {
		return Config{}, err
	}
	if cfg.AccessTokenTTL, err = requireEnvDuration("ACCESS_TOKEN_TTL"); err != nil {
		return Config{}, err
	}
	if cfg.RefreshTokenTTL, err = requireEnvDuration("REFRESH_TOKEN_TTL"); err != nil {
		return Config{}, err
	}
	origins, err := requireEnv("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return Config{}, err
	}
	cfg.CORSAllowedOrigins = strings.Split(origins, ",")

	return cfg, nil
}

func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("config: missing required env var %s", key)
	}
	return v, nil
}

// optionalEnv returns the env var if set and non-empty, else fallback.
// Used only for values with a genuinely safe default — everything else
// goes through requireEnv so a missing var fails at startup, not later.
func optionalEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnvBool(key string) (bool, error) {
	v, err := requireEnv(key)
	if err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: env var %s must be a bool: %w", key, err)
	}
	return b, nil
}

func requireEnvDuration(key string) (time.Duration, error) {
	v, err := requireEnv(key)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: env var %s must be a duration: %w", key, err)
	}
	return d, nil
}
