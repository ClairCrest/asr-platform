package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const apiKeyPrefix = "asrk_"

// GenerateAPIKey returns a random raw API key (shown to the user exactly
// once) and the SHA-256 hex hash stored in the database for lookups.
func GenerateAPIKey() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("auth: generate api key: %w", err)
	}
	raw = apiKeyPrefix + hex.EncodeToString(buf)
	hash = HashAPIKey(raw)
	return raw, hash, nil
}

// HashAPIKey returns the SHA-256 hex digest of a raw API key, used both to
// store and to look up keys without ever persisting the raw value.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
