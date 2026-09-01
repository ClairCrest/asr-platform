package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ClairCrest/asr-platform/api/internal/metrics"
)

const reconnectDelay = 2 * time.Second

// terminalStatuses are the job_events event types that also mark a job's
// jobs.status as terminal — the only ones metrics cares about.
var terminalStatuses = map[string]bool{"succeeded": true, "failed": true, "cancelled": true}

// notification mirrors the JSON object built by the notify_job_event()
// trigger (migrations 000008, extended by 000009): job id, owning user
// id, event type, the event's own payload, its timestamp, and — for
// metrics, not the WebSocket feed — the job's audio duration and the
// wall-clock time elapsed since it was created.
type notification struct {
	JobID           uuid.UUID       `json:"job_id"`
	UserID          uuid.UUID       `json:"user_id"`
	EventType       string          `json:"event_type"`
	Payload         json.RawMessage `json:"payload"`
	CreatedAt       time.Time       `json:"created_at"`
	DurationSeconds *float64        `json:"duration_seconds"`
	ElapsedSeconds  float64         `json:"elapsed_seconds"`
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
	l.recordMetrics(n)
}

func (l *Listener) recordMetrics(n notification) {
	// "created" only fires on a genuine new row — job.Service.Create
	// returns early on an idempotency-key replay, before writing any
	// event — so this can't double-count retried submissions.
	if n.EventType == "created" {
		metrics.JobsSubmittedTotal.Inc()
		return
	}
	if !terminalStatuses[n.EventType] {
		return
	}
	metrics.JobsCompletedTotal.WithLabelValues(n.EventType).Inc()
	metrics.JobDurationSeconds.Observe(n.ElapsedSeconds)
	if n.EventType == "succeeded" && n.DurationSeconds != nil {
		metrics.AudioSecondsProcessedTotal.Add(*n.DurationSeconds)
	}
}
