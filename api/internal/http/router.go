package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
	"github.com/ClairCrest/asr-platform/api/internal/job"
	"github.com/ClairCrest/asr-platform/api/internal/observability"
)

type Deps struct {
	Logger      *slog.Logger
	AuthSvc     *auth.Service
	Tokens      *auth.TokenIssuer
	JobSvc      *job.Service
	Objects     ObjectPresigner
	HealthCheck map[string]Checker
}

// NewRouter assembles the full HTTP surface described in the build plan's
// API surface section, under the /api/v1 base path plus unversioned
// /healthz and /readyz probes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(observability.RequestIDMiddleware)
	r.Use(observability.RequestLogger(d.Logger))
	r.Use(observability.Recoverer(d.Logger))

	authHandler := NewAuthHandler(d.AuthSvc)
	jobHandler := NewJobHandler(d.JobSvc)
	uploadHandler := NewUploadHandler(d.Objects)
	healthHandler := NewHealthHandler(d.HealthCheck)

	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)

	requireAuth := middleware.RequireAuth(d.Tokens, d.AuthSvc, WriteError)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(requireAuth)

			r.Get("/me", authHandler.Me)

			r.Post("/api-keys", authHandler.CreateAPIKey)
			r.Get("/api-keys", authHandler.ListAPIKeys)
			r.Delete("/api-keys/{id}", authHandler.RevokeAPIKey)

			r.Post("/uploads", uploadHandler.CreateUpload)

			r.Post("/jobs", jobHandler.Create)
			r.Get("/jobs", jobHandler.List)
			r.Get("/jobs/{id}", jobHandler.Get)
			r.Post("/jobs/{id}/cancel", jobHandler.Cancel)
			r.Post("/jobs/{id}/retry", jobHandler.Retry)
			r.Delete("/jobs/{id}", jobHandler.Delete)
		})
	})

	return r
}
