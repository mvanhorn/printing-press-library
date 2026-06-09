// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the cobratree-mirrored PII post-processor: real flat command JSON
// output is pseudonymized at the MCP boundary unless include_pii.
package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// withTempStore points dbPath()'s $HOME at a temp dir holding a fresh store so
// saltFromStore resolves against an isolated DB (never the operator's real one).
func withTempStore(t *testing.T) {
	t.Helper()
	resetSaltCacheForTest()
	t.Cleanup(resetSaltCacheForTest)
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Create the store at the canonical path used by search/sql handlers.
	p := filepath.Join(home, ".local", "share", "dice-fm-pp-cli", "data.db")
	s, err := store.Open(p)
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	s.Close()
}

const (
	doorListJSON     = `[{"ticket_id":"tk1","code":"ABC123","holder_name":"Dee Door","holder_email":"door1@example.com","claimed":true,"transferred":false}]`
	fansOptinJSON    = `[{"first_name":"Olive","last_name":"Optin","email":"fan@example.com","phone":"5550100","city":"London","country":"GB"}]`
	fansTopJSON      = `[{"email":"fan@example.com","name":"Olive Optin","total_spend":42.5,"orders_count":2}]`
	fansRepeatJSON   = `[{"email":"fan@example.com","name":"Olive Optin","events_count":2,"total_spend":42.5,"event_ids":["evt-1","evt-2"]}]`
	fansSegmentJSON  = `[{"email":"fan@example.com","name":"Olive Optin","events_count":2,"total_spend":42.5,"opted_in":true}]`
	fansProfileJSON  = `{"found":true,"email":"fan@example.com","name":"Olive Optin","opted_in":true,"orders_count":2,"total_spend":42.5,"events_purchased":["Show"],"ticket_types":["GA"]}`
	fansOptinCSVBody = "first_name,last_name,email,phone,city,country\nOlive,Optin,fan@example.com,5550100,London,GB\n"
)

func TestMirroredPIIPostProcessorRedactsRealFlatRowsByDefault(t *testing.T) {
	cases := []struct {
		name string
		in   string
		pii  []string
	}{
		{name: "door list", in: doorListJSON, pii: []string{"Dee Door", "door1@example.com"}},
		{name: "fans optin", in: fansOptinJSON, pii: []string{"Olive", "Optin", "fan@example.com", "5550100"}},
		{name: "fans top", in: fansTopJSON, pii: []string{"Olive Optin", "fan@example.com"}},
		{name: "fans repeat", in: fansRepeatJSON, pii: []string{"Olive Optin", "fan@example.com"}},
		{name: "fans segment", in: fansSegmentJSON, pii: []string{"Olive Optin", "fan@example.com"}},
		{name: "fans profile", in: fansProfileJSON, pii: []string{"Olive Optin", "fan@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempStore(t)
			out, err := mirroredPIIPostProcessor(tc.in, map[string]any{})
			if err != nil {
				t.Fatalf("post-processor: %v", err)
			}
			assertNoRawPII(t, out, tc.pii...)
			if !strings.Contains(out, "fan_ref") {
				t.Errorf("mirrored output missing fan_ref: %s", out)
			}
			if !json.Valid([]byte(out)) {
				t.Errorf("mirrored output is not JSON: %q", out)
			}
		})
	}
}

func TestMirroredPIIPostProcessorIncludePIIReturnsRawExceptDOB(t *testing.T) {
	withTempStore(t)
	in := `[{"email":"fan@example.com","name":"Olive Optin","dob":"1990-01-01","orders_count":2}]`
	out, err := mirroredPIIPostProcessor(in, map[string]any{"include_pii": true})
	if err != nil {
		t.Fatalf("post-processor: %v", err)
	}
	for _, raw := range []string{"fan@example.com", "Olive Optin"} {
		if !strings.Contains(out, raw) {
			t.Errorf("include_pii dropped raw value %q: %s", raw, out)
		}
	}
	if strings.Contains(out, "1990-01-01") || strings.Contains(out, "dob") {
		t.Errorf("include_pii must still omit dob: %s", out)
	}
	if !strings.Contains(out, "fan_ref") {
		t.Errorf("include_pii must still include fan_ref: %s", out)
	}
}

func TestMirroredPIIPostProcessorSamePersonSameTokenAcrossRows(t *testing.T) {
	withTempStore(t)
	in := `[{"email":"same@example.com","name":"Same One","total_spend":10},{"email":"same@example.com","name":"Same One","total_spend":20}]`
	out, err := mirroredPIIPostProcessor(in, map[string]any{})
	if err != nil {
		t.Fatalf("post-processor: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal scrubbed output: %v\n%s", err, out)
	}
	if len(rows) != 2 || rows[0]["fan_ref"] == "" || rows[0]["fan_ref"] != rows[1]["fan_ref"] {
		t.Fatalf("same person did not get same token: %+v", rows)
	}
}

func TestMirroredPIIPostProcessorFansOptinCSVArgScrubsJSON(t *testing.T) {
	withTempStore(t)
	out, err := mirroredPIIPostProcessor(fansOptinJSON, map[string]any{"csv": true})
	if err != nil {
		t.Fatalf("post-processor: %v", err)
	}
	assertNoRawPII(t, out, "Olive", "Optin", "fan@example.com", "5550100")
	if !strings.Contains(out, "fan_ref") {
		t.Fatalf("scrubbed CSV-arg output missing fan_ref: %s", out)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("scrubbed CSV-arg output is not JSON: %q", out)
	}
}

func TestMirroredPIIPostProcessorFailsClosedOnNonJSON(t *testing.T) {
	withTempStore(t)
	out, err := mirroredPIIPostProcessor(fansOptinCSVBody, map[string]any{})
	if err == nil {
		t.Fatalf("post-processor accepted non-JSON PII output: %q", out)
	}
}

func assertNoRawPII(t *testing.T, out string, pii ...string) {
	t.Helper()
	for _, p := range pii {
		if strings.Contains(out, p) {
			t.Errorf("output still contains raw PII %q: %s", p, out)
		}
	}
}
