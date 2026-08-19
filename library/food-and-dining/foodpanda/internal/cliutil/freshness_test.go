package cliutil

import (
	"testing"
	"time"
)

func TestEnsureFresh_NeverSyncedIsNotStale(t *testing.T) {
	// A resource with no completed sync must not be reported stale: there is
	// no prior run to reproduce, so a refresh would be a guess about scope
	// rather than a repeat of something the user chose.
	v := EnsureFresh(time.Time{}, time.Hour)
	if v.Stale {
		t.Fatalf("never-synced reported stale; want not stale (reason %q)", v.Reason)
	}
	if v.Reason != "never synced" {
		t.Fatalf("Reason = %q, want %q", v.Reason, "never synced")
	}
}

func TestEnsureFresh_WithinThresholdIsFresh(t *testing.T) {
	v := EnsureFresh(time.Now().Add(-30*time.Minute), 6*time.Hour)
	if v.Stale {
		t.Fatalf("30m-old data reported stale against a 6h threshold")
	}
}

func TestEnsureFresh_BeyondThresholdIsStale(t *testing.T) {
	v := EnsureFresh(time.Now().Add(-7*time.Hour), 6*time.Hour)
	if !v.Stale {
		t.Fatalf("7h-old data not reported stale against a 6h threshold")
	}
	if v.Age < 6*time.Hour {
		t.Fatalf("Age = %v, want >= 6h", v.Age)
	}
}

func TestEnsureFresh_NonPositiveThresholdFallsBackToDefault(t *testing.T) {
	// A zero threshold must not mean "everything is stale" — that would make
	// a misconfigured caller refresh on every single command.
	v := EnsureFresh(time.Now().Add(-1*time.Minute), 0)
	if v.Stale {
		t.Fatalf("1m-old data reported stale under the default %v threshold", DefaultStaleAfter)
	}
}

func TestStaleAfterFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  string
		want time.Duration
	}{
		{"unset falls back", "", DefaultStaleAfter},
		{"valid duration honored", "15m", 15 * time.Minute},
		{"garbage falls back", "not-a-duration", DefaultStaleAfter},
		{"negative falls back", "-5m", DefaultStaleAfter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FOODPANDA_STALE_AFTER", tc.set)
			if got := StaleAfterFromEnv(); got != tc.want {
				t.Fatalf("StaleAfterFromEnv() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAutoRefreshEnabled_DefaultsOff(t *testing.T) {
	t.Setenv("FOODPANDA_AUTO_REFRESH", "")
	if AutoRefreshEnabled() {
		t.Fatal("auto-refresh enabled with no env var set; it must be opt-in")
	}
	for _, on := range []string{"1", "true", "YES", "on"} {
		t.Setenv("FOODPANDA_AUTO_REFRESH", on)
		if !AutoRefreshEnabled() {
			t.Fatalf("AutoRefreshEnabled() = false for %q", on)
		}
	}
	t.Setenv("FOODPANDA_AUTO_REFRESH", "0")
	if AutoRefreshEnabled() {
		t.Fatal("AutoRefreshEnabled() = true for \"0\"")
	}
}
