// Command api runs the ASR platform control plane: auth, uploads, jobs,
// and the queue producer. It never loads a transcription model itself.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/config"
	httpapi "github.com/ClairCrest/asr-platform/api/internal/http"
	"github.com/ClairCrest/asr-platform/api/internal/job"
	"github.com/ClairCrest/asr-platform/api/internal/metrics"
	"github.com/ClairCrest/asr-platform/api/internal/objectstore"
	"github.com/ClairCrest/asr-platform/api/internal/observability"
	"github.com/ClairCrest/asr-platform/api/internal/queue"
	"github.com/ClairCrest/asr-platform/api/internal/store"
	"github.com/ClairCrest/asr-platform/api/internal/ws"
)

func main() {
	logger := observability.NewLogger()
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}

	objects, err := objectstore.New(ctx, objectstore.Config{
		Endpoint:       cfg.S3Endpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		Bucket:         cfg.S3Bucket,
		UseSSL:         cfg.S3UseSSL,
		PublicEndpoint: cfg.S3PublicEndpoint,
	})
	if err != nil {
		return err
	}

	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authSvc := auth.NewService(store.NewUserStore(pool), store.NewAPIKeyStore(pool), tokens)

	producer := queue.NewProducer(rdb)
	jobSvc := job.NewService(store.NewJobStore(pool), producer, objects)

	reaper := queue.NewReaper(store.NewLeaseStore(pool), producer, logger)
	go reaper.Run(ctx, 15*time.Second)

	hub := ws.NewHub()
	listener := ws.NewListener(pool, hub, logger)
	go listener.Run(ctx)
	wsHandler := ws.NewHandler(hub, tokens, logger, cfg.CORSAllowedOrigins)

	metricsRegistry := prometheus.NewRegistry()
	metrics.Register(metricsRegistry)
	metricsCollector := queue.NewMetricsCollector(rdb, logger)
	go metricsCollector.Run(ctx, 10*time.Second)

	router := httpapi.NewRouter(httpapi.Deps{
		Logger:             logger,
		AuthSvc:            authSvc,
		Tokens:             tokens,
		JobSvc:             jobSvc,
		Objects:            objects,
		AudioPresigner:     objects,
		WSHandler:          wsHandler,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		HealthCheck: map[string]httpapi.Checker{
			"postgres": func(ctx context.Context) error { return pool.Ping(ctx) },
			"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
			"minio":    objects.Ping,
		},
	})

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// A separate port for /metrics, per PLAN.md section 4: scraping
	// shouldn't share a listener with real traffic's auth/rate-limit
	// middleware, and a slow scrape shouldn't compete with request
	// handling for the same server's connection limits.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}))
	metricsSrv := &http.Server{
		Addr:         ":" + cfg.MetricsPort,
		Handler:      metricsMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("asr-platform api starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		logger.Info("asr-platform metrics starting", "port", cfg.MetricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stop:
		logger.Info("shutting down")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = metricsSrv.Shutdown(shutdownCtx)
	return srv.Shutdown(shutdownCtx)
}
