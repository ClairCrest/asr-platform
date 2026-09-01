package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ClairCrest/asr-platform/api/internal/metrics"
)

// MetricsCollector periodically refreshes metrics.JobQueueDepth from the
// consumer group's lag. It polls rather than reading synchronously on
// every /metrics scrape because XINFO GROUPS is cheap but not free, and
// Prometheus may scrape far more often than the queue actually changes.
type MetricsCollector struct {
	rdb    *redis.Client
	logger *slog.Logger
}

func NewMetricsCollector(rdb *redis.Client, logger *slog.Logger) *MetricsCollector {
	return &MetricsCollector{rdb: rdb, logger: logger}
}

func (c *MetricsCollector) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

func (c *MetricsCollector) refresh(ctx context.Context) {
	groups, err := c.rdb.XInfoGroups(ctx, StreamName).Result()
	if err != nil {
		// A brand new stream with no consumer group yet is not an error
		// worth logging on every tick — it just means depth is 0.
		if err.Error() != "ERR no such key" {
			c.logger.Error("metrics: xinfo groups", "error", err)
		}
		metrics.JobQueueDepth.Set(0)
		return
	}

	var depth int64
	for _, g := range groups {
		if g.Name != ConsumerGroup {
			continue
		}
		lag := g.Lag
		if lag < 0 {
			// Redis returns -1 when it can't determine lag (e.g. after
			// XDEL/XTRIM broke the entries-added/entries-read invariant).
			// Pending alone still undercounts but beats reporting -1.
			lag = 0
		}
		depth = lag + g.Pending
	}
	metrics.JobQueueDepth.Set(float64(depth))
}
