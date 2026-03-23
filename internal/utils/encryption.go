package utils

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EncodeCursor takes the last record's metadata and turns it into a string
func EncodeCursor(t time.Time, id int64) string {
	if t.IsZero() {
		return ""
	}
	// Format: "unix_timestamp,id"
	str := fmt.Sprintf("%d,%d", t.UnixNano(), id)
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// DecodeCursor turns the string back into Go types for your SQL query
func DecodeCursor(cursor string) (time.Time, int64, error) {
	if cursor == "" {
		return time.Time{}, 0, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, err
	}

	parts := strings.Split(string(decoded), ",")
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}

	nano, _ := strconv.ParseInt(parts[0], 10, 64)
	id, _ := strconv.ParseInt(parts[1], 10, 64)

	return time.Unix(0, nano), id, nil
}
