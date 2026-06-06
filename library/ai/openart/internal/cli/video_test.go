// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestMediaExtForURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://cdn.openart.ai/r/abc123.mp4", ".mp4"},
		{"https://cdn.openart.ai/r/abc123.png", ".png"},
		{"https://cdn.openart.ai/r/abc123.PNG?Expires=123&Signature=x", ".png"},
		{"https://cdn.openart.ai/r/abc123.webp", ".webp"},
		{"https://cdn.openart.ai/r/abc123.jpg#frag", ".jpg"},
		{"https://cdn.openart.ai/r/abc123.webm", ".webm"},
		{"https://cdn.openart.ai/r/abc123", ".mp4"}, // extension-less video CDN URL
		{"", ".mp4"},
	}
	for _, c := range cases {
		if got := mediaExtForURL(c.url); got != c.want {
			t.Errorf("mediaExtForURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
