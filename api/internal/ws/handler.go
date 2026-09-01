package ws

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
)

const writeTimeout = 5 * time.Second

// Handler upgrades GET /ws to a WebSocket, authenticated by a short-lived
// ticket (from POST /ws-ticket) in the "ticket" query parameter — a
// browser's WebSocket API cannot set an Authorization header, so the
// ticket exists to cross that gap.
type Handler struct {
	hub    *Hub
	tokens *auth.TokenIssuer
	logger *slog.Logger
}

func NewHandler(hub *Hub, tokens *auth.TokenIssuer, logger *slog.Logger) *Handler {
	return &Handler{hub: hub, tokens: tokens, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	claims, err := h.tokens.Verify(ticket, auth.TokenTypeWSTicket)
	if err != nil {
		http.Error(w, "invalid or expired ws ticket", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.logger.Warn("ws: accept failed", "error", err)
		return
	}

	client := h.hub.Register(claims.UserID)
	defer h.hub.Unregister(client)

	ctx := r.Context()
	go h.writeLoop(ctx, conn, client)
	h.readLoop(ctx, conn)

	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// writeLoop drains the client's send channel onto the connection until it
// closes (on unregister) or the connection's context is done.
func (h *Handler) writeLoop(ctx context.Context, conn *websocket.Conn, client *Client) {
	for msg := range client.Send {
		writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
		err := conn.Write(writeCtx, websocket.MessageText, msg)
		cancel()
		if err != nil {
			return
		}
	}
}

// readLoop's only job is to detect the client going away — the dashboard
// never sends anything meaningful over this socket — and to apply
// backpressure per the websocket package's own requirements.
func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}
