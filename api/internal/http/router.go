package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
	"github.com/ClairCrest/asr-platform/api/internal/job"
	"github.com/ClairCrest/asr-platform/api/internal/observability"
)

type Deps struct {
	Logger             *slog.Logger
	AuthSvc            *auth.Service
	Tokens             *auth.TokenIssuer
	JobSvc             *job.Service
	Objects            ObjectPresigner
	AudioPresigner     AudioPresigner
	HealthCheck        map[string]Checker
	WSHandler          http.Handler
	CORSAllowedOrigins []string
}

// NewRouter assembles the full HTTP surface described in the build plan's
// API surface section, under the /api/v1 base path plus unversioned
// /healthz and /readyz probes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(observability.RequestIDMiddleware)
	r.Use(observability.RequestLogger(d.Logger))
	r.Use(observability.Recoverer(d.Logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-API-Key", "Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	authHandler := NewAuthHandler(d.AuthSvc)
	jobHandler := NewJobHandler(d.JobSvc, d.AudioPresigner)
	uploadHandler := NewUploadHandler(d.Objects)
	healthHandler := NewHealthHandler(d.HealthCheck)
	wsTicketHandler := NewWSTicketHandler(d.Tokens)

	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)

	// GET /ws authenticates itself via the ticket query param (see
	// WSTicketHandler), not the bearer/API-key middleware, since a
	// browser's WebSocket API cannot set an Authorization header.
	r.Get("/ws", d.WSHandler.ServeHTTP)

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

			r.Post("/ws-ticket", wsTicketHandler.CreateTicket)

			r.Post("/uploads", uploadHandler.CreateUpload)

			r.Post("/jobs", jobHandler.Create)
			r.Get("/jobs", jobHandler.List)
			r.Get("/jobs/{id}", jobHandler.Get)
			r.Post("/jobs/{id}/cancel", jobHandler.Cancel)
			r.Post("/jobs/{id}/retry", jobHandler.Retry)
			r.Delete("/jobs/{id}", jobHandler.Delete)
			r.Get("/jobs/{id}/transcript", jobHandler.GetTranscript)
		})
	})

	return r
}
