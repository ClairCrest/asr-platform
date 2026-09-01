package httpapi

import (
	"context"
	"net/http"
)

// Checker pings one dependency and returns a non-nil error if it is
// unreachable. /readyz runs one per dependency; /healthz runs none.
type Checker func(ctx context.Context) error

type HealthHandler struct {
	checks map[string]Checker
}

func NewHealthHandler(checks map[string]Checker) *HealthHandler {
	return &HealthHandler{checks: checks}
}

// Healthz reports liveness only: the process is running and able to serve
// HTTP. It never touches a dependency, so a slow database cannot cause a
// container to be killed for being "unhealthy".
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports readiness: every dependency in checks must respond.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	results := make(map[string]string, len(h.checks))
	allOK := true
	for name, check := range h.checks {
		if err := check(r.Context()); err != nil {
			results[name] = err.Error()
			allOK = false
			continue
		}
		results[name] = "ok"
	}

	status := http.StatusOK
	if !allOK {
		status = http.StatusServiceUnavailable
	}
	WriteJSON(w, status, map[string]any{"status": results})
}
