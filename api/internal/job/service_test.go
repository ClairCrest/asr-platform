package job

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeStore is an in-memory Store used to exercise Service without a real
// database, mirroring the state-machine constraints the SQL layer enforces.
type fakeStore struct {
	jobs        map[uuid.UUID]Job
	events      map[uuid.UUID][]Event
	transcripts map[uuid.UUID]Transcript // keyed by job ID
	segments    map[uuid.UUID][]Segment  // keyed by transcript ID
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		jobs:        map[uuid.UUID]Job{},
		events:      map[uuid.UUID][]Event{},
		transcripts: map[uuid.UUID]Transcript{},
		segments:    map[uuid.UUID][]Segment{},
	}
}

func (f *fakeStore) CreateJob(_ context.Context, j Job) (Job, error) {
	j.CreatedAt = time.Now()
	f.jobs[j.ID] = j
	return j, nil
}

func (f *fakeStore) GetJob(_ context.Context, id, userID uuid.UUID) (Job, error) {
	j, ok := f.jobs[id]
	if !ok || j.UserID != userID {
		return Job{}, ErrNotFound
	}
	return j, nil
}

func (f *fakeStore) GetJobByIdempotencyKey(_ context.Context, userID uuid.UUID, key string) (Job, error) {
	for _, j := range f.jobs {
		if j.UserID == userID && j.IdempotencyKey != nil && *j.IdempotencyKey == key {
			return j, nil
		}
	}
	return Job{}, ErrNotFound
}

func (f *fakeStore) ListJobs(_ context.Context, userID uuid.UUID, status *Status, _ *time.Time, limit int32) ([]Job, error) {
	var out []Job
	for _, j := range f.jobs {
		if j.UserID != userID {
			continue
		}
		if status != nil && j.Status != *status {
			continue
		}
		out = append(out, j)
		if int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) UpdateJobStatus(_ context.Context, id uuid.UUID, status Status) (Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	j.Status = status
	f.jobs[id] = j
	return j, nil
}

func (f *fakeStore) CancelJob(_ context.Context, id, userID uuid.UUID) (Job, error) {
	j, ok := f.jobs[id]
	if !ok || j.UserID != userID || !CanCancel(j.Status) {
		return Job{}, ErrNotFound
	}
	j.Status = StatusCancelled
	f.jobs[id] = j
	return j, nil
}

func (f *fakeStore) RetryJob(_ context.Context, id, userID uuid.UUID) (Job, error) {
	j, ok := f.jobs[id]
	if !ok || j.UserID != userID || !CanRetry(j.Status) {
		return Job{}, ErrNotFound
	}
	j.Status = StatusQueued
	j.Attempts = 0
	f.jobs[id] = j
	return j, nil
}

func (f *fakeStore) SoftDeleteJob(_ context.Context, id, userID uuid.UUID) error {
	j, ok := f.jobs[id]
	if !ok || j.UserID != userID {
		return ErrNotFound
	}
	delete(f.jobs, id)
	return nil
}

func (f *fakeStore) CreateEvent(_ context.Context, jobID uuid.UUID, eventType EventType, payload []byte) (Event, error) {
	e := Event{ID: uuid.New(), JobID: jobID, EventType: eventType, Payload: payload, CreatedAt: time.Now()}
	f.events[jobID] = append(f.events[jobID], e)
	return e, nil
}

func (f *fakeStore) ListEvents(_ context.Context, jobID uuid.UUID) ([]Event, error) {
	return f.events[jobID], nil
}

func (f *fakeStore) GetTranscript(_ context.Context, jobID uuid.UUID) (Transcript, error) {
	t, ok := f.transcripts[jobID]
	if !ok {
		return Transcript{}, ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) ListSegments(_ context.Context, transcriptID uuid.UUID) ([]Segment, error) {
	return f.segments[transcriptID], nil
}

type fakeQueue struct {
	enqueued []uuid.UUID
	failNext bool
}

func (q *fakeQueue) Enqueue(_ context.Context, jobID uuid.UUID) error {
	if q.failNext {
		q.failNext = false
		return errEnqueueFailed
	}
	q.enqueued = append(q.enqueued, jobID)
	return nil
}

var errEnqueueFailed = &fakeError{"enqueue failed"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }

type fakeObjectStore struct {
	deleted []string
}

func (o *fakeObjectStore) DeleteObject(_ context.Context, key string) error {
	o.deleted = append(o.deleted, key)
	return nil
}

func newTestService() (*Service, *fakeStore, *fakeQueue, *fakeObjectStore) {
	store := newFakeStore()
	queue := &fakeQueue{}
	objects := &fakeObjectStore{}
	return NewService(store, queue, objects), store, queue, objects
}

func TestServiceCreate(t *testing.T) {
	svc, _, queue, _ := newTestService()
	userID := uuid.New()

	j, err := svc.Create(context.Background(), CreateParams{
		UserID:           userID,
		ObjectKey:        "audio/1.wav",
		OriginalFilename: "1.wav",
		SizeBytes:        1024,
		Model:             "small.en",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if j.Status != StatusQueued {
		t.Errorf("Create() status = %s, want %s", j.Status, StatusQueued)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0] != j.ID {
		t.Errorf("Create() did not enqueue job %s", j.ID)
	}
}

func TestServiceCreateIdempotent(t *testing.T) {
	svc, _, queue, _ := newTestService()
	userID := uuid.New()
	key := "req-123"

	first, err := svc.Create(context.Background(), CreateParams{
		UserID:           userID,
		IdempotencyKey:   &key,
		ObjectKey:        "audio/1.wav",
		OriginalFilename: "1.wav",
		SizeBytes:        1024,
		Model:            "small.en",
	})
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	second, err := svc.Create(context.Background(), CreateParams{
		UserID:           userID,
		IdempotencyKey:   &key,
		ObjectKey:        "audio/2.wav",
		OriginalFilename: "2.wav",
		SizeBytes:        2048,
		Model:            "small.en",
	})
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("second Create() returned a different job: got %s, want %s", second.ID, first.ID)
	}
	if len(queue.enqueued) != 1 {
		t.Errorf("expected exactly one enqueue for a replayed idempotency key, got %d", len(queue.enqueued))
	}
}

func TestServiceCreateEnqueueFailure(t *testing.T) {
	svc, _, queue, _ := newTestService()
	queue.failNext = true

	_, err := svc.Create(context.Background(), CreateParams{
		UserID:           uuid.New(),
		ObjectKey:        "audio/1.wav",
		OriginalFilename: "1.wav",
		SizeBytes:        1024,
		Model:            "small.en",
	})
	if err == nil {
		t.Fatal("Create() expected an error when enqueue fails, got nil")
	}
}

func TestServiceCancel(t *testing.T) {
	svc, _, _, _ := newTestService()
	userID := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	cancelled, err := svc.Cancel(context.Background(), userID, j.ID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Errorf("Cancel() status = %s, want %s", cancelled.Status, StatusCancelled)
	}

	if _, err := svc.Cancel(context.Background(), userID, j.ID); err != ErrNotCancellable {
		t.Errorf("second Cancel() error = %v, want %v", err, ErrNotCancellable)
	}
}

func TestServiceCancelNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	if _, err := svc.Cancel(context.Background(), uuid.New(), uuid.New()); err != ErrNotFound {
		t.Errorf("Cancel() error = %v, want %v", err, ErrNotFound)
	}
}

func TestServiceRetry(t *testing.T) {
	svc, store, queue, _ := newTestService()
	userID := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	failed := store.jobs[j.ID]
	failed.Status = StatusFailed
	store.jobs[j.ID] = failed

	retried, err := svc.Retry(context.Background(), userID, j.ID)
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if retried.Status != StatusQueued {
		t.Errorf("Retry() status = %s, want %s", retried.Status, StatusQueued)
	}
	if len(queue.enqueued) != 2 {
		t.Errorf("Retry() expected a second enqueue, got %d total", len(queue.enqueued))
	}
}

func TestServiceRetryNotRetryable(t *testing.T) {
	svc, _, _, _ := newTestService()
	userID := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	if _, err := svc.Retry(context.Background(), userID, j.ID); err != ErrNotRetryable {
		t.Errorf("Retry() error = %v, want %v", err, ErrNotRetryable)
	}
}

func TestServiceDelete(t *testing.T) {
	svc, _, _, objects := newTestService()
	userID := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "audio/1.wav", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	if err := svc.Delete(context.Background(), userID, j.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(objects.deleted) != 1 || objects.deleted[0] != "audio/1.wav" {
		t.Errorf("Delete() did not remove the object, got %v", objects.deleted)
	}
	if _, _, err := svc.Get(context.Background(), userID, j.ID); err != ErrNotFound {
		t.Errorf("Get() after Delete() error = %v, want %v", err, ErrNotFound)
	}
}

func TestServiceGetScopedToUser(t *testing.T) {
	svc, _, _, _ := newTestService()
	owner := uuid.New()
	other := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: owner, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	if _, _, err := svc.Get(context.Background(), other, j.ID); err != ErrNotFound {
		t.Errorf("Get() by non-owner error = %v, want %v", err, ErrNotFound)
	}
	if got, _, err := svc.Get(context.Background(), owner, j.ID); err != nil || got.ID != j.ID {
		t.Errorf("Get() by owner = %v, %v, want job %s", got, err, j.ID)
	}
}

func TestServiceList(t *testing.T) {
	svc, _, _, _ := newTestService()
	userID := uuid.New()
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})
	}

	jobs, err := svc.List(context.Background(), userID, nil, nil, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("List() returned %d jobs, want 3", len(jobs))
	}
}

func TestServiceGetTranscript(t *testing.T) {
	svc, store, _, _ := newTestService()
	userID := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	transcriptID := uuid.New()
	store.transcripts[j.ID] = Transcript{ID: transcriptID, JobID: j.ID, Text: "hello world", LanguageDetected: "en", LanguageProbability: 0.99}
	store.segments[transcriptID] = []Segment{{ID: uuid.New(), TranscriptID: transcriptID, Idx: 0, StartMs: 0, EndMs: 1000, Text: "hello world"}}

	transcript, segments, err := svc.GetTranscript(context.Background(), userID, j.ID)
	if err != nil {
		t.Fatalf("GetTranscript() error = %v", err)
	}
	if transcript.Text != "hello world" {
		t.Errorf("GetTranscript() text = %q, want %q", transcript.Text, "hello world")
	}
	if len(segments) != 1 {
		t.Errorf("GetTranscript() returned %d segments, want 1", len(segments))
	}
}

func TestServiceGetTranscriptNotReady(t *testing.T) {
	svc, _, _, _ := newTestService()
	userID := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: userID, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})

	if _, _, err := svc.GetTranscript(context.Background(), userID, j.ID); err != ErrNotFound {
		t.Errorf("GetTranscript() before completion error = %v, want %v", err, ErrNotFound)
	}
}

func TestServiceGetTranscriptScopedToUser(t *testing.T) {
	svc, store, _, _ := newTestService()
	owner := uuid.New()
	other := uuid.New()
	j, _ := svc.Create(context.Background(), CreateParams{UserID: owner, ObjectKey: "a", OriginalFilename: "a", SizeBytes: 1, Model: "small.en"})
	store.transcripts[j.ID] = Transcript{ID: uuid.New(), JobID: j.ID, Text: "secret"}

	if _, _, err := svc.GetTranscript(context.Background(), other, j.ID); err != ErrNotFound {
		t.Errorf("GetTranscript() by non-owner error = %v, want %v", err, ErrNotFound)
	}
}
