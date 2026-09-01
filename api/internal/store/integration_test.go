//go:build integration

// Integration tests against a real Postgres via testcontainers. Run with:
//
//	go test -tags=integration ./internal/store/...
//
// Skipped by default (and by `go test -race ./...` in CI's unit test step)
// because they need a working Docker daemon.
package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/job"
)

// newTestPool starts a real Postgres in a container, applies every
// migrations/*.up.sql in order directly over pgx, and returns a connected
// pool. Migrations are applied by hand rather than via golang-migrate to
// avoid that library's lib/pq driver, which failed to connect to the
// container's mapped port in this environment.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("asr"),
		postgres.WithUsername("asr"),
		postgres.WithPassword("asr"),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	// Postgres restarts once internally right after initdb, which can drop
	// a connection made in the small window between the "ready" wait
	// strategy firing and that restart completing. Retry briefly instead
	// of failing on that first flaky attempt.
	var pool *pgxpool.Pool
	for attempt := 1; attempt <= 5; attempt++ {
		pool, err = NewPool(ctx, dsn)
		if err == nil {
			break
		}
		if attempt == 5 {
			t.Fatalf("connect pool after %d attempts: %v", attempt, err)
		}
		time.Sleep(2 * time.Second)
	}
	t.Cleanup(pool.Close)

	applyMigrations(t, ctx, pool)

	return pool
}

func applyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		sql, err := os.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func TestJobStoreIntegration(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	u, err := users.CreateUser(ctx, uuid.New(), "integration@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	jobs := NewJobStore(pool)
	created, err := jobs.CreateJob(ctx, job.Job{
		ID:               uuid.New(),
		UserID:           u.ID,
		Status:           job.StatusPending,
		ObjectKey:        "audio/1.wav",
		OriginalFilename: "1.wav",
		SizeBytes:        1024,
		Model:            "small.en",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if created.Status != job.StatusPending {
		t.Errorf("CreateJob() status = %s, want %s", created.Status, job.StatusPending)
	}

	queued, err := jobs.UpdateJobStatus(ctx, created.ID, job.StatusQueued)
	if err != nil {
		t.Fatalf("UpdateJobStatus() error = %v", err)
	}
	if queued.Status != job.StatusQueued {
		t.Errorf("UpdateJobStatus() status = %s, want %s", queued.Status, job.StatusQueued)
	}

	got, err := jobs.GetJob(ctx, created.ID, u.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetJob() ID = %s, want %s", got.ID, created.ID)
	}

	if _, err := jobs.CreateEvent(ctx, created.ID, job.EventQueued, []byte("{}")); err != nil {
		t.Fatalf("CreateEvent() error = %v", err)
	}
	events, err := jobs.ListEvents(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Errorf("ListEvents() returned %d events, want 1", len(events))
	}

	cancelled, err := jobs.CancelJob(ctx, created.ID, u.ID)
	if err != nil {
		t.Fatalf("CancelJob() error = %v", err)
	}
	if cancelled.Status != job.StatusCancelled {
		t.Errorf("CancelJob() status = %s, want %s", cancelled.Status, job.StatusCancelled)
	}

	if err := jobs.SoftDeleteJob(ctx, created.ID, u.ID); err != nil {
		t.Fatalf("SoftDeleteJob() error = %v", err)
	}
	if _, err := jobs.GetJob(ctx, created.ID, u.ID); err != job.ErrNotFound {
		t.Errorf("GetJob() after delete error = %v, want %v", err, job.ErrNotFound)
	}
}

func TestUserAndAPIKeyStoreIntegration(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	u, err := users.CreateUser(ctx, uuid.New(), "keys@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := users.CreateUser(ctx, uuid.New(), "keys@example.com", "hash"); err != auth.ErrEmailTaken {
		t.Errorf("duplicate CreateUser() error = %v, want %v", err, auth.ErrEmailTaken)
	}

	keys := NewAPIKeyStore(pool)
	key, err := keys.CreateAPIKey(ctx, uuid.New(), u.ID, "ci", "hash-of-raw-key")
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	got, err := keys.GetAPIKeyByHash(ctx, "hash-of-raw-key")
	if err != nil || got.ID != key.ID {
		t.Fatalf("GetAPIKeyByHash() = %v, %v, want key %s", got, err, key.ID)
	}

	if err := keys.RevokeAPIKey(ctx, key.ID, u.ID); err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}
	if _, err := keys.GetAPIKeyByHash(ctx, "hash-of-raw-key"); err != auth.ErrAPIKeyNotFound {
		t.Errorf("GetAPIKeyByHash() after revoke error = %v, want %v", err, auth.ErrAPIKeyNotFound)
	}
}
