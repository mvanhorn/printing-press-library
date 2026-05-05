// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestEnforceCommandPolicy_RuntimeAllowlist(t *testing.T) {
	if err := EnforceCommandPolicy("albums.list", "albums", ""); err != nil {
		t.Fatalf("parent allow should permit child command: %v", err)
	}
	if err := EnforceCommandPolicy("picker.delete-session", "albums", ""); err == nil {
		t.Fatalf("unlisted command should be blocked when allowlist is set")
	}
}

func TestEnforceCommandPolicy_RuntimeDenylistWins(t *testing.T) {
	err := EnforceCommandPolicy("albums.list", "albums", "albums.list")
	if err == nil {
		t.Fatalf("denylist should block even when allowlist permits")
	}
}

func TestNormalizeCommandPath(t *testing.T) {
	got := normalizeCommandPath(" media-items  batch-get ")
	if got != "media-items.batch-get" {
		t.Fatalf("normalizeCommandPath = %q, want media-items.batch-get", got)
	}
}
