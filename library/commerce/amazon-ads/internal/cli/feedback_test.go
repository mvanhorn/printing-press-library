package cli

import (
	"strings"
	"testing"
	"time"
)

func TestFeedbackRedactionAndSummaryEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	text := redactFeedbackText(`campaign "BestSelf Launch" had ASIN B0ABCDEF12 in /tmp/private report.csv`)
	if strings.Contains(text, "B0ABCDEF12") || strings.Contains(text, "BestSelf Launch") || strings.Contains(text, "report.csv") {
		t.Fatalf("text was not redacted: %s", text)
	}
	entry := FeedbackEntry{
		Text:       text,
		CLI:        "amazon-ads-pp-cli",
		Command:    "reports recipe",
		Version:    version,
		Persona:    sellerNoDSPPersona,
		ExitStatus: 2,
		Platform:   "test/test",
		Timestamp:  time.Now().UTC(),
	}
	if err := appendFeedback(entry); err != nil {
		t.Fatal(err)
	}
	entries, err := readFeedbackEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Command != "reports recipe" || entries[0].Persona != sellerNoDSPPersona {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestRedactFeedbackContextRecurses(t *testing.T) {
	t.Parallel()
	got := redactFeedbackContext(map[string]any{
		"safe": "campaign Launch Plan used reports/week.csv",
		"nested": map[string]any{
			"profile_id": "123",
			"items": []any{
				map[string]any{"asin": "B012345678", "note": "brand ExampleCo"},
				"account Primary",
			},
		},
	})
	if got["safe"] != "campaign [REDACTED_NAME]" {
		t.Fatalf("safe text redaction = %#v", got["safe"])
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested context = %#v", got["nested"])
	}
	if nested["profile_id"] != "[REDACTED]" {
		t.Fatalf("nested profile_id = %#v", nested["profile_id"])
	}
	items, ok := nested["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("nested items = %#v", nested["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item = %#v", items[0])
	}
	if first["asin"] != "[REDACTED]" || first["note"] != "brand [REDACTED_NAME]" {
		t.Fatalf("first item redaction = %#v", first)
	}
	if items[1] != "account [REDACTED_NAME]" {
		t.Fatalf("array string redaction = %#v", items[1])
	}
}
