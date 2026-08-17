package fares

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fixedNow is the constant "now" used across all freshness tests.
var fixedNow = time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)

func TestCheckFreshness(t *testing.T) {
	// Real-shaped HTTP-date values used across cases.
	const (
		metaLastModified  = "Tue, 16 Jun 2026 21:35:05 GMT"
		newerLastModified = "Wed, 17 Jun 2026 10:00:00 GMT"
	)
	// A LastModified equal to metaLastModified (probe returns same).
	const equalLastModified = metaLastModified

	cases := []struct {
		name string
		// seed: if nil, no WriteMeta call (empty store)
		meta    *FeedMeta
		offline bool
		// probe: optional override. If nil, the real var is left untouched.
		probe        func(ctx context.Context, token, user, pass string) (string, error)
		wantOK       bool
		wantStale    bool
		wantReason   string // exact match required when non-empty
		wantWarning  string // "nonempty" means assert != "", "" means assert == ""
		nonemptyWarn bool   // true: assert Warning != ""; false: assert Warning == wantWarning exactly
	}{
		{
			name:       "A empty store",
			meta:       nil,
			offline:    false,
			wantOK:     false,
			wantStale:  false,
			wantReason: "not synced",
		},
		{
			name: "B aged 40d offline=false",
			meta: &FeedMeta{
				LastModified: metaLastModified,
				PublishDate:  "2026-05-15",
				SyncedAt:     fixedNow.Add(-40 * 24 * time.Hour).Format(time.RFC3339),
			},
			offline: false,
			// Probe must NOT be called for case B. If it is called, fail the test.
			probe: func(ctx context.Context, token, user, pass string) (string, error) {
				t.Fatal("case B: freshnessProbe must not be called when age backstop already decides staleness")
				return "", nil
			},
			wantOK:    false,
			wantStale: true,
		},
		{
			name: "C fresh probe newer offline=false",
			meta: &FeedMeta{
				LastModified: metaLastModified,
				PublishDate:  "2026-06-16",
				SyncedAt:     fixedNow.Add(-24 * time.Hour).Format(time.RFC3339),
			},
			offline: false,
			probe: func(ctx context.Context, token, user, pass string) (string, error) {
				return newerLastModified, nil
			},
			wantOK:    false,
			wantStale: true,
		},
		{
			name: "D fresh probe equal offline=false",
			meta: &FeedMeta{
				LastModified: metaLastModified,
				PublishDate:  "2026-06-16",
				SyncedAt:     fixedNow.Add(-24 * time.Hour).Format(time.RFC3339),
			},
			offline: false,
			probe: func(ctx context.Context, token, user, pass string) (string, error) {
				return equalLastModified, nil
			},
			wantOK:       true,
			wantStale:    false,
			wantWarning:  "",
			nonemptyWarn: false,
		},
		{
			name: "E aged 40d offline=true",
			meta: &FeedMeta{
				LastModified: metaLastModified,
				PublishDate:  "2026-05-15",
				SyncedAt:     fixedNow.Add(-40 * 24 * time.Hour).Format(time.RFC3339),
			},
			offline:      true,
			wantOK:       true,
			wantStale:    true,
			nonemptyWarn: true,
		},
		{
			name: "F fresh probe errors offline=false",
			meta: &FeedMeta{
				LastModified: metaLastModified,
				PublishDate:  "2026-06-16",
				SyncedAt:     fixedNow.Add(-24 * time.Hour).Format(time.RFC3339),
			},
			offline: false,
			probe: func(ctx context.Context, token, user, pass string) (string, error) {
				return "", errors.New("network unreachable")
			},
			wantOK:       true,
			wantStale:    false,
			nonemptyWarn: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			if err := EnsureSchema(db); err != nil {
				t.Fatalf("EnsureSchema: %v", err)
			}

			if tc.meta != nil {
				if err := WriteMeta(db, *tc.meta); err != nil {
					t.Fatalf("WriteMeta: %v", err)
				}
			}

			// Override freshnessProbe if the case provides one.
			if tc.probe != nil {
				orig := freshnessProbe
				freshnessProbe = tc.probe
				defer func() { freshnessProbe = orig }()
			}

			ctx := context.Background()
			result, err := CheckFreshness(ctx, db, "", "", "", tc.offline, fixedNow)
			if err != nil {
				t.Fatalf("CheckFreshness returned unexpected error: %v", err)
			}

			if result.OK != tc.wantOK {
				t.Errorf("OK: want %v, got %v", tc.wantOK, result.OK)
			}
			if result.Stale != tc.wantStale {
				t.Errorf("Stale: want %v, got %v", tc.wantStale, result.Stale)
			}
			if tc.wantReason != "" && result.Reason != tc.wantReason {
				t.Errorf("Reason: want %q, got %q", tc.wantReason, result.Reason)
			}
			if tc.nonemptyWarn {
				if result.Warning == "" {
					t.Errorf("Warning: want non-empty, got empty string")
				}
			} else {
				if result.Warning != tc.wantWarning {
					t.Errorf("Warning: want %q, got %q", tc.wantWarning, result.Warning)
				}
			}
		})
	}
}
