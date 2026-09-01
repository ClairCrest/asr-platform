package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const reconnectDelay = 2 * time.Second

// notification mirrors the JSON object built by the notify_job_event()
// trigger (migration 000008): job id, owning user id, event type, the
// event's own payload, and its timestamp.
type notification struct {
	JobID     uuid.UUID       `json:"job_id"`
	UserID    uuid.UUID       `json:"user_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// Listener holds a dedicated Postgres connection LISTENing on the
// job_events channel and forwards every notification to the hub. It is
// the bridge between "something wrote a job_events row" (API or worker)
// and "the owning user's browser found out."
type Listener struct {
	pool   *pgxpool.Pool
	hub    *Hub
	logger *slog.Logger
}

func NewListener(pool *pgxpool.Pool, hub *Hub, logger *slog.Logger) *Listener {
	return &Listener{pool: pool, hub: hub, logger: logger}
}

// Run LISTENs and forwards notifications until ctx is cancelled,
// reconnecting on any connection error so a dropped connection does not
// permanently stop the WebSocket feed.
func (l *Listener) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := l.listenOnce(ctx); err != nil && ctx.Err() == nil {
			l.logger.Error("ws: listener error, reconnecting", "error", err)
			time.Sleep(reconnectDelay)
		}
	}
}

func (l *Listener) listenOnce(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN job_events"); err != nil {
		return err
	}

	for {
		notif, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		l.handle(notif.Payload)
	}
}

func (l *Listener) handle(payload string) {
	var n notification
	if err := json.Unmarshal([]byte(payload), &n); err != nil {
		l.logger.Error("ws: malformed notification payload", "error", err)
		return
	}

	msg, err := json.Marshal(Event{
		JobID:     n.JobID,
		EventType: n.EventType,
		Payload:   n.Payload,
		CreatedAt: n.CreatedAt,
	})
	if err != nil {
		l.logger.Error("ws: encode event", "error", err)
		return
	}

	l.hub.BroadcastToUser(n.UserID, msg)
}
