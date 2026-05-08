package cli

import (
	"strings"
	"testing"
)

func TestRenderInstallSnippet(t *testing.T) {
	got, language, err := renderInstallSnippet("abc123", "html")
	if err != nil {
		t.Fatalf("renderInstallSnippet returned error: %v", err)
	}
	if language != "html" {
		t.Fatalf("language = %q, want html", language)
	}
	if !strings.Contains(got, "https://www.clarity.ms/tag/") {
		t.Fatalf("snippet missing Clarity tag URL: %s", got)
	}
	if !strings.Contains(got, `"abc123"`) {
		t.Fatalf("snippet missing quoted project ID: %s", got)
	}
}

func TestAuditHTML(t *testing.T) {
	html := `<html><head>
<script src="https://www.clarity.ms/tag/abc123"></script>
<script>
window.clarity("identify", "user-1");
window.clarity("set", "experiment", "a");
window.clarity("event", "signup");
</script>
</head><body data-clarity-mask="true"><article data-clarity-unmask="true"></article></body></html>`

	got := auditHTML("index.html", html)
	if !got.HasInstall {
		t.Fatal("HasInstall = false, want true")
	}
	if got.FoundProjectID != "abc123" {
		t.Fatalf("FoundProjectID = %q, want abc123", got.FoundProjectID)
	}
	if got.Calls["identify"] != 1 || got.Calls["set"] != 1 || got.Calls["event"] != 1 {
		t.Fatalf("Calls = %#v, want identify/set/event counts", got.Calls)
	}
	if got.MaskCount != 1 || got.UnmaskCount != 1 {
		t.Fatalf("mask counts = %d/%d, want 1/1", got.MaskCount, got.UnmaskCount)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("Warnings empty, want mixed mask/unmask warning")
	}
}

func TestBuildInsightsQuery(t *testing.T) {
	got, err := buildInsightsQuery(2, []string{"os", "Country"})
	if err != nil {
		t.Fatalf("buildInsightsQuery returned error: %v", err)
	}
	if got.Get("numOfDays") != "2" {
		t.Fatalf("numOfDays = %q, want 2", got.Get("numOfDays"))
	}
	if got.Get("dimension1") != "OS" {
		t.Fatalf("dimension1 = %q, want OS", got.Get("dimension1"))
	}
	if got.Get("dimension2") != "Country/Region" {
		t.Fatalf("dimension2 = %q, want Country/Region", got.Get("dimension2"))
	}
}
