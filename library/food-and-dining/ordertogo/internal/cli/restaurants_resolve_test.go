// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestMatchRestID_BareArray(t *testing.T) {
	data := []byte(`[{"name":"mixsushibar","id":69},{"name":"mixsushibarlin","id":72}]`)
	got, err := matchRestID(data, "mixsushibarlin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "72" {
		t.Fatalf("want 72, got %q", got)
	}
}

func TestMatchRestID_Envelope(t *testing.T) {
	data := []byte(`{"results":[{"name":"crepe","id":31},{"name":"mixsushibarlin","id":72}]}`)
	got, err := matchRestID(data, "mixsushibarlin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "72" {
		t.Fatalf("want 72, got %q", got)
	}
}

func TestMatchRestID_NotFound(t *testing.T) {
	data := []byte(`[{"name":"crepe","id":31}]`)
	got, err := matchRestID(data, "mixsushibarlin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "" {
		t.Fatalf("want empty for missing slug, got %q", got)
	}
}

func TestMatchRestID_ExactSlugOnly(t *testing.T) {
	// A prefix collision must not match: mixsushibar (69) != mixsushibarlin.
	data := []byte(`[{"name":"mixsushibar","id":69},{"name":"mixsushibarlin","id":72}]`)
	got, _ := matchRestID(data, "mixsushibar")
	if got != "69" {
		t.Fatalf("want 69 for exact slug, got %q", got)
	}
}
