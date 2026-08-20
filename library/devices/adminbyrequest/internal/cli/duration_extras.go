// Copyright 2026 joltsconsulting and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDurationExtras accepts standard Go durations (5m, 24h) and the human-
// friendly suffixes 'd' (days), 'w' (weeks), and 'mo' (30-day months) that
// time.ParseDuration does not understand. Used by novel-feature commands so
// users can write --window 30d instead of --window 720h.
func parseDurationExtras(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	s = strings.TrimSpace(s)
	switch {
	case strings.HasSuffix(s, "mo"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "mo"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case strings.HasSuffix(s, "w"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case strings.HasSuffix(s, "d"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	default:
		return time.ParseDuration(s)
	}
}
