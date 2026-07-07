// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for the MCP pseudonymization layer (review finding #4): a salted
// HMAC token replaces raw fan PII (email/phone/name) in MCP output by default;
// include_pii returns raw + token except dob, which is always stripped.
package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var tokenRE = regexp.MustCompile(`^fan:[0-9a-f]{16}$`)

func TestTokenDeterministicAndFormat(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	a := Token(salt, "fan-123")
	b := Token(salt, "fan-123")
	if a != b {
		t.Errorf("Token not deterministic: %q != %q", a, b)
	}
	if !tokenRE.MatchString(a) {
		t.Errorf("Token %q does not match fan:<16hex>", a)
	}
	if Token(salt, "fan-999") == a {
		t.Errorf("different identity keys produced the same token")
	}
}

func TestTokenSaltIsolation(t *testing.T) {
	a := Token([]byte("saltAsaltAsaltAsaltAsaltAsaltA123"), "fan-123")
	b := Token([]byte("saltBsaltBsaltBsaltBsaltBsaltB123"), "fan-123")
	if a == b {
		t.Errorf("same key under different salts produced the same token (salt not mixed in)")
	}
}

func TestScrubDefaultRedacts(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	row := map[string]any{
		"id":            "fan-123",
		"email":         "buyer@example.com",
		"phoneNumber":   "5550100",
		"firstName":     "Alice",
		"lastName":      "Smith",
		"dob":           "1990-01-01",
		"optInPartners": true,
		"total":         2500,
	}
	spec := FieldSpec{
		IdentityKey: "id",
		EmailKeys:   []string{"email"},
		PhoneKeys:   []string{"phoneNumber"},
		NameKeys:    []string{"firstName", "lastName"},
	}
	out := Scrub(row, spec, Opts{Salt: salt})

	// Identifiers removed.
	for _, k := range []string{"email", "phoneNumber", "firstName", "lastName", "dob"} {
		if _, ok := out[k]; ok {
			t.Errorf("default Scrub left PII key %q: %v", k, out[k])
		}
	}
	// Token added and stable with Token().
	ref, ok := out["fan_ref"].(string)
	if !ok || !tokenRE.MatchString(ref) {
		t.Errorf("fan_ref missing/malformed: %v", out["fan_ref"])
	}
	if ref != Token(salt, "fan-123") {
		t.Errorf("fan_ref %q != Token(salt, id)", ref)
	}
	// Non-PII preserved.
	if out["total"] != 2500 || out["optInPartners"] != true || out["id"] != "fan-123" {
		t.Errorf("non-PII fields altered: %+v", out)
	}
}

func TestScrubIncludePIIKeepsRawPlusToken(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	row := map[string]any{
		"id":    "fan-7",
		"email": "vip@example.com",
		"dob":   "1985-05-05",
	}
	spec := FieldSpec{IdentityKey: "id", EmailKeys: []string{"email"}}
	out := Scrub(row, spec, Opts{Salt: salt, IncludePII: true})

	if out["email"] != "vip@example.com" {
		t.Errorf("include_pii dropped raw email: %v", out["email"])
	}
	if _, ok := out["dob"]; ok {
		t.Errorf("include_pii must still drop dob: %v", out["dob"])
	}
	if ref, _ := out["fan_ref"].(string); ref != Token(salt, "fan-7") {
		t.Errorf("include_pii must still add the linking token, got %v", out["fan_ref"])
	}
}

func TestScrubSameIdentitySameToken(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	specA := FieldSpec{IdentityKey: "id", EmailKeys: []string{"email"}}
	specB := FieldSpec{IdentityKey: "fanId", EmailKeys: []string{"email"}}
	a := Scrub(map[string]any{"id": "p1", "email": "x@example.com"}, specA, Opts{Salt: salt})
	b := Scrub(map[string]any{"fanId": "p1", "email": "x@example.com"}, specB, Opts{Salt: salt})
	if a["fan_ref"] != b["fan_ref"] {
		t.Errorf("same identity across tools produced different tokens: %v vs %v", a["fan_ref"], b["fan_ref"])
	}
}

func TestScrubNestedContainers(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	// A ticket row with a nested holder object carrying PII.
	row := map[string]any{
		"id":   "tk-1",
		"code": "ABC123",
		"holder": map[string]any{
			"id":        "h-1",
			"email":     "holder@example.com",
			"firstName": "Bob",
		},
	}
	spec := FieldSpec{
		NestedContainers: []NestedSpec{
			{Key: "holder", Spec: FieldSpec{IdentityKey: "email", EmailKeys: []string{"email"}, NameKeys: []string{"firstName"}}},
		},
	}
	out := Scrub(row, spec, Opts{Salt: salt})
	holder, ok := out["holder"].(map[string]any)
	if !ok {
		t.Fatalf("holder not a map after scrub: %T", out["holder"])
	}
	if _, ok := holder["email"]; ok {
		t.Errorf("nested holder email not scrubbed: %v", holder)
	}
	if _, ok := holder["firstName"]; ok {
		t.Errorf("nested holder firstName not scrubbed: %v", holder)
	}
	if ref, _ := holder["fan_ref"].(string); ref != Token(salt, "holder@example.com") {
		t.Errorf("nested holder fan_ref wrong: %v", holder["fan_ref"])
	}
	// Non-PII top-level preserved.
	if out["code"] != "ABC123" {
		t.Errorf("non-PII top-level field altered: %v", out["code"])
	}
}

func TestScrubBlobRecursive(t *testing.T) {
	// ScrubJSONBlob covers the sql/search path: scrub nested containers and
	// flat PII rows without altering non-PII event/venue names.
	salt := []byte("0123456789abcdef0123456789abcdef")
	blob := map[string]any{
		"event": map[string]any{"name": "Show"},
		"fan": map[string]any{
			"id":    "f-9",
			"email": "deep@example.com",
			"dob":   "2000-02-02",
		},
		"top_fans": []any{
			map[string]any{"email": "flat@example.com", "name": "Flat Fan", "total_spend": 50},
		},
	}
	out := ScrubJSONBlob(blob, Opts{Salt: salt}).(map[string]any)
	fan := out["fan"].(map[string]any)
	for _, k := range []string{"email", "dob"} {
		if _, ok := fan[k]; ok {
			t.Errorf("ScrubJSONBlob left PII key %q in nested fan: %v", k, fan)
		}
	}
	if !strings.HasPrefix(fan["fan_ref"].(string), "fan:") {
		t.Errorf("ScrubJSONBlob did not add fan_ref to nested fan: %v", fan)
	}
	// Non-PII untouched.
	if out["event"].(map[string]any)["name"] != "Show" {
		t.Errorf("ScrubJSONBlob altered non-PII: %v", out["event"])
	}
	flat := out["top_fans"].([]any)[0].(map[string]any)
	for _, k := range []string{"email", "name"} {
		if _, ok := flat[k]; ok {
			t.Errorf("ScrubJSONBlob left flat PII key %q: %v", k, flat)
		}
	}
	if flat["fan_ref"] != Token(salt, "flat@example.com") {
		t.Errorf("flat fan_ref wrong: %v", flat["fan_ref"])
	}
}

func TestScrubJSONBlobFanContainerAndFlatRowShareEmailSeed(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	nested := ScrubJSONBlob(map[string]any{
		"fan": map[string]any{
			"id":        "f1",
			"email":     "p@example.com",
			"firstName": "P",
		},
	}, Opts{Salt: salt}).(map[string]any)
	flat := ScrubJSONBlob(map[string]any{
		"email": "p@example.com",
		"name":  "P",
	}, Opts{Salt: salt}).(map[string]any)

	fan := nested["fan"].(map[string]any)
	want := Token(salt, "p@example.com")
	if fan["fan_ref"] != want {
		t.Fatalf("nested fan_ref = %v, want email-seeded %v", fan["fan_ref"], want)
	}
	if flat["fan_ref"] != want {
		t.Fatalf("flat fan_ref = %v, want email-seeded %v", flat["fan_ref"], want)
	}
	if fan["fan_ref"] != flat["fan_ref"] {
		t.Fatalf("same person produced different nested and flat fan_ref values: %v vs %v", fan["fan_ref"], flat["fan_ref"])
	}
}

func TestScrubJSONBlobFanContainerKeepsSingleEmailSeededToken(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	out := ScrubJSONBlob(map[string]any{
		"fan": map[string]any{
			"id":        "f1",
			"email":     "p@example.com",
			"name":      "P Example",
			"firstName": "P",
			"phone":     "5550100",
		},
	}, Opts{Salt: salt}).(map[string]any)

	fan := out["fan"].(map[string]any)
	if fan["fan_ref"] != Token(salt, "p@example.com") {
		t.Fatalf("fan_ref = %v, want email-seeded token", fan["fan_ref"])
	}
	for _, k := range []string{"email", "name", "firstName", "phone"} {
		if _, ok := fan[k]; ok {
			t.Fatalf("fan container left direct identifier %q: %+v", k, fan)
		}
	}
}

func TestScrubJSONBlobFanContainerUsesIDWhenEmailMissing(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	out := ScrubJSONBlob(map[string]any{
		"fan": map[string]any{
			"id":        "f1",
			"firstName": "P",
			"phone":     "5550100",
		},
	}, Opts{Salt: salt}).(map[string]any)

	fan := out["fan"].(map[string]any)
	if fan["fan_ref"] != Token(salt, "f1") {
		t.Fatalf("fan_ref = %v, want id-seeded token", fan["fan_ref"])
	}
	for _, k := range []string{"firstName", "phone"} {
		if _, ok := fan[k]; ok {
			t.Fatalf("fan container left direct identifier %q: %+v", k, fan)
		}
	}
}

func TestScrubJSONBlobIncludePIIDropsDOB(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	blob := map[string]any{
		"email": "raw@example.com",
		"name":  "Raw Fan",
		"dob":   "1999-09-09",
	}
	out := ScrubJSONBlob(blob, Opts{Salt: salt, IncludePII: true}).(map[string]any)
	if out["email"] != "raw@example.com" || out["name"] != "Raw Fan" {
		t.Fatalf("include_pii dropped raw non-dob identifiers: %+v", out)
	}
	if _, ok := out["dob"]; ok {
		t.Fatalf("include_pii left dob: %+v", out)
	}
	if out["fan_ref"] != Token(salt, "raw@example.com") {
		t.Fatalf("include_pii missing stable fan_ref: %+v", out)
	}
}

func TestScrubJSONBlobRedactsHolderNameWithoutEmail(t *testing.T) {
	salt := []byte("0123456789abcdef0123456789abcdef")
	blob := map[string]any{
		"holder_name":  "Name Only",
		"holder_email": "",
		"dob":          "1990-01-01",
		"claimed":      true,
	}
	out := ScrubJSONBlob(blob, Opts{Salt: salt}).(map[string]any)
	for _, k := range []string{"holder_name", "holder_email", "dob"} {
		if _, ok := out[k]; ok {
			t.Fatalf("ScrubJSONBlob left seedless holder PII key %q: %+v", k, out)
		}
	}
	if out["claimed"] != true {
		t.Fatalf("ScrubJSONBlob altered non-PII field: %+v", out)
	}
	if out["fan_ref"] != Token(salt, "name only") {
		t.Fatalf("ScrubJSONBlob missing name-derived fan_ref: %+v", out)
	}
}

func TestReadOrCreateSaltRejectsWrongLengthSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), saltSidecarName)
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatalf("write short sidecar: %v", err)
	}

	salt, err := readOrCreateSalt(path)
	if err == nil {
		t.Fatalf("readOrCreateSalt returned nil error and salt %x for wrong-length sidecar", salt)
	}
	if !strings.Contains(err.Error(), "wrong length 5 (expected 32)") {
		t.Fatalf("wrong-length error did not explain refusal: %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read sidecar after error: %v", readErr)
	}
	if string(got) != "short" {
		t.Fatalf("wrong-length sidecar was overwritten: %q", got)
	}
}

func TestReadOrCreateSaltCreatesMissingSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), saltSidecarName)

	salt, err := readOrCreateSalt(path)
	if err != nil {
		t.Fatalf("readOrCreateSalt missing sidecar: %v", err)
	}
	if len(salt) != saltLen {
		t.Fatalf("created salt length = %d, want %d", len(salt), saltLen)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat created sidecar: %v", err)
	}
	if info.Size() != saltLen {
		t.Fatalf("created sidecar size = %d, want %d", info.Size(), saltLen)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("created sidecar mode = %o, want 0600", got)
	}
}
