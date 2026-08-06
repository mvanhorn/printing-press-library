// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for novel feature helpers and store-backed commands.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
)

func TestHasTag(t *testing.T) {
	c := map[string]any{"tags": []any{"vip", "premium"}}
	cases := []struct {
		tag string
		ok  bool
	}{
		{"vip", true},
		{"premium", true},
		{"unpaid", false},
	}
	for _, tc := range cases {
		if got := hasTag(c, tc.tag); got != tc.ok {
			t.Errorf("hasTag(%q) = %v, want %v", tc.tag, got, tc.ok)
		}
	}
	if hasTag(map[string]any{}, "vip") {
		t.Errorf("hasTag on contact without tags should be false")
	}
}

func TestCustomFieldValue(t *testing.T) {
	c := map[string]any{
		"custom_fields": []any{
			map[string]any{"name": "orderId", "value": "A-100"},
			map[string]any{"name": "region", "value": "us-east"},
		},
	}
	v, ok := customFieldValue(c, "orderId")
	if !ok || v != "A-100" {
		t.Errorf("customFieldValue(orderId) = (%v, %v), want (A-100, true)", v, ok)
	}
	if _, ok := customFieldValue(c, "missing"); ok {
		t.Errorf("customFieldValue(missing) should be not-found")
	}
}

func TestAgentName(t *testing.T) {
	if got := agentName(map[string]any{"email": "a@x.com", "firstName": "A", "lastName": "B"}); got != "a@x.com" {
		t.Errorf("agentName prefer email = %q", got)
	}
	if got := agentName(map[string]any{"firstName": "A", "lastName": "B"}); got != "A B" {
		t.Errorf("agentName fallback = %q", got)
	}
}

func seedContacts(t *testing.T, raws ...string) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, len(raws))
	for _, r := range raws {
		var m map[string]any
		if err := json.Unmarshal([]byte(r), &m); err != nil {
			t.Fatalf("bad raw contact: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func seedContactStore(t *testing.T, contacts []map[string]any) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for _, c := range contacts {
		id := toStrID(c["id"])
		data, _ := json.Marshal(c)
		if err := db.Upsert("contact", id, data); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	db.Close()
	return dbPath
}

func toStrID(v any) string {
	switch t := v.(type) {
	case float64:
		return strconvI(int(t))
	case int:
		return strconvI(t)
	case string:
		return t
	}
	return "0"
}

func strconvI(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestNovelContactByTagBehavior(t *testing.T) {
	contacts := seedContacts(t,
		`{"id":1,"firstName":"Alice","email":"alice@x.com","tags":["vip"]}`,
		`{"id":2,"firstName":"Bob","email":"bob@x.com","tags":["premium"]}`,
		`{"id":3,"firstName":"Eve","email":"eve@x.com","tags":["vip","premium"]}`,
	)
	dbPath := seedContactStore(t, contacts)
	cmd := RootCmd()
	cmd.SetArgs([]string{"contact", "by-tag", "vip", "--db", dbPath, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("by-tag error: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not JSON: %v out=%s", err, out.String())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 vip contacts, got %d out=%s", len(got), out.String())
	}
}
