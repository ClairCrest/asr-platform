package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken covers every way a presented JWT can fail verification:
// bad signature, wrong type claim, expiry, or malformed claims.
var ErrInvalidToken = errors.New("auth: invalid token")

// TokenType distinguishes access tokens from refresh tokens so a refresh
// token cannot be replayed as an access token and vice versa.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Type   TokenType `json:"type"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewTokenIssuer(secret string, accessTTL, refreshTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{
		secret:          []byte(secret),
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}
}

// IssueAccessToken returns a short-lived JWT identifying userID.
func (i *TokenIssuer) IssueAccessToken(userID uuid.UUID) (string, error) {
	return i.issue(userID, TokenTypeAccess, i.accessTokenTTL)
}

// IssueRefreshToken returns a long-lived JWT that can be exchanged for a
// new access token via POST /auth/refresh.
func (i *TokenIssuer) IssueRefreshToken(userID uuid.UUID) (string, error) {
	return i.issue(userID, TokenTypeRefresh, i.refreshTokenTTL)
}

func (i *TokenIssuer) issue(userID uuid.UUID, typ TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Type:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Subject:   userID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates tokenString, ensuring its signature and
// expiry are valid and its type claim matches want.
func (i *TokenIssuer) Verify(tokenString string, want TokenType) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidToken
	}
	if claims.Type != want {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}
