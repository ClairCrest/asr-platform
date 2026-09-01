package httpapi

import (
	"net/http"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/http/dto"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
)

type WSTicketHandler struct {
	tokens *auth.TokenIssuer
}

func NewWSTicketHandler(tokens *auth.TokenIssuer) *WSTicketHandler {
	return &WSTicketHandler{tokens: tokens}
}

// CreateTicket issues a short-lived, single-purpose token the client
// immediately exchanges for a WebSocket connection at GET /ws?ticket=...,
// since a browser's WebSocket API cannot set an Authorization header.
func (h *WSTicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	ticket, err := h.tokens.IssueWSTicket(userID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not create ws ticket")
		return
	}
	WriteJSON(w, http.StatusOK, dto.WSTicketResponse{Ticket: ticket})
}
