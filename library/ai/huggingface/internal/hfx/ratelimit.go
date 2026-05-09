package hfx

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RateLimitState is the persisted shape of the shared HF rate-limit bucket.
// Persisted to <state-dir>/rate-limit-bucket.json so CC + JARVIS + cron all
// share the same view of remaining quota.
//
// Source of truth is the server's `RateLimit:` response header
// (RFC draft v9 — `RateLimit: "api";r=N;t=N`). We snapshot it on every HTTP
// response and clamp our outbound rate to remaining/window.
type RateLimitState struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Remaining     int       `json:"remaining"`
	ResetSeconds  int       `json:"reset_seconds"` // seconds until window resets
	WindowSeen    string    `json:"window_seen"`   // raw header value for debugging
}

const rateLimitFile = "rate-limit-bucket.json"

// ParseRateLimitHeader extracts (remaining, resetSeconds) from an HTTP
// response. Honors both the modern RFC draft `RateLimit:` header and the
// legacy `X-RateLimit-*` headers. Returns (-1, -1) if no rate-limit info is
// present (caller should leave the bucket unchanged in that case).
func ParseRateLimitHeader(h http.Header) (remaining, resetSeconds int, raw string) {
	// Modern RFC draft form: `RateLimit: "api";r=0;t=60`
	if v := h.Get("RateLimit"); v != "" {
		raw = v
		// Naive parse: split on ; and pull r=N, t=N tokens.
		remaining, resetSeconds = -1, -1
		for _, part := range strings.Split(v, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "r=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(part, "r=")); err == nil {
					remaining = n
				}
			} else if strings.HasPrefix(part, "t=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(part, "t=")); err == nil {
					resetSeconds = n
				}
			}
		}
		return
	}
	// Legacy form: X-RateLimit-Remaining + X-RateLimit-Reset
	if rem := h.Get("X-RateLimit-Remaining"); rem != "" {
		raw = "X-RateLimit-Remaining=" + rem
		if n, err := strconv.Atoi(rem); err == nil {
			remaining = n
		} else {
			remaining = -1
		}
		if reset := h.Get("X-RateLimit-Reset"); reset != "" {
			if n, err := strconv.Atoi(reset); err == nil {
				resetSeconds = n
				raw += " X-RateLimit-Reset=" + reset
			} else {
				resetSeconds = -1
			}
		}
		return
	}
	return -1, -1, ""
}

// LoadRateLimit reads the persisted bucket. Returns a zero-value state with
// Remaining=-1 (unknown) if the file does not yet exist.
func LoadRateLimit(stateDir string) RateLimitState {
	var s RateLimitState
	path := filepath.Join(stateDir, rateLimitFile)
	if err := ReadJSONLocked(stateDir, path, &s); err != nil {
		return RateLimitState{SchemaVersion: SchemaVersion, Remaining: -1}
	}
	return s
}

// SaveRateLimit persists the bucket. Honors --no-write; callers pass through.
func SaveRateLimit(stateDir string, s RateLimitState, noWrite bool) error {
	s.SchemaVersion = SchemaVersion
	s.UpdatedAt = time.Now().UTC()
	path := filepath.Join(stateDir, rateLimitFile)
	return WriteJSONLocked(stateDir, path, s, noWrite)
}

// IsRateLimitResponse returns true when status indicates the caller should
// emit ExitRateLimited and abort, NOT silently retry.
func IsRateLimitResponse(status int) bool {
	return status == http.StatusTooManyRequests
}
