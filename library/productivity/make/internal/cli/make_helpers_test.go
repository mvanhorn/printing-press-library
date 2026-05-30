// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReasonFingerprint(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"replaces numbers", "Connection 12345 expired", "Connection N expired"},
		{"replaces uuids", "request abc12345-1234-1234-1234-1234567890ab failed", "request <uuid> failed"},
		{"trims whitespace", "  rate limit hit  ", "rate limit hit"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reasonFingerprint(tc.in)
			if got != tc.want {
				t.Errorf("reasonFingerprint(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompileReasonMatcher(t *testing.T) {
	if _, err := compileReasonMatcher(""); err == nil {
		t.Errorf("compileReasonMatcher(\"\") should error: empty pattern is dangerous")
	}
	re, err := compileReasonMatcher("rate.*limit")
	if err != nil {
		t.Fatalf("compileReasonMatcher(rate.*limit): %v", err)
	}
	if !re.MatchString("Rate Limit exceeded") {
		t.Errorf("matcher should be case-insensitive by default")
	}
	if re.MatchString("permission denied") {
		t.Errorf("matcher should not match unrelated reasons")
	}
}

func TestWalkBlueprintWebhookRefs(t *testing.T) {
	bp := []byte(`{
		"blueprint": {
			"flow": [
				{"id": 1, "module": "gateway:CustomWebHook", "parameters": {"hook": 12345}},
				{"id": 2, "module": "smartsuite:create"},
				{"id": 3, "module": "gateway:CustomMailHook", "parameters": {"hook": 67890}},
				{"id": 4, "module": "router", "routes": [
					{"flow": [{"id": 5, "module": "gateway:CustomWebHook", "parameters": {"hook": 99999}}]}
				]}
			]
		}
	}`)
	ids := walkBlueprintWebhookRefs(bp)
	if len(ids) != 3 {
		t.Fatalf("walkBlueprintWebhookRefs = %v, want 3 IDs", ids)
	}
	want := map[int64]bool{12345: true, 67890: true, 99999: true}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected webhook id %d", id)
		}
	}
}

func TestWalkBlueprintConnectionRefs(t *testing.T) {
	bp := []byte(`{
		"blueprint": {
			"flow": [
				{"id": 1, "module": "google-mail:listMessages", "parameters": {"__IMTCONN__": 555}},
				{"id": 2, "module": "google-mail:sendMessage", "parameters": {"__IMTCONN__": 555}},
				{"id": 3, "module": "smartsuite:create", "parameters": {"__IMTCONN__": 999}}
			]
		}
	}`)
	ids := walkBlueprintConnectionRefs(bp)
	if len(ids) != 2 {
		t.Fatalf("walkBlueprintConnectionRefs = %v, want 2 unique IDs", ids)
	}
}

func TestCanonicalBlueprintJSONStripsMetadata(t *testing.T) {
	bp := []byte(`{
		"blueprint": {
			"flow": [
				{
					"id": 1,
					"module": "gateway:CustomWebHook",
					"parameters": {"hook": 42},
					"metadata": {
						"expect": [{"name": "hook", "type": "hook:gateway-webhook"}],
						"restore": {"parameters": {"hook": {"label": "Some webhook"}}},
						"designer": {"x": 100, "y": 200}
					}
				}
			]
		}
	}`)
	cleaned, err := canonicalBlueprintJSON(bp, false)
	if err != nil {
		t.Fatalf("canonicalBlueprintJSON: %v", err)
	}
	if strings.Contains(string(cleaned), "expect") {
		t.Errorf("expected metadata.expect to be stripped from canonical output")
	}
	if strings.Contains(string(cleaned), "designer") {
		t.Errorf("expected metadata.designer to be stripped from canonical output")
	}
	if !strings.Contains(string(cleaned), "\"hook\": 42") {
		t.Errorf("expected parameters.hook to survive canonicalization")
	}
	kept, _ := canonicalBlueprintJSON(bp, true)
	if !strings.Contains(string(kept), "expect") {
		t.Errorf("--keep-metadata should preserve metadata.expect")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Old Town Design Build - Update Tags", "old-town-design-build-update-tags"},
		{"  Buzzsprout App Stats (from Task Magic to SS v2)  ", "buzzsprout-app-stats-from-task-magic-to-ss-v2"},
		{"@@@", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := slugify(tc.in)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestApplyRemap(t *testing.T) {
	bp := []byte(`{
		"blueprint": {
			"flow": [
				{"id": 1, "module": "gateway:CustomWebHook", "parameters": {"hook": 111}},
				{"id": 2, "module": "google-mail:listMessages", "parameters": {"__IMTCONN__": 555}}
			]
		}
	}`)
	rm := remapManifest{
		Hooks:       map[int64]int64{111: 222},
		Connections: map[int64]int64{555: 999},
	}
	out, summary := applyRemap(bp, rm)
	if summary["hooks"] != 1 {
		t.Errorf("expected 1 hook rewrite, got %d", summary["hooks"])
	}
	if summary["connections"] != 1 {
		t.Errorf("expected 1 connection rewrite, got %d", summary["connections"])
	}
	var top map[string]any
	if err := json.Unmarshal(out, &top); err != nil {
		t.Fatalf("rewritten JSON must parse: %v", err)
	}
	root := top["blueprint"].(map[string]any)
	flow := root["flow"].([]any)
	hookParams := flow[0].(map[string]any)["parameters"].(map[string]any)
	if int64(hookParams["hook"].(float64)) != 222 {
		t.Errorf("hookId not rewritten: %v", hookParams["hook"])
	}
}
