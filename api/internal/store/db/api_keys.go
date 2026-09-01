package db

import (
	"context"

	"github.com/google/uuid"
)

const createAPIKey = `
INSERT INTO api_keys (id, user_id, name, key_hash)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, name, key_hash, last_used_at, created_at, revoked_at
`

func (q *Queries) CreateAPIKey(ctx context.Context, id, userID uuid.UUID, name, keyHash string) (ApiKey, error) {
	row := q.db.QueryRow(ctx, createAPIKey, id, userID, name, keyHash)
	var k ApiKey
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt)
	return k, err
}

const listAPIKeysByUser = `
SELECT id, user_id, name, key_hash, last_used_at, created_at, revoked_at
FROM api_keys
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC
`

func (q *Queries) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]ApiKey, error) {
	rows, err := q.db.Query(ctx, listAPIKeysByUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []ApiKey{}
	for rows.Next() {
		var k ApiKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

const getAPIKeyByHash = `
SELECT id, user_id, name, key_hash, last_used_at, created_at, revoked_at
FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL
`

func (q *Queries) GetAPIKeyByHash(ctx context.Context, keyHash string) (ApiKey, error) {
	row := q.db.QueryRow(ctx, getAPIKeyByHash, keyHash)
	var k ApiKey
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt)
	return k, err
}

const revokeAPIKey = `
UPDATE api_keys
SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
`

func (q *Queries) RevokeAPIKey(ctx context.Context, id, userID uuid.UUID) (int64, error) {
	tag, err := q.db.Exec(ctx, revokeAPIKey, id, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

const touchAPIKeyLastUsed = `
UPDATE api_keys SET last_used_at = now() WHERE id = $1
`

func (q *Queries) TouchAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, touchAPIKeyLastUsed, id)
	return err
}
