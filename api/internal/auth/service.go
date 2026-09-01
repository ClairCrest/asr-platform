package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// UserStore is the persistence boundary for user accounts. GetUser methods
// return ErrUserNotFound when no row matches; CreateUser returns
// ErrEmailTaken on a duplicate email.
type UserStore interface {
	CreateUser(ctx context.Context, id uuid.UUID, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
}

// APIKeyStore is the persistence boundary for API keys. GetAPIKeyByHash
// returns ErrAPIKeyNotFound when no active key matches the hash.
type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, id, userID uuid.UUID, name, keyHash string) (APIKey, error)
	ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error)
	GetAPIKeyByHash(ctx context.Context, keyHash string) (APIKey, error)
	RevokeAPIKey(ctx context.Context, id, userID uuid.UUID) error
	TouchAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	users   UserStore
	apiKeys APIKeyStore
	tokens  *TokenIssuer
}

func NewService(users UserStore, apiKeys APIKeyStore, tokens *TokenIssuer) *Service {
	return &Service{users: users, apiKeys: apiKeys, tokens: tokens}
}

// Register creates a new user with an argon2id-hashed password. It returns
// ErrEmailTaken if the email is already registered.
func (s *Service) Register(ctx context.Context, email, password string) (User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("auth: hash password: %w", err)
	}

	u, err := s.users.CreateUser(ctx, uuid.New(), email, hash)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("auth: register: %w", err)
	}
	return u, nil
}

// Login verifies email and password and, on success, issues a fresh access
// and refresh token pair. It returns ErrInvalidCredentials for both an
// unknown email and a wrong password, so the caller cannot distinguish
// which one failed.
func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	u, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, fmt.Errorf("auth: login: %w", err)
	}

	ok, err := VerifyPassword(u.PasswordHash, password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: verify password: %w", err)
	}
	if !ok {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.issueTokenPair(u.ID)
}

// Refresh exchanges a valid refresh token for a fresh access and refresh
// token pair. It returns ErrInvalidCredentials if the token is expired,
// malformed, or not a refresh token.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := s.tokens.Verify(refreshToken, TokenTypeRefresh)
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	if _, err := s.users.GetUserByID(ctx, claims.UserID); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, fmt.Errorf("auth: refresh: %w", err)
	}

	return s.issueTokenPair(claims.UserID)
}

func (s *Service) issueTokenPair(userID uuid.UUID) (TokenPair, error) {
	access, err := s.tokens.IssueAccessToken(userID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: issue access token: %w", err)
	}
	refresh, err := s.tokens.IssueRefreshToken(userID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: issue refresh token: %w", err)
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// CurrentUser resolves the user identified by a verified access token.
func (s *Service) CurrentUser(ctx context.Context, userID uuid.UUID) (User, error) {
	u, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("auth: current user: %w", err)
	}
	return u, nil
}

// CreateAPIKey generates a new API key for userID and returns both the raw
// key — shown to the caller exactly once — and the stored record.
func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string) (string, APIKey, error) {
	raw, hash, err := GenerateAPIKey()
	if err != nil {
		return "", APIKey{}, fmt.Errorf("auth: generate api key: %w", err)
	}

	k, err := s.apiKeys.CreateAPIKey(ctx, uuid.New(), userID, name, hash)
	if err != nil {
		return "", APIKey{}, fmt.Errorf("auth: create api key: %w", err)
	}
	return raw, k, nil
}

// ListAPIKeys returns every active (non-revoked) API key for userID. The
// raw key material is never stored, so these records never include it.
func (s *Service) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	keys, err := s.apiKeys.ListAPIKeysByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list api keys: %w", err)
	}
	return keys, nil
}

// RevokeAPIKey revokes keyID, scoped to userID so a user cannot revoke
// another user's key.
func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	if err := s.apiKeys.RevokeAPIKey(ctx, keyID, userID); err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return ErrAPIKeyNotFound
		}
		return fmt.Errorf("auth: revoke api key: %w", err)
	}
	return nil
}

// AuthenticateAPIKey resolves the user owning a raw API key and records its
// use. It returns ErrInvalidAPIKey if the key does not match any active key.
func (s *Service) AuthenticateAPIKey(ctx context.Context, raw string) (uuid.UUID, error) {
	hash := HashAPIKey(raw)
	k, err := s.apiKeys.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return uuid.Nil, ErrInvalidAPIKey
		}
		return uuid.Nil, fmt.Errorf("auth: authenticate api key: %w", err)
	}
	_ = s.apiKeys.TouchAPIKeyLastUsed(ctx, k.ID)
	return k.UserID, nil
}
