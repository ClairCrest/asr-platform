package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/store/db"
)

const uniqueViolation = "23505"

// UserStore implements auth.UserStore against Postgres.
type UserStore struct {
	q *db.Queries
}

func NewUserStore(pool db.DBTX) *UserStore {
	return &UserStore{q: db.New(pool)}
}

func (s *UserStore) CreateUser(ctx context.Context, id uuid.UUID, email, passwordHash string) (auth.User, error) {
	row, err := s.q.CreateUser(ctx, id, email, passwordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return auth.User{}, auth.ErrEmailTaken
		}
		return auth.User{}, fmt.Errorf("userstore: create: %w", err)
	}
	return fromDBUser(row), nil
}

func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	row, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, fmt.Errorf("userstore: get by email: %w", err)
	}
	return fromDBUser(row), nil
}

func (s *UserStore) GetUserByID(ctx context.Context, id uuid.UUID) (auth.User, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, fmt.Errorf("userstore: get by id: %w", err)
	}
	return fromDBUser(row), nil
}

func fromDBUser(u db.User) auth.User {
	return auth.User{ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, CreatedAt: u.CreatedAt}
}
