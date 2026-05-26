// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"
)

func TestHostOf(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://www.example.com/path", "example.com"},
		{"https://EXAMPLE.com", "example.com"},
		{"https://sub.example.com/x", "sub.example.com"},
		{"https://amzn.to/abc", "amzn.to"},
		{"not a url", ""},
	}
	for _, tc := range cases {
		if got := hostOf(tc.in); got != tc.want {
			t.Errorf("hostOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsNoisyHost(t *testing.T) {
	noisy := []string{"instagram.com", "m.facebook.com", "twitter.com", "x.com", "youtube.com", "youtu.be"}
	for _, h := range noisy {
		if !isNoisyHost(h) {
			t.Errorf("isNoisyHost(%q) = false, want true", h)
		}
	}
	clean := []string{"example.com", "github.com", "amzn.to", "nottwitter.com.evil.example"}
	for _, h := range clean {
		if isNoisyHost(h) {
			t.Errorf("isNoisyHost(%q) = true, want false", h)
		}
	}
}

// extractDescriptionLinks with resolve=false performs no network I/O, so it is
// safe to test directly. Covers dedupe, trailing-punctuation trim, shortener
// flagging, and the social-skip filter.
func TestExtractDescriptionLinks(t *testing.T) {
	desc := `Check out my gear: https://amzn.to/3abc.
Repo: https://github.com/foo/bar
Follow me: https://instagram.com/me
Dup: https://github.com/foo/bar
Sentence end (https://example.com/page).`

	t.Run("skips social by default", func(t *testing.T) {
		links := extractDescriptionLinks(context.Background(), desc, false, false)
		var resolved, skipped int
		var sawShortener, sawTrimmed bool
		for _, l := range links {
			if l.Skipped {
				skipped++
				continue
			}
			resolved++
			if l.Host == "amzn.to" && l.Shortener {
				sawShortener = true
			}
			if l.URL == "https://example.com/page" {
				sawTrimmed = true // trailing ). was trimmed
			}
		}
		if !sawShortener {
			t.Error("expected amzn.to flagged as shortener")
		}
		if !sawTrimmed {
			t.Error("expected trailing punctuation trimmed from example.com link")
		}
		if skipped == 0 {
			t.Error("expected instagram.com link to be skipped")
		}
		// github appears twice but must dedupe to one non-skipped link.
		var githubCount int
		for _, l := range links {
			if l.Host == "github.com" {
				githubCount++
			}
		}
		if githubCount != 1 {
			t.Errorf("expected github.com deduped to 1, got %d", githubCount)
		}
	})

	t.Run("include-social keeps social", func(t *testing.T) {
		links := extractDescriptionLinks(context.Background(), desc, false, true)
		var sawInstagram bool
		for _, l := range links {
			if l.Host == "instagram.com" && !l.Skipped {
				sawInstagram = true
			}
		}
		if !sawInstagram {
			t.Error("expected instagram.com kept with include-social")
		}
	})
}
