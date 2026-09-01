package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/store/db"
)

// APIKeyStore implements auth.APIKeyStore against Postgres.
type APIKeyStore struct {
	q *db.Queries
}

func NewAPIKeyStore(pool db.DBTX) *APIKeyStore {
	return &APIKeyStore{q: db.New(pool)}
}

func (s *APIKeyStore) CreateAPIKey(ctx context.Context, id, userID uuid.UUID, name, keyHash string) (auth.APIKey, error) {
	row, err := s.q.CreateAPIKey(ctx, id, userID, name, keyHash)
	if err != nil {
		return auth.APIKey{}, fmt.Errorf("apikeystore: create: %w", err)
	}
	return fromDBAPIKey(row), nil
}

func (s *APIKeyStore) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]auth.APIKey, error) {
	rows, err := s.q.ListAPIKeysByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("apikeystore: list: %w", err)
	}
	out := make([]auth.APIKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromDBAPIKey(r))
	}
	return out, nil
}

func (s *APIKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (auth.APIKey, error) {
	row, err := s.q.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.APIKey{}, auth.ErrAPIKeyNotFound
		}
		return auth.APIKey{}, fmt.Errorf("apikeystore: get by hash: %w", err)
	}
	return fromDBAPIKey(row), nil
}

func (s *APIKeyStore) RevokeAPIKey(ctx context.Context, id, userID uuid.UUID) error {
	affected, err := s.q.RevokeAPIKey(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("apikeystore: revoke: %w", err)
	}
	if affected == 0 {
		return auth.ErrAPIKeyNotFound
	}
	return nil
}

func (s *APIKeyStore) TouchAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	if err := s.q.TouchAPIKeyLastUsed(ctx, id); err != nil {
		return fmt.Errorf("apikeystore: touch last used: %w", err)
	}
	return nil
}

func fromDBAPIKey(k db.ApiKey) auth.APIKey {
	return auth.APIKey{
		ID:         k.ID,
		UserID:     k.UserID,
		Name:       k.Name,
		KeyHash:    k.KeyHash,
		LastUsedAt: k.LastUsedAt,
		CreatedAt:  k.CreatedAt,
		RevokedAt:  k.RevokedAt,
	}
}
