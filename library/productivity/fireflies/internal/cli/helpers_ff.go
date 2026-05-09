// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseSinceDuration parses strings like "24h", "7d", "2h", "30m" into a time.Duration.
func parseSinceDuration(s string) (time.Duration, error) {
	if s == "" {
		return 24 * time.Hour, nil
	}
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// nowMinusDuration returns the current time minus the given duration as epoch milliseconds.
func nowMinusDuration(d time.Duration) int64 {
	return time.Now().Add(-d).UnixMilli()
}
