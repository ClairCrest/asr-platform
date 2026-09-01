// Package metrics defines the Prometheus collectors served on the API's
// dedicated metrics port (see PLAN.md section 4: GET /metrics, port 9090,
// separate from the main API port so /metrics can't be blocked behind
// auth middleware or rate limits meant for real traffic).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	JobsSubmittedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "jobs_submitted_total",
		Help: "Total number of jobs created via POST /jobs.",
	})

	// JobsCompletedTotal is labeled by terminal status (succeeded, failed,
	// cancelled) rather than split into three counters, so a Grafana panel
	// can group/stack by status without joining multiple series.
	JobsCompletedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_completed_total",
		Help: "Total number of jobs that reached a terminal state, by status.",
	}, []string{"status"})

	// JobQueueDepth is refreshed periodically from the Redis consumer
	// group's lag (see queue.MetricsCollector), not read synchronously
	// per-scrape, since XINFO GROUPS is cheap but not free and Prometheus
	// may scrape more often than the queue actually changes.
	JobQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "job_queue_depth",
		Help: "Number of jobs in jobs:pending not yet delivered to or acked by a worker.",
	})

	// JobDurationSeconds measures wall-clock time from job creation to
	// reaching a terminal state — the metric a user actually experiences,
	// as opposed to worker_rtf (worker.py), which measures only the
	// transcription step itself.
	JobDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "job_duration_seconds",
		Help:    "Wall-clock seconds from job creation to reaching a terminal state.",
		Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800, 3600},
	})

	AudioSecondsProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "audio_seconds_processed_total",
		Help: "Total seconds of audio successfully transcribed.",
	})
)

// Register adds every collector in this package to reg. Called once at
// startup; a package-level init() would register collectors even for
// code (e.g. unit tests) that never serves /metrics and doesn't want them.
func Register(reg prometheus.Registerer) {
	reg.MustRegister(
		JobsSubmittedTotal,
		JobsCompletedTotal,
		JobQueueDepth,
		JobDurationSeconds,
		AudioSecondsProcessedTotal,
	)
}
