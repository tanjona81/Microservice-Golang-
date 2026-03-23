package utils

import (
	"os"
	"strconv"
	"time"
)

// GetEnv gets a string from the env or returns a default
func GetEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// GetEnvAsInt gets an int from the env or returns a default
func GetEnvAsInt(key string, fallback int) int {
	valueStr := GetEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

// GetEnvAsDuration gets a time.Duration from the env (e.g. "24h") or returns a default
func GetEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valueStr := GetEnv(key, "")
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return fallback
}
