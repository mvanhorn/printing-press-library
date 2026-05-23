// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParsePlaylistID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"playlist url", "https://www.youtube.com/playlist?list=PLkdrbggVaVRDNyR3b5SlqcI9Z0xJjShTh", "PLkdrbggVaVRDNyR3b5SlqcI9Z0xJjShTh"},
		{"watch url with list", "https://www.youtube.com/watch?v=abc123&list=PLfoo", "PLfoo"},
		{"bare PL id", "PLkdrbggVaVRDNyR3b5SlqcI9Z0xJjShTh", "PLkdrbggVaVRDNyR3b5SlqcI9Z0xJjShTh"},
		{"bare UU id", "UUabcdefghijklmnopqrstuv", "UUabcdefghijklmnopqrstuv"},
		{"url without list param", "https://www.youtube.com/watch?v=abc123", ""},
		{"junk with spaces", "not a playlist", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePlaylistID(tc.in); got != tc.want {
				t.Errorf("parsePlaylistID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseVideoID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"watch url", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"watch url with extra params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLfoo&t=10s", "dQw4w9WgXcQ"},
		{"youtu.be short", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed url", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"shorts url", "https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"bare id", "dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"junk with spaces", "not a video", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseVideoID(tc.in); got != tc.want {
				t.Errorf("parseVideoID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
