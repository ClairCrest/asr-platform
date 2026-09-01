package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeUserStore struct {
	byEmail map[string]User
	byID    map[uuid.UUID]User
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{byEmail: map[string]User{}, byID: map[uuid.UUID]User{}}
}

func (f *fakeUserStore) CreateUser(_ context.Context, id uuid.UUID, email, passwordHash string) (User, error) {
	if _, exists := f.byEmail[email]; exists {
		return User{}, ErrEmailTaken
	}
	u := User{ID: id, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now()}
	f.byEmail[email] = u
	f.byID[id] = u
	return u, nil
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserStore) GetUserByID(_ context.Context, id uuid.UUID) (User, error) {
	u, ok := f.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

type fakeAPIKeyStore struct {
	keys map[uuid.UUID]APIKey
}

func newFakeAPIKeyStore() *fakeAPIKeyStore {
	return &fakeAPIKeyStore{keys: map[uuid.UUID]APIKey{}}
}

func (f *fakeAPIKeyStore) CreateAPIKey(_ context.Context, id, userID uuid.UUID, name, keyHash string) (APIKey, error) {
	k := APIKey{ID: id, UserID: userID, Name: name, KeyHash: keyHash, CreatedAt: time.Now()}
	f.keys[id] = k
	return k, nil
}

func (f *fakeAPIKeyStore) ListAPIKeysByUser(_ context.Context, userID uuid.UUID) ([]APIKey, error) {
	var out []APIKey
	for _, k := range f.keys {
		if k.UserID == userID && k.RevokedAt == nil {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeAPIKeyStore) GetAPIKeyByHash(_ context.Context, keyHash string) (APIKey, error) {
	for _, k := range f.keys {
		if k.KeyHash == keyHash && k.RevokedAt == nil {
			return k, nil
		}
	}
	return APIKey{}, ErrAPIKeyNotFound
}

func (f *fakeAPIKeyStore) RevokeAPIKey(_ context.Context, id, userID uuid.UUID) error {
	k, ok := f.keys[id]
	if !ok || k.UserID != userID || k.RevokedAt != nil {
		return ErrAPIKeyNotFound
	}
	now := time.Now()
	k.RevokedAt = &now
	f.keys[id] = k
	return nil
}

func (f *fakeAPIKeyStore) TouchAPIKeyLastUsed(_ context.Context, id uuid.UUID) error {
	k, ok := f.keys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}
	now := time.Now()
	k.LastUsedAt = &now
	f.keys[id] = k
	return nil
}

func newTestService() (*Service, *fakeUserStore, *fakeAPIKeyStore) {
	users := newFakeUserStore()
	apiKeys := newFakeAPIKeyStore()
	tokens := NewTokenIssuer("test-secret", 15*time.Minute, 7*24*time.Hour)
	return NewService(users, apiKeys, tokens), users, apiKeys
}

func TestServiceRegister(t *testing.T) {
	svc, _, _ := newTestService()

	u, err := svc.Register(context.Background(), "a@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.Email != "a@example.com" {
		t.Errorf("Register() email = %s, want a@example.com", u.Email)
	}
	if u.PasswordHash == "hunter2" {
		t.Error("Register() stored the raw password instead of a hash")
	}
}

func TestServiceRegisterDuplicateEmail(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	if _, err := svc.Register(ctx, "a@example.com", "hunter2"); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if _, err := svc.Register(ctx, "a@example.com", "other"); err != ErrEmailTaken {
		t.Errorf("second Register() error = %v, want %v", err, ErrEmailTaken)
	}
}

func TestServiceLogin(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.Register(ctx, "a@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	pair, err := svc.Login(ctx, "a@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("Login() returned empty tokens")
	}
}

func TestServiceLoginWrongPassword(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	_, _ = svc.Register(ctx, "a@example.com", "hunter2")

	if _, err := svc.Login(ctx, "a@example.com", "wrong"); err != ErrInvalidCredentials {
		t.Errorf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestServiceLoginUnknownEmail(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.Login(context.Background(), "nobody@example.com", "x"); err != ErrInvalidCredentials {
		t.Errorf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestServiceRefresh(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	_, _ = svc.Register(ctx, "a@example.com", "hunter2")
	pair, err := svc.Login(ctx, "a@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	refreshed, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.AccessToken == "" {
		t.Error("Refresh() returned an empty access token")
	}
}

func TestServiceRefreshRejectsAccessToken(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	_, _ = svc.Register(ctx, "a@example.com", "hunter2")
	pair, _ := svc.Login(ctx, "a@example.com", "hunter2")

	if _, err := svc.Refresh(ctx, pair.AccessToken); err != ErrInvalidCredentials {
		t.Errorf("Refresh() with an access token error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestServiceAPIKeyLifecycle(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	u, _ := svc.Register(ctx, "a@example.com", "hunter2")

	raw, key, err := svc.CreateAPIKey(ctx, u.ID, "ci")
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if raw == "" {
		t.Fatal("CreateAPIKey() returned an empty raw key")
	}

	gotUserID, err := svc.AuthenticateAPIKey(ctx, raw)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}
	if gotUserID != u.ID {
		t.Errorf("AuthenticateAPIKey() userID = %s, want %s", gotUserID, u.ID)
	}

	keys, err := svc.ListAPIKeys(ctx, u.ID)
	if err != nil || len(keys) != 1 {
		t.Fatalf("ListAPIKeys() = %v, %v, want 1 key", keys, err)
	}

	if err := svc.RevokeAPIKey(ctx, u.ID, key.ID); err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}

	if _, err := svc.AuthenticateAPIKey(ctx, raw); err != ErrInvalidAPIKey {
		t.Errorf("AuthenticateAPIKey() after revoke error = %v, want %v", err, ErrInvalidAPIKey)
	}
}

func TestServiceAuthenticateAPIKeyInvalid(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.AuthenticateAPIKey(context.Background(), "asrk_doesnotexist"); err != ErrInvalidAPIKey {
		t.Errorf("AuthenticateAPIKey() error = %v, want %v", err, ErrInvalidAPIKey)
	}
}
