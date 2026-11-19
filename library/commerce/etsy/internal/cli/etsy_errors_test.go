package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/platform"
)

func TestClassifyEtsyError(t *testing.T) {
	tests := []struct {
		name, body, kind string
		status           int
		retry            time.Duration
		markers          []string
	}{
		{"format", `{"error":"Invalid API key: should be in the format 'keystring:shared_secret'"}`, "api_key_format", 403, 0, []string{"ETSY_SHARED_SECRET"}},
		{"inactive", `{"error":"API key not found or not active, or incorrect shared secret"}`, "api_key_inactive", 403, 0, []string{"swapped"}},
		{"secret", "incorrect shared secret", "api_key_inactive", 403, 0, []string{"swapped"}},
		{"scope", `{"error":"The access token does not have the required scope: listings_w"}`, "missing_scope", 403, 0, []string{"listings_w", "auth login --scopes"}},
		{"two scopes", "scope requires transactions_r and email_r and transactions_r", "missing_scope", 403, 0, []string{"scope(s): transactions_r, email_r.", "<existing scopes>,transactions_r,email_r"}},
		{"no token", "required scope missing", "missing_scope", 403, 0, []string{"scope listed in the Etsy docs", "auth login --help"}},
		{"unrelated", `{"error":"forbidden"}`, "", 403, 0, nil},
		{"retry", "", "rate_limited", 429, 30 * time.Second, []string{"30s"}},
		{"budget", "", "rate_limited", 429, 0, []string{"doctor"}},
		{"server", "", "", 500, 0, nil},
		{"format precedence", "SCOPE SHOULD BE IN THE FORMAT", "api_key_format", 403, 0, []string{"ETSY_SHARED_SECRET"}},
		{"inactive precedence", "SCOPE NOT FOUND OR NOT ACTIVE", "api_key_inactive", 403, 0, []string{"swapped"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := classifyEtsyError(tt.status, tt.body, tt.retry)
			if ok != (tt.kind != "") || got.Kind != tt.kind {
				t.Fatalf("got %+v, %v; want kind %q", got, ok, tt.kind)
			}
			for _, marker := range tt.markers {
				if !strings.Contains(got.Fix, marker) {
					t.Errorf("fix %q missing %q", got.Fix, marker)
				}
			}
			if tt.name == "two scopes" {
				// Each scope appears once in the explanation and once in the command.
				for _, scope := range []string{"transactions_r", "email_r"} {
					if strings.Count(got.Fix, scope) != 2 {
						t.Errorf("scope not deduplicated: %s", got.Fix)
					}
				}
			}
		})
	}
}

func TestEtsyErrorHintAndExitCodes(t *testing.T) {
	api := &client.APIError{Method: "GET", Path: "/test", StatusCode: 403, Body: "Invalid API key: should be in the format 'keystring:shared_secret'"}
	wrapped := fmt.Errorf("wrapped: %w", api)
	if hint := etsyErrorHint(wrapped); !strings.HasPrefix(hint, "hint: ") || !strings.Contains(hint, "ETSY_SHARED_SECRET") {
		t.Fatalf("hint = %q", hint)
	}
	if hint := etsyErrorHint(errors.New("plain")); hint != "" {
		t.Fatalf("plain hint = %q", hint)
	}
	got := classifyAPIErrorOnly(api)
	if !strings.Contains(got.Error(), "ETSY_SHARED_SECRET") || ExitCode(got) != ExitCode(authErr(errors.New("x"))) {
		t.Fatalf("classified auth error: %v, code %d", got, ExitCode(got))
	}
	rate := &client.APIError{Method: "GET", Path: "/test", StatusCode: 429}
	for _, err := range []error{rate, fmt.Errorf("wrapped: %w", &platform.RateLimitedError{RetryAfter: 30 * time.Second, Cause: rate})} {
		got := classifyAPIErrorOnly(err)
		if ExitCode(got) != ExitCode(rateLimitErr(errors.New("x"))) || !strings.Contains(got.Error(), "hint: Etsy rate limit hit") {
			t.Fatalf("classified rate error: %v, code %d", got, ExitCode(got))
		}
	}
	if hint := etsyErrorHint(&platform.RateLimitedError{RetryAfter: 30 * time.Second}); !strings.Contains(hint, "30s") {
		t.Fatalf("retry hint = %q", hint)
	}
}
