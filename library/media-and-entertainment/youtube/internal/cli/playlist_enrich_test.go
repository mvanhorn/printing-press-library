// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

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
		{"live url", "https://www.youtube.com/live/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"bare id", "dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		// Scheme-less URLs are common copy-paste shapes and must parse too
		// (library issue #875): without a "://" they used to fall through to
		// the bare-ID check and fail the 11-char regex.
		{"scheme-less youtu.be", "youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"scheme-less youtu.be with si param", "youtu.be/dQw4w9WgXcQ?si=yZha3HjrYqLmc5et", "dQw4w9WgXcQ"},
		{"scheme-less www.youtube.com watch", "www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"scheme-less youtube.com watch", "youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"scheme-less embed", "www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"scheme-less watch with t param", "www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s", "dQw4w9WgXcQ"},
		{"scheme-less m.youtube.com watch", "m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"m.youtube.com watch", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"junk with spaces", "not a video", ""},
		{"empty", "", ""},
		// Non-video URLs must return "" so the caller errors clearly rather
		// than passing a bogus ID to videos.list (Greptile #803).
		{"channel /c/ url", "https://www.youtube.com/c/SomeChannel", ""},
		{"channel /channel/ url", "https://www.youtube.com/channel/UCabcdefghijklmnopqrstuv", ""},
		{"user url", "https://www.youtube.com/user/SomeUser", ""},
		{"playlist url", "https://www.youtube.com/playlist?list=PLfoo", ""},
		{"handle url", "https://www.youtube.com/@SomeHandle", ""},
		{"too-short bare token", "abc", ""},
		{"too-long bare token", "dQw4w9WgXcQ123", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseVideoID(tc.in); got != tc.want {
				t.Errorf("parseVideoID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
