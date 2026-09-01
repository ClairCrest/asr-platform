package ws

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
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
	hub            *Hub
	tokens         *auth.TokenIssuer
	logger         *slog.Logger
	originPatterns []string
}

// NewHandler builds a WebSocket handler. allowedOrigins are full origin
// URLs (e.g. "http://localhost:5173", matching CORS_ALLOWED_ORIGINS) —
// coder/websocket's own origin check is separate from the CORS middleware
// (it guards the upgrade itself, which CORS preflight never covers), so
// without this every cross-origin browser client is rejected with a 403
// even though ordinary REST calls succeed.
func NewHandler(hub *Hub, tokens *auth.TokenIssuer, logger *slog.Logger, allowedOrigins []string) *Handler {
	patterns := make([]string, 0, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			patterns = append(patterns, u.Host)
		}
	}
	return &Handler{hub: hub, tokens: tokens, logger: logger, originPatterns: patterns}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	claims, err := h.tokens.Verify(ticket, auth.TokenTypeWSTicket)
	if err != nil {
		http.Error(w, "invalid or expired ws ticket", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.originPatterns})
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
