// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cliutil — shared HTTP retry helpers used by source clients.
package cliutil

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter parses the two RFC-7231 Retry-After header shapes:
// a bare integer number of seconds, or an HTTP-date. Returns 0 when
// the value is empty or unparseable so callers can fall back to an
// exponential backoff default. The maximum is capped at 60s so a
// misconfigured server cannot stall a sync forever.
func ParseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		if secs > 60 {
			secs = 60
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		if d > 60*time.Second {
			d = 60 * time.Second
		}
		return d
	}
	return 0
}

// TruncateBytes returns the first n bytes of b as a string with an
// ellipsis appended when truncated. Used for clamping upstream error
// bodies before logging — the byte cut is intentional here because the
// bodies are diagnostic, not user-facing, and a multibyte split would
// just produce mojibake in a log line.
func TruncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
