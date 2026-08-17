// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import "testing"

func TestWebBaseURL(t *testing.T) {
	cases := []struct {
		base string
		want string
	}{
		{"https://app.dreaming.com/.netlify/functions", "https://app.dreaming.com"},
		{"https://app.dreaming.com/.netlify/functions/", "https://app.dreaming.com"},
		{"https://www.dreamingspanish.com/.netlify/functions", "https://www.dreamingspanish.com"},
		{"https://app.dreaming.com", "https://app.dreaming.com"},
		{"https://app.dreaming.com/", "https://app.dreaming.com"},
	}
	for _, c := range cases {
		cfg := &Config{BaseURL: c.base}
		if got := cfg.WebBaseURL(); got != c.want {
			t.Errorf("WebBaseURL(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}

func TestVideoWebURL(t *testing.T) {
	es := &Config{BaseURL: "https://app.dreaming.com/.netlify/functions", Language: "es"}
	if got := es.VideoWebURL("abc123"); got != "https://app.dreaming.com/spanish/watch?id=abc123" {
		t.Errorf("es VideoWebURL = %q", got)
	}
	fr := &Config{BaseURL: "https://app.dreaming.com/.netlify/functions", Language: "fr"}
	if got := fr.VideoWebURL("abc123"); got != "https://app.dreaming.com/french/watch?id=abc123" {
		t.Errorf("fr VideoWebURL = %q", got)
	}
	// Empty language defaults to spanish.
	def := &Config{BaseURL: "https://app.dreaming.com/.netlify/functions"}
	if got := def.VideoWebURL("x"); got != "https://app.dreaming.com/spanish/watch?id=x" {
		t.Errorf("default-lang VideoWebURL = %q", got)
	}
	if got := es.VideoWebURL(""); got != "" {
		t.Errorf("VideoWebURL(\"\") = %q, want empty", got)
	}
}
