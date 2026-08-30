// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED, not generated.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactSessionCredentialsHidesSidAndToken(t *testing.T) {
	in := json.RawMessage(`{"account":"smoochy","device_id":"dev-1","sid":"LIVE_SID","synotoken":"LIVE_TOKEN"}`)

	out := string(redactSessionCredentials(in))

	for _, secret := range []string{"LIVE_SID", "LIVE_TOKEN"} {
		if strings.Contains(out, secret) {
			t.Fatalf("credential %q survived redaction: %s", secret, out)
		}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("redacted payload is not valid JSON: %v", err)
	}
	if obj["sid"] != "***" || obj["synotoken"] != "***" {
		t.Fatalf("expected both credentials replaced with ***, got %s", out)
	}
	if obj["account"] != "smoochy" {
		t.Fatalf("account must survive redaction, got %s", out)
	}
	if obj["device_id"] != "dev-1" {
		t.Fatalf("device_id is needed for the next two-factor login and must survive, got %s", out)
	}
}

func TestRedactSessionCredentialsHidesCredentialsInsideDSMEnvelope(t *testing.T) {
	in := json.RawMessage(`{"data":{"account":"smoochy","sid":"LIVE_SID","synotoken":"LIVE_TOKEN"},"success":true}`)

	out := string(redactSessionCredentials(in))

	for _, secret := range []string{"LIVE_SID", "LIVE_TOKEN"} {
		if strings.Contains(out, secret) {
			t.Fatalf("credential %q survived redaction inside the envelope: %s", secret, out)
		}
	}
	if !strings.Contains(out, `"success":true`) || !strings.Contains(out, "smoochy") {
		t.Fatalf("envelope and non-credential fields must survive, got %s", out)
	}
}

func TestRedactSessionCredentialsLeavesOtherPayloadsAlone(t *testing.T) {
	in := json.RawMessage(`{"account":"smoochy","is_portal_port":false}`)

	if got := string(redactSessionCredentials(in)); got != string(in) {
		t.Fatalf("payload without credentials must be returned byte-identical, got %s", got)
	}
}

func TestRedactSessionCredentialsPassesNonObjectsThrough(t *testing.T) {
	for _, in := range []string{`[1,2,3]`, `"plain string"`, `not json at all`} {
		if got := string(redactSessionCredentials(json.RawMessage(in))); got != in {
			t.Fatalf("non-object input %q must pass through unchanged, got %q", in, got)
		}
	}
}
