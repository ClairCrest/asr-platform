// Package httpapi wires the router, middleware, and handlers that make up
// the HTTP surface described in the build plan's API surface section.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ClairCrest/asr-platform/api/internal/observability"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteJSON encodes v as the JSON response body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the API's single error response shape:
// {"error": {"code", "message", "request_id"}}.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteJSON(w, status, errorBody{Error: errorDetail{
		Code:      code,
		Message:   message,
		RequestID: observability.RequestID(r.Context()),
	}})
}

// DecodeJSON decodes the request body into v, rejecting unknown fields so
// typos in client payloads surface as errors instead of being silently
// ignored.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
