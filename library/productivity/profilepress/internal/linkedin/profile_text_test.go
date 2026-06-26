package linkedin

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/browser"
)

func TestParseProfilePageTextExtractsSections(t *testing.T) {
	page := browser.PageText{
		URL:   "https://www.linkedin.com/in/michael-marcusa-90259176/",
		Title: "Michael Marcusa | LinkedIn",
		Text: strings.Join([]string{
			"Home",
			"Michael Marcusa",
			"Principal Researcher (UX) @ Meta | Human-Centered AI Evaluation & Alignment",
			"Washington DC-Baltimore Area",
			"Contact info",
			"About",
			"I design evaluation systems for AI products.",
			"Experience",
			"Meta",
			"Principal Researcher",
			"Education",
			"Brown University",
			"Skills",
			"AI Evaluation",
		}, "\n"),
	}
	sections, source, err := ParseProfilePageText(page, "Michael Marcusa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "browser-cdp:https://www.linkedin.com/in/michael-marcusa-90259176/") {
		t.Fatalf("bad source: %s", source)
	}
	got := map[string]string{}
	for _, section := range sections {
		got[section.Name] = section.RawText
	}
	if got["headline"] != "Principal Researcher (UX) @ Meta | Human-Centered AI Evaluation & Alignment" {
		t.Fatalf("bad headline: %#v", got["headline"])
	}
	if !strings.Contains(got["about"], "evaluation systems") {
		t.Fatalf("missing about: %#v", got["about"])
	}
	if !strings.Contains(got["experience"], "Meta") {
		t.Fatalf("missing experience: %#v", got["experience"])
	}
	if !strings.Contains(got["raw_text"], "Michael Marcusa") {
		t.Fatal("raw_text should preserve capture")
	}
}

func TestParseProfilePageTextRejectsWrongName(t *testing.T) {
	page := browser.PageText{URL: "https://www.linkedin.com/in/other/", Title: "Other Person | LinkedIn", Text: "Other Person\nAbout\nAI"}
	_, _, err := ParseProfilePageText(page, "Michael Marcusa")
	if err == nil || !strings.Contains(err.Error(), "expected profile name") {
		t.Fatalf("expected wrong-name error, got %v", err)
	}
}

func TestParseProfilePageTextRejectsNonLinkedIn(t *testing.T) {
	page := browser.PageText{URL: "https://example.com", Title: "Example", Text: "Michael Marcusa"}
	_, _, err := ParseProfilePageText(page, "Michael Marcusa")
	if err == nil || !strings.Contains(err.Error(), "not LinkedIn") {
		t.Fatalf("expected non-linkedin error, got %v", err)
	}
}

func TestParseProfilePageTextRequiresRecognizableSections(t *testing.T) {
	page := browser.PageText{URL: "https://www.linkedin.com/in/michael-marcusa-90259176/", Title: "Michael Marcusa | LinkedIn", Text: "Michael Marcusa\nOnly a name"}
	_, _, err := ParseProfilePageText(page, "Michael Marcusa")
	if err == nil || !strings.Contains(err.Error(), "recognizable profile sections") {
		t.Fatalf("expected thin-section error, got %v", err)
	}
}
func TestParseProfilePageTextRejectsLinkedInFeed(t *testing.T) {
	page := browser.PageText{URL: "https://www.linkedin.com/feed/", Title: "Feed | LinkedIn", Text: "Michael Marcusa\nAbout\nNot a profile"}
	_, _, err := ParseProfilePageText(page, "Michael Marcusa")
	if err == nil || !strings.Contains(err.Error(), "does not look like a LinkedIn profile") {
		t.Fatalf("expected non-profile LinkedIn error, got %v", err)
	}
}
