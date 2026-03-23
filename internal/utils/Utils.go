package utils

import (
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// Calculate the offset for the pagination
func CalculateOffset(page int, pageSize int) int {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10 // Default size
	}

	return (page - 1) * pageSize
}

// Calculate the offset for the pagination
func CalculateTotalPages(totalRecords int, pageSize int) int {

	return (totalRecords + pageSize - 1) / pageSize
}

// ID for the request
func GenerateULID() string {
	t := time.Now()
	// Create a local entropy source for thread-safety and performance
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return id.String()
}
