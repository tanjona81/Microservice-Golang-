package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateRefreshToken creates a random 32-byte string (64 characters in hex)
func GenerateRefreshToken() (string, error) {
	// 32 bytes provides 256 bits of entropy - overkill for safety
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure bytes: %w", err)
	}

	// Convert to hex string for easy storage and transmission
	return hex.EncodeToString(b), nil
}
