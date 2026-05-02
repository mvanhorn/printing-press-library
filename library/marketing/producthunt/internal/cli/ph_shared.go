package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseSinceDurationISO accepts shorthand like "2h", "24h", "7d", "30d", "1w"
// and returns the corresponding ISO8601-RFC3339 timestamp at exactly N units in
// the past, in UTC. Use this when a GraphQL postedAfter variable needs an
// RFC3339 string; for callers that want a time.Time, the existing
// parseSinceDuration in sync.go does the same shape but returns time.Time.
func parseSinceDurationISO(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty duration")
	}
	last := s[len(s)-1]
	num := s[:len(s)-1]
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 {
		return "", fmt.Errorf("expected <int><unit>, got %q (units: h, d, w, m)", s)
	}
	var dur time.Duration
	switch last {
	case 'h':
		dur = time.Duration(n) * time.Hour
	case 'd':
		dur = time.Duration(n) * 24 * time.Hour
	case 'w':
		dur = time.Duration(n) * 7 * 24 * time.Hour
	case 'm':
		dur = time.Duration(n) * 30 * 24 * time.Hour
	default:
		return "", fmt.Errorf("unsupported unit %q (use h, d, w, m)", string(last))
	}
	return time.Now().Add(-dur).UTC().Format(time.RFC3339), nil
}

// midnightUTC returns today's midnight UTC as RFC3339 — used by `today`.
func midnightUTC() string {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
}
