package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken takes a raw token string and returns its SHA-256 hash
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
