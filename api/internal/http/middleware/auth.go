// Package middleware holds HTTP middleware for the API, currently just
// authentication. Request ID, logging, and panic recovery live in
// internal/observability since they apply below the auth layer too.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
)

type contextKey string

const userIDKey contextKey = "user_id"

// UserID returns the authenticated user's ID from ctx, set by RequireAuth.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

type errorWriter func(w http.ResponseWriter, r *http.Request, status int, code, message string)

// RequireAuth accepts either a "Bearer <access token>" Authorization header
// or an "X-API-Key" header, and rejects the request with 401 if neither
// resolves to a valid, active identity.
func RequireAuth(tokens *auth.TokenIssuer, authSvc *auth.Service, writeError errorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				userID, err := authSvc.AuthenticateAPIKey(r.Context(), apiKey)
				if err != nil {
					writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid api key")
					return
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
				return
			}

			authHeader := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || token == "" {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing bearer token or api key")
				return
			}

			claims, err := tokens.Verify(token, auth.TokenTypeAccess)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, claims.UserID)))
		})
	}
}
