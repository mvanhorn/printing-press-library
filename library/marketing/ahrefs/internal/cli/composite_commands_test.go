// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRefdomainFromURL(t *testing.T) {
	tests := map[string]string{
		"https://www.example.com/path": "example.com",
		"http://Sub.Example.com/a":     "sub.example.com",
		"blog.example.com/page":        "blog.example.com",
		"":                             "",
	}
	for input, want := range tests {
		if got := refdomainFromURL(input); got != want {
			t.Fatalf("refdomainFromURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSnapshotCompactResult(t *testing.T) {
	result := snapshotResult{
		Authority: map[string]any{"domain_rating": 76.5},
		Backlinks: map[string]any{"live_refdomains": 1234},
		Organic:   map[string]any{"org_traffic": 98765},
	}
	compact := compactSnapshotResult(result)
	if compact["domain_rating"] != 76.5 {
		t.Fatalf("domain_rating = %v, want 76.5", compact["domain_rating"])
	}
	if compact["live_refdomains"] != 1234 {
		t.Fatalf("live_refdomains = %v, want 1234", compact["live_refdomains"])
	}
	if compact["org_traffic"] != 98765 {
		t.Fatalf("org_traffic = %v, want 98765", compact["org_traffic"])
	}
}

func TestCompositeCommandsRegisteredReadOnly(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"keyword-gap", "striking-distance", "link-intersect", "snapshot"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("finding %s: %v", name, err)
		}
		if cmd == nil || cmd.Use != name {
			t.Fatalf("root.Find(%s) returned %#v", name, cmd)
		}
		if got := cmd.Annotations["mcp:read-only"]; got != "true" {
			t.Fatalf("%s mcp:read-only annotation = %q, want true", name, got)
		}
	}
}
