// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestDecodeSlackEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		errHas  string
	}{
		{"ok true with messages", `{"ok":true,"messages":[{"ts":"1.0","text":"hi"}]}`, false, ""},
		{"ok true empty", `{"ok":true,"messages":[]}`, false, ""},
		{"ok false missing_scope", `{"ok":false,"error":"missing_scope"}`, true, "missing_scope"},
		{"ok false channel_not_found", `{"ok":false,"error":"channel_not_found"}`, true, "channel_not_found"},
		{"ok false no code", `{"ok":false}`, true, "no error code"},
		{"absent ok field is a failure", `{"messages":[]}`, true, ""},
		{"malformed", `not json`, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := decodeSlackEnvelope(json.RawMessage(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s, got env=%+v", tt.body, env)
				}
				if tt.errHas != "" && !contains(err.Error(), tt.errHas) {
					t.Fatalf("error %q should mention %q", err.Error(), tt.errHas)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// An ok:false body must never reach the store. This is the regression guard
// for the prior published CLI's bug, where the error object was persisted as
// if it were a record and counted as a successful sync.
func TestDecodeSlackEnvelopeRefusesErrorBodyAsData(t *testing.T) {
	_, err := decodeSlackEnvelope(json.RawMessage(`{"ok":false,"error":"missing_scope"}`))
	if err == nil {
		t.Fatal("ok:false must be an error, never storable data")
	}
}

func TestInjectChannel(t *testing.T) {
	raw := json.RawMessage(`{"ts":"1717171717.000100","text":"deploy failed","user":"U1"}`)
	out, ts, err := injectChannel(raw, "C0ABC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "1717171717.000100" {
		t.Fatalf("ts = %q, want the message ts", ts)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// The local readers resolve a message's channel from $.channel; without
	// this stamp every history message is unattributable.
	if m["channel"] != "C0ABC" {
		t.Fatalf("channel = %v, want C0ABC", m["channel"])
	}
	if m["text"] != "deploy failed" {
		t.Fatalf("original fields must survive; text = %v", m["text"])
	}
}

func TestInjectChannelRejectsMessageWithoutTS(t *testing.T) {
	if _, _, err := injectChannel(json.RawMessage(`{"text":"no ts"}`), "C0ABC"); err == nil {
		t.Fatal("a message without ts has no stable store id and must be rejected")
	}
}

func TestSelectSyncChannels(t *testing.T) {
	channels := map[string]localChannel{
		"C1": {ID: "C1", Name: "general"},
		"C2": {ID: "C2", Name: "random"},
		"C3": {ID: "C3", Name: "old", IsArchived: true},
	}
	t.Run("empty filter skips archived", func(t *testing.T) {
		got := selectSyncChannels(channels, "")
		if len(got) != 2 {
			t.Fatalf("got %d channels, want 2 (archived excluded)", len(got))
		}
	})
	t.Run("filter by id", func(t *testing.T) {
		got := selectSyncChannels(channels, "C1")
		if len(got) != 1 || got[0].ID != "C1" {
			t.Fatalf("got %+v, want only C1", got)
		}
	})
	t.Run("filter by #name", func(t *testing.T) {
		got := selectSyncChannels(channels, "#random")
		if len(got) != 1 || got[0].ID != "C2" {
			t.Fatalf("got %+v, want only C2", got)
		}
	})
	t.Run("explicit filter includes archived", func(t *testing.T) {
		got := selectSyncChannels(channels, "old")
		if len(got) != 1 || got[0].ID != "C3" {
			t.Fatalf("explicitly naming an archived channel should select it; got %+v", got)
		}
	})
	t.Run("stable order", func(t *testing.T) {
		a := selectSyncChannels(channels, "")
		b := selectSyncChannels(channels, "")
		for i := range a {
			if a[i].ID != b[i].ID {
				t.Fatal("channel ordering must be deterministic across runs")
			}
		}
	})
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestSlackEnvelopeError(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"ok true", `{"ok":true,"channels":[]}`, false},
		{"ok false with code", `{"ok":false,"error":"not_authed"}`, true},
		{"ok false no code", `{"ok":false}`, true},
		{"no ok field", `{"channels":[]}`, false},
		{"dry-run sentinel", `{"dry_run":true}`, false},
		{"array payload", `[{"id":"C1"}]`, false},
		{"non-boolean ok", `{"ok":"yes"}`, false},
		{"not json", `garbage`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := slackEnvelopeError(json.RawMessage(tc.body))
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s", tc.body)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.body, err)
			}
		})
	}
}

// The exact payload that corrupted the mirror during live testing.
func TestSlackEnvelopeErrorCatchesNotAuthed(t *testing.T) {
	if err := slackEnvelopeError(json.RawMessage(`{"ok":false,"error":"not_authed"}`)); err == nil {
		t.Fatal("not_authed envelope must fail the resource, never be stored as a record")
	}
}

func TestSlackEnvelopeErrorCodeAndClassification(t *testing.T) {
	cases := []struct {
		body       string
		wantCode   string
		wantAccess bool
	}{
		{`{"ok":false,"error":"not_allowed_token_type"}`, "not_allowed_token_type", true},
		{`{"ok":false,"error":"missing_scope"}`, "missing_scope", true},
		{`{"ok":false,"error":"not_in_channel"}`, "not_in_channel", true},
		{`{"ok":false,"error":"not_authed"}`, "not_authed", false},
		{`{"ok":false,"error":"invalid_auth"}`, "invalid_auth", false},
		{`{"ok":false,"error":"token_revoked"}`, "token_revoked", false},
		{`{"ok":true}`, "", false},
		{`{"channels":[]}`, "", false},
	}
	for _, c := range cases {
		gotCode := slackEnvelopeErrorCode(json.RawMessage(c.body))
		if gotCode != c.wantCode {
			t.Fatalf("code for %s = %q, want %q", c.body, gotCode, c.wantCode)
		}
		if slackAccessDeniedCodes[gotCode] != c.wantAccess {
			t.Fatalf("access classification for %s = %v, want %v", c.body, slackAccessDeniedCodes[gotCode], c.wantAccess)
		}
	}
}

// A bad credential must never be softened into a per-resource warning: that
// would let a fully broken token report a clean sync.
func TestBadCredentialIsNotDowngradedToWarning(t *testing.T) {
	for _, code := range []string{"not_authed", "invalid_auth", "token_revoked", "account_inactive"} {
		if slackAccessDeniedCodes[code] {
			t.Fatalf("%q must stay a hard error, not an access warning", code)
		}
	}
}
