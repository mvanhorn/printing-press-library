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
