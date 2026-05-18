// PATCH (fix-fundamentals-dry-run-url-506): regression guard for the dry-run
// URL preview. Issue #506 reported that --dry-run printed every query param
// prefixed with `?`, suggesting the live URL was malformed. The live HTTP path
// always used net/http's q.Encode(); only the preview was wrong. These tests
// pin the corrected behavior so a future regen or hand-edit cannot quietly
// reintroduce the per-param `?` prefix.

package client

import (
	"strings"
	"testing"
)

func TestDryRunURLJoinsParamsWithAmpersand(t *testing.T) {
	got := dryRunDisplayURL(
		"https://query1.finance.yahoo.com/ws/fundamentals/v1/finance/timeseries/NVDA",
		map[string]string{
			"type":    "annualTotalRevenue,quarterlyTotalRevenue",
			"period1": "1683911800",
		},
		"c60pJpDxZC8",
	)
	// Exactly one `?` separator — every subsequent param must join with `&`.
	if strings.Count(got, "?") != 1 {
		t.Fatalf("expected exactly one `?` in URL, got %d in %q", strings.Count(got, "?"), got)
	}
	for _, key := range []string{"type=", "period1=", "crumb="} {
		if !strings.Contains(got, key) {
			t.Errorf("expected %q in URL, got %q", key, got)
		}
	}
}

func TestDryRunURLOmitsQueryStringWhenNoParams(t *testing.T) {
	got := dryRunDisplayURL("https://example.com/path", nil, "")
	if got != "https://example.com/path" {
		t.Errorf("expected URL unchanged when no params/crumb, got %q", got)
	}
}

func TestDryRunURLSkipsEmptyParamValues(t *testing.T) {
	// Mirrors the real-request path which also drops empty-string values.
	got := dryRunDisplayURL(
		"https://example.com/path",
		map[string]string{"keep": "yes", "skip": ""},
		"",
	)
	if !strings.Contains(got, "keep=yes") {
		t.Errorf("expected keep=yes in URL, got %q", got)
	}
	if strings.Contains(got, "skip=") {
		t.Errorf("expected skip= to be dropped, got %q", got)
	}
}

func TestDryRunURLIncludesCrumbWhenNoOtherParams(t *testing.T) {
	got := dryRunDisplayURL("https://example.com/path", nil, "abc123")
	if got != "https://example.com/path?crumb=abc123" {
		t.Errorf("expected crumb-only query, got %q", got)
	}
}
