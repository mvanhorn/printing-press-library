// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 source adapter interface + typed errors.

package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// Adapter is implemented by every transcript source.
type Adapter interface {
	Name() string
	Tier() transcript.Tier
	Match(url string) bool
	Fetch(ctx context.Context, url string) (*transcript.Transcript, error)
}

// CookieMissingError signals a cookie-tier adapter has no captured cookie
// for its service. The CLI translates this into a remediation hint pointing
// at `auth login --chrome --service <Service>`.
type CookieMissingError struct {
	Service string
	Hint    string
}

func (e *CookieMissingError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("cookie for %q not captured; %s", e.Service, e.Hint)
	}
	return fmt.Sprintf("cookie for %q not captured; run 'podcast-goat-pp-cli auth login --chrome --service %s'", e.Service, e.Service)
}

// KeyMissingError signals a paid adapter is missing its API key.
type KeyMissingError struct {
	EnvVar string
	URL    string
}

func (e *KeyMissingError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("API key not set: %s (get one at %s)", e.EnvVar, e.URL)
	}
	return fmt.Sprintf("API key not set: %s", e.EnvVar)
}

// NotApplicableError signals the adapter recognizes the URL shape but can't
// produce a transcript (no podcast:transcript tag, no episode found, etc.).
// Dispatcher records this and walks to the next adapter.
type NotApplicableError struct {
	Source string
	URL    string
	Reason string
}

func (e *NotApplicableError) Error() string {
	return fmt.Sprintf("%s adapter does not apply to %s: %s", e.Source, e.URL, e.Reason)
}

// RateLimitError signals a per-source limiter blocked the fetch.
type RateLimitError struct {
	Source     string
	RetryAfter int // seconds, 0 if unknown
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s rate-limited (retry after %ds)", e.Source, e.RetryAfter)
	}
	return fmt.Sprintf("%s rate-limited", e.Source)
}

// NotImplementedError signals a deferred-to-v0.2 path.
type NotImplementedError struct {
	Adapter      string
	NeedsCapture bool // true = "first-time browser capture required"
	Detail       string
}

func (e *NotImplementedError) Error() string {
	if e.NeedsCapture {
		return fmt.Sprintf("%s parser is deferred to v0.2 (first-time HTML capture from a logged-in browser session is required to calibrate the parser). %s", e.Adapter, e.Detail)
	}
	return fmt.Sprintf("%s is deferred to v0.2: %s", e.Adapter, e.Detail)
}

// CookiesDir is the standard on-disk location for captured cookie files.
func CookiesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "podcast-goat", "cookies")
}

// CookieFile returns the canonical cookies-<service>.json path.
func CookieFile(service string) string {
	return filepath.Join(CookiesDir(), "cookies-"+service+".json")
}

// HasCookie reports whether a cookies-<service>.json file exists.
func HasCookie(service string) bool {
	st, err := os.Stat(CookieFile(service))
	return err == nil && st.Size() > 2 // not empty {} or []
}
