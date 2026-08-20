// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWarnUnmatchedChannelFilter(t *testing.T) {
	channels := map[string]localChannel{
		"C0GENERAL": {ID: "C0GENERAL", Name: "general"},
		"C0DEPLOY":  {ID: "C0DEPLOY", Name: "deploys"},
	}
	// C0ORPHAN carries messages but its conversation record was never synced:
	// a real partial-mirror shape, and the case a naive "is it in channels?"
	// check would wrongly warn about.
	messages := []localMessage{
		{Channel: "C0GENERAL", TS: "1"},
		{Channel: "C0ORPHAN", TS: "2"},
	}

	cases := []struct {
		name     string
		filter   string
		wantWarn bool
	}{
		{"empty filter never warns", "", false},
		{"known id", "C0GENERAL", false},
		{"known name with hash", "#general", false},
		{"known bare name", "deploys", false},
		{"case-insensitive name", "#GENERAL", false},
		{"id present only in messages", "C0ORPHAN", false},
		{"typo'd name", "#genral", true},
		{"unknown id", "C0NOPE", true},
		{"name of a channel that was never mirrored", "random", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := warnUnmatchedChannelFilter(&buf, tc.filter, channels, messages)
			if got != tc.wantWarn {
				t.Fatalf("warnUnmatchedChannelFilter(%q) = %t; want %t (stderr: %q)", tc.filter, got, tc.wantWarn, buf.String())
			}
			if tc.wantWarn {
				// The warning has to name the offending value, or the reader
				// still cannot tell which of their flags was wrong.
				if !strings.Contains(buf.String(), tc.filter) {
					t.Errorf("warning does not name the filter %q: %q", tc.filter, buf.String())
				}
			} else if buf.Len() != 0 {
				t.Errorf("wrote to stderr for a matching filter %q: %q", tc.filter, buf.String())
			}
		})
	}
}

// A mirror with no conversation records at all must still not warn about an
// ID that its messages reference — otherwise every pre-`sync` archive user
// gets a spurious warning on a filter that works.
func TestWarnUnmatchedChannelFilterWithNoChannelRecords(t *testing.T) {
	var buf bytes.Buffer
	messages := []localMessage{{Channel: "C0ONLY", TS: "1"}}
	if warnUnmatchedChannelFilter(&buf, "C0ONLY", nil, messages) {
		t.Fatalf("warned for an ID present in messages: %q", buf.String())
	}
	if !warnUnmatchedChannelFilter(&buf, "#general", nil, messages) {
		t.Fatal("did not warn for a name no mirrored channel carries")
	}
}

func TestNewTextRendererResolvesFromMirror(t *testing.T) {
	users := map[string]localUser{
		"U0ALICE": {ID: "U0ALICE", DisplayName: "Alice Adams"},
		"U0BARE":  {ID: "U0BARE"},
	}
	channels := map[string]localChannel{"C0GENERAL": {ID: "C0GENERAL", Name: "general"}}
	r := newTextRenderer(users, channels)

	cases := map[string]string{
		"<@U0ALICE> shipped to <#C0GENERAL>": "@Alice Adams shipped to #general",
		// A user record with no name at all still resolves — to its own ID,
		// via UserIdentity.DisplayLabel — so output never renders a bare "@".
		"<@U0BARE> replied":  "@U0BARE replied",
		"<@U0GHOST> replied": "@U0GHOST replied",
		"in <#C0GHOST>":      "in #C0GHOST",
		"plain text":         "plain text",
	}
	for in, want := range cases {
		if got := r.Render(in); got != want {
			t.Errorf("Render(%q) = %q; want %q", in, got, want)
		}
	}
}
