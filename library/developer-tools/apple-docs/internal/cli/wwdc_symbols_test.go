// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSessionMatches(t *testing.T) {
	cases := []struct {
		name    string
		session string
		refURL  string
		refID   string
		want    bool
	}{
		{
			name:    "exact wwdc2024 URL match",
			session: "wwdc2024-10169",
			refURL:  "https://developer.apple.com/videos/play/wwdc2024/10169/",
			want:    true,
		},
		{
			name:    "wwdc2024 form matches wwdc24 URL via shortening",
			session: "wwdc2024-10169",
			refURL:  "/videos/play/wwdc24/10169",
			want:    true,
		},
		{
			name:    "wwdc24 form matches wwdc2024 URL via expansion",
			session: "wwdc24-10169",
			refURL:  "https://developer.apple.com/videos/play/wwdc2024/10169/",
			want:    true,
		},
		{
			name:    "identifier-only match (no URL)",
			session: "wwdc2024-10169",
			refURL:  "",
			refID:   "doc://com.apple.video/wwdc2024-10169",
			want:    true,
		},
		{
			name:    "no match — different session number",
			session: "wwdc2024-10169",
			refURL:  "https://developer.apple.com/videos/play/wwdc2024/99999/",
			want:    false,
		},
		{
			name:    "no match — different year",
			session: "wwdc2024-10169",
			refURL:  "https://developer.apple.com/videos/play/wwdc2023/10169/",
			want:    false,
		},
		{
			name:    "empty session returns false",
			session: "",
			refURL:  "https://developer.apple.com/videos/play/wwdc2024/10169/",
			want:    false,
		},
		{
			name:    "case-insensitive — uppercase session input",
			session: "WWDC2024-10169",
			refURL:  "https://developer.apple.com/videos/play/wwdc2024/10169/",
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sessionMatches(tc.session, tc.refURL, tc.refID)
			if got != tc.want {
				t.Errorf("sessionMatches(%q, %q, %q) = %v, want %v",
					tc.session, tc.refURL, tc.refID, got, tc.want)
			}
		})
	}
}
