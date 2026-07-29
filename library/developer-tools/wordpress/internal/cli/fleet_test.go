// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestNovelFleetHelpWires smoke-tests that the fleet command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFleetHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"fleet", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "fleet"} {
		if !strings.Contains(help, want) {
			t.Fatalf("fleet --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestFleetSiteDBPath(t *testing.T) {
	tests := []struct {
		name       string
		anchor     string
		activeName string
		siteName   string
		active     bool
		want       string
	}{
		{
			name:       "active site keeps exact helper path",
			anchor:     "/tmp/wordpress-pp-cli-alpha-site.db",
			activeName: "Alpha Site",
			siteName:   "Alpha Site",
			active:     true,
			want:       "/tmp/wordpress-pp-cli-alpha-site.db",
		},
		{
			name:       "sibling site replaces active suffix",
			anchor:     "/tmp/wordpress-pp-cli-alpha-site.db",
			activeName: "Alpha Site",
			siteName:   "Beta & Co",
			want:       "/tmp/wordpress-pp-cli-beta-co.db",
		},
		{
			name:     "generic anchor gains site suffix",
			anchor:   "/tmp/wordpress-pp-cli.sqlite",
			siteName: "Mi Sitio",
			want:     "/tmp/wordpress-pp-cli-mi-sitio.sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fleetSiteDBPath(tt.anchor, tt.activeName, tt.siteName, tt.active); got != tt.want {
				t.Fatalf("fleetSiteDBPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFleetSyncAge(t *testing.T) {
	now := time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		lastSync string
		want     string
	}{
		{name: "never synced", lastSync: "", want: "never"},
		{name: "invalid timestamp", lastSync: "not-a-time", want: "unknown"},
		{name: "minutes", lastSync: now.Add(-45 * time.Minute).Format(time.RFC3339), want: "45m"},
		{name: "hours", lastSync: now.Add(-6 * time.Hour).Format(time.RFC3339), want: "6h"},
		{name: "days", lastSync: now.Add(-72 * time.Hour).Format(time.RFC3339), want: "3d"},
		{name: "future clock skew clamps", lastSync: now.Add(time.Hour).Format(time.RFC3339), want: "<1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fleetSyncAge(tt.lastSync, now); got != tt.want {
				t.Fatalf("fleetSyncAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPopulateFleetSiteFromEmptyMirror(t *testing.T) {
	ctx, db := openLocalCommandTestStore(t)
	result := fleetSiteResult{Name: "Empty", Status: "ok"}
	if err := populateFleetSite(ctx, db, &result); err != nil {
		t.Fatalf("populateFleetSite() error = %v", err)
	}
	for name, value := range map[string]*int{
		"posts": result.Posts, "pages": result.Pages, "media": result.Media,
		"users": result.Users, "administrators": result.Administrators,
		"active_plugins": result.ActivePlugins, "total_plugins": result.TotalPlugins,
	} {
		if value == nil || *value != 0 {
			t.Fatalf("populateFleetSite() %s = %v, want pointer to zero", name, value)
		}
	}
	if result.LastSyncAge != "never" {
		t.Fatalf("populateFleetSite() LastSyncAge = %q, want never", result.LastSyncAge)
	}
}
