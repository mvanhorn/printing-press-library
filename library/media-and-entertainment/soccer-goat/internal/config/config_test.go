// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"reflect"
	"testing"
)

// clearBaseURLEnv wipes both base-URL env vars for a hermetic resolution test.
func clearBaseURLEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SOCCER_GOAT_BASE_URL", "")
	t.Setenv("SOCCER_GOAT_BASE_URLS", "")
}

func TestResolveBaseURLs_DefaultList(t *testing.T) {
	clearBaseURLEnv(t)
	c := &Config{}
	c.resolveBaseURLs()
	if !reflect.DeepEqual(c.BaseURLs, defaultBaseURLs) {
		t.Fatalf("BaseURLs = %v, want default %v", c.BaseURLs, defaultBaseURLs)
	}
	if c.BaseURL != defaultBaseURLs[0] {
		t.Fatalf("primary BaseURL = %q, want %q", c.BaseURL, defaultBaseURLs[0])
	}
}

func TestResolveBaseURLs_EnvSingleBeatsEnvList(t *testing.T) {
	t.Setenv("SOCCER_GOAT_BASE_URL", "https://single.example")
	t.Setenv("SOCCER_GOAT_BASE_URLS", "https://a.example,https://b.example")
	c := &Config{}
	c.resolveBaseURLs()
	if want := []string{"https://single.example"}; !reflect.DeepEqual(c.BaseURLs, want) {
		t.Fatalf("BaseURLs = %v, want %v (single env override collapses to one source)", c.BaseURLs, want)
	}
}

func TestResolveBaseURLs_FileSingle(t *testing.T) {
	clearBaseURLEnv(t)
	c := &Config{BaseURL: "https://file.example"}
	c.resolveBaseURLs()
	if want := []string{"https://file.example"}; !reflect.DeepEqual(c.BaseURLs, want) {
		t.Fatalf("BaseURLs = %v, want %v", c.BaseURLs, want)
	}
}

func TestResolveBaseURLs_EnvList(t *testing.T) {
	t.Setenv("SOCCER_GOAT_BASE_URL", "")
	t.Setenv("SOCCER_GOAT_BASE_URLS", "https://a.example, https://b.example ,https://c.example")
	c := &Config{}
	c.resolveBaseURLs()
	want := []string{"https://a.example", "https://b.example", "https://c.example"}
	if !reflect.DeepEqual(c.BaseURLs, want) {
		t.Fatalf("BaseURLs = %v, want %v", c.BaseURLs, want)
	}
	if c.BaseURL != "https://a.example" {
		t.Fatalf("primary = %q, want first list entry", c.BaseURL)
	}
}

func TestResolveBaseURLs_FileList(t *testing.T) {
	clearBaseURLEnv(t)
	c := &Config{BaseURLs: []string{"https://a.example", "https://b.example"}}
	c.resolveBaseURLs()
	want := []string{"https://a.example", "https://b.example"}
	if !reflect.DeepEqual(c.BaseURLs, want) {
		t.Fatalf("BaseURLs = %v, want %v", c.BaseURLs, want)
	}
	if c.BaseURL != "https://a.example" {
		t.Fatalf("primary = %q, want first file-list entry", c.BaseURL)
	}
}

func TestSetBaseURLOverride_WinsOverList(t *testing.T) {
	c := &Config{BaseURLs: append([]string(nil), defaultBaseURLs...)}
	c.SetBaseURLOverride("  https://flag.example  ")
	if want := []string{"https://flag.example"}; !reflect.DeepEqual(c.BaseURLs, want) {
		t.Fatalf("BaseURLs = %v, want %v", c.BaseURLs, want)
	}
	if c.BaseURL != "https://flag.example" {
		t.Fatalf("primary = %q, want the override", c.BaseURL)
	}
}

func TestSetBaseURLOverride_EmptyIsNoop(t *testing.T) {
	c := &Config{BaseURLs: []string{"https://keep.example"}, BaseURL: "https://keep.example"}
	c.SetBaseURLOverride("   ")
	if c.BaseURL != "https://keep.example" {
		t.Fatalf("empty override should be a no-op, got %q", c.BaseURL)
	}
}

func TestSplitBaseURLs(t *testing.T) {
	got := splitBaseURLs("https://a.example, ,https://b.example,")
	want := []string{"https://a.example", "https://b.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBaseURLs = %v, want %v", got, want)
	}
}
