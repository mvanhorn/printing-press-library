// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"reflect"
	"testing"
)

func TestQueryTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"single", "deploy", []string{"deploy"}},
		{"lowercases", "Deploy ROLLBACK", []string{"deploy", "rollback"}},
		{"strips punctuation", "deploy, rollback!", []string{"deploy", "rollback"}},
		{"keeps digits and underscores", "release_2 v3", []string{"release_2", "v3"}},
		{"punctuation only", "--- ???", []string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := QueryTokens(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("QueryTokens(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatchesAllTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		text  string
		query string
		want  bool
	}{
		{"exact", "the deploy is green", "deploy", true},
		{"case insensitive", "The DEPLOY is green", "deploy", true},
		{"substring of longer word", "deployment finished", "deploy", true},
		{"missing token", "the release is green", "deploy", false},
		{"all tokens present", "deploy rollback plan", "deploy rollback", true},
		{"one token missing", "deploy plan", "deploy rollback", false},
		{"empty query matches nothing", "deploy", "", false},
		{"empty text", "", "deploy", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchesAllTokens(tc.text, tc.query); got != tc.want {
				t.Fatalf("MatchesAllTokens(%q, %q) = %v, want %v", tc.text, tc.query, got, tc.want)
			}
		})
	}
}

func TestFTSMatchQuery(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"punctuation only", "!!!", ""},
		{"single token", "deploy", `"deploy"`},
		{"two tokens", "deploy rollback", `"deploy" "rollback"`},
		{"lowercases", "Deploy", `"deploy"`},
		{"neutralises fts syntax", "deploy OR *", `"deploy" "or"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FTSMatchQuery(tc.in); got != tc.want {
				t.Fatalf("FTSMatchQuery(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSnippet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		max  int
		want string
	}{
		{"short passthrough", "hello there", 40, "hello there"},
		{"collapses whitespace", "hello\n\t there", 40, "hello there"},
		{"truncates", "abcdefghij", 5, "abcde…"},
		{"no limit", "abcdefghij", 0, "abcdefghij"},
		{"exact length", "abcde", 5, "abcde"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Snippet(tc.text, tc.max); got != tc.want {
				t.Fatalf("Snippet(%q, %d) = %q, want %q", tc.text, tc.max, got, tc.want)
			}
		})
	}
}
