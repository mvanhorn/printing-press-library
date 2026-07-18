// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/client"
)

// wrapAPIError mimics how the report layer surfaces a transport-level HTTP
// failure: a *client.APIError wrapped with contextual prose.
func wrapAPIError(status int) error {
	return fmt.Errorf("transfermarkt player search %q: %w", "someone",
		&client.APIError{Method: "GET", Path: "/players/search/someone", StatusCode: status, Body: "boom"})
}

func TestClassifyAPIError_5xxIsFriendlyUpstream(t *testing.T) {
	for _, status := range []int{500, 502, 503} {
		got := classifyAPIError(wrapAPIError(status), &rootFlags{})
		if code := ExitCode(got); code != 5 {
			t.Fatalf("status %d: exit code = %d, want 5", status, code)
		}
		msg := got.Error()
		if !strings.Contains(msg, "unavailable") || !strings.Contains(msg, "--base-url") {
			t.Fatalf("status %d: hint missing --base-url/unavailable guidance: %q", status, msg)
		}
	}
}

func TestClassifyAPIError_TransportExhaustionIsFriendly(t *testing.T) {
	// All-sources transport failure surfaces as *client.SourceUnavailableError,
	// which must get the same exit-5 outage hint as an all-5xx exhaustion.
	base := fmt.Errorf("transfermarkt player search %q: %w", "someone",
		&client.SourceUnavailableError{Err: errors.New("dial tcp: connection refused")})
	got := classifyAPIError(base, &rootFlags{})
	if code := ExitCode(got); code != 5 {
		t.Fatalf("exit code = %d, want 5", code)
	}
	if msg := got.Error(); !strings.Contains(msg, "unavailable") || !strings.Contains(msg, "--base-url") {
		t.Fatalf("transport-down hint missing --base-url/unavailable guidance: %q", msg)
	}
}

func TestClassifyAPIError_AppNotFoundIsExit3(t *testing.T) {
	// The report layer emits a plain (non-APIError) "player not found: X" when a
	// working source returns zero results. It must map to exit 3, not the 5xx path.
	got := classifyAPIError(fmt.Errorf("player not found: %s", "nobody"), &rootFlags{})
	if code := ExitCode(got); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if strings.Contains(got.Error(), "--base-url") {
		t.Fatalf("not-found must not carry the upstream-down hint: %q", got.Error())
	}
}

func TestClassifyAPIError_HTTP404IsExit3(t *testing.T) {
	got := classifyAPIError(wrapAPIError(404), &rootFlags{})
	if code := ExitCode(got); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
}

func TestClassifyAPIError_HTTP429IsExit7(t *testing.T) {
	got := classifyAPIError(wrapAPIError(429), &rootFlags{})
	if code := ExitCode(got); code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
}

func TestBaseURLFlagRegistered(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	if root.PersistentFlags().Lookup("base-url") == nil {
		t.Fatal("--base-url persistent flag is not registered")
	}
}
