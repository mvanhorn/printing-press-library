// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseCookieString(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string]string
		absent  []string
		comment string
	}{
		{
			name: "space separated",
			in:   "a=1; b=2; c=3",
			want: map[string]string{"a": "1", "b": "2", "c": "3"},
		},
		{
			// barnardb/cookies emits ";" with no trailing space. Splitting on
			// "; " collapsed the whole header into a single entry.
			name: "no space after semicolon",
			in:   "a=1;b=2;c=3",
			want: map[string]string{"a": "1", "b": "2", "c": "3"},
		},
		{
			name: "mixed separators",
			in:   "a=1; b=2;c=3",
			want: map[string]string{"a": "1", "b": "2", "c": "3"},
		},
		{
			// Chrome stores several alaskaair.com cookie names percent-encoded.
			// Callers look them up by canonical name.
			name: "percent encoded names get a decoded alias",
			in:   "AS%5FACNT=acnt;AS%5FNAME=name;as%5Fpers=pers",
			want: map[string]string{
				"AS%5FACNT": "acnt", "AS_ACNT": "acnt",
				"AS%5FNAME": "name", "AS_NAME": "name",
				"as%5Fpers": "pers", "as_pers": "pers",
			},
		},
		{
			name: "raw name wins over decoded alias",
			in:   "AS_ACNT=raw;AS%5FACNT=encoded",
			want: map[string]string{"AS_ACNT": "raw", "AS%5FACNT": "encoded"},
		},
		{
			name: "value containing = is preserved",
			in:   "token=abc=def==",
			want: map[string]string{"token": "abc=def=="},
		},
		{
			name:   "malformed pairs are skipped",
			in:     "a=1;;novalue;=orphan;b=2",
			want:   map[string]string{"a": "1", "b": "2"},
			absent: []string{"", "novalue"},
		},
		{
			name: "empty input",
			in:   "",
			want: map[string]string{},
		},
		{
			// A real Alaska session mixes both quirks at once.
			name: "regression: alaska session shape",
			in:   "guestsession=gs;AS%5FACNT=acnt;ASSession=sess;AS%5FNAME=nm;as%5Fpers=pr;guestidentity=gi;ASSessionSSL=ssl",
			want: map[string]string{
				"AS_ACNT": "acnt", "AS_NAME": "nm", "as_pers": "pr",
				"guestsession": "gs", "guestidentity": "gi",
				"ASSession": "sess", "ASSessionSSL": "ssl",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCookieString(tt.in)
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("parseCookieString(%q)[%q] = %q, want %q", tt.in, k, got[k], want)
				}
			}
			for _, k := range tt.absent {
				if _, ok := got[k]; ok {
					t.Errorf("parseCookieString(%q) unexpectedly contains key %q", tt.in, k)
				}
			}
		})
	}
}

// TestParseCookieStringSatisfiesRequiredAuthCookies asserts that a session in
// the shape Chrome actually stores resolves every cookie auth login depends on.
func TestParseCookieStringSatisfiesRequiredAuthCookies(t *testing.T) {
	// Percent-encoded names, no space after the semicolon: what the
	// `cookies` backend hands us for a logged-in alaskaair.com session.
	session := "AS%5FACNT=1;AS%5FNAME=2;guestsession=3;guestidentity=4;ASSession=5;ASSessionSSL=6;as%5Fpers=7"

	got := parseCookieString(session)
	for _, name := range requiredAuthCookies() {
		if _, ok := got[name]; !ok {
			t.Errorf("required auth cookie %q not resolved from session string", name)
		}
	}
}
