package transcripts

import (
	"strings"
	"testing"
)

func TestPickTrackExact(t *testing.T) {
	tracks := []captionTrack{{LanguageCode: "es", BaseURL: "u-es"}, {LanguageCode: "en", BaseURL: "u-en"}}
	got := pickTrack(tracks, "en")
	if got.BaseURL != "u-en" {
		t.Fatalf("want u-en, got %q", got.BaseURL)
	}
}

func TestPickTrackPrefix(t *testing.T) {
	tracks := []captionTrack{{LanguageCode: "es-419", BaseURL: "u-419"}, {LanguageCode: "fr", BaseURL: "u-fr"}}
	got := pickTrack(tracks, "es")
	if got.BaseURL != "u-419" {
		t.Fatalf("want u-419 (prefix match), got %q", got.BaseURL)
	}
}

func TestPickTrackFallback(t *testing.T) {
	tracks := []captionTrack{{LanguageCode: "ja", BaseURL: "u-ja"}}
	got := pickTrack(tracks, "en")
	if got.BaseURL != "u-ja" {
		t.Fatalf("expected fallback to first track, got %q", got.BaseURL)
	}
}

func TestPlainText(t *testing.T) {
	tr := &Transcript{Segments: []Segment{{Text: "hola"}, {Text: "mundo"}}}
	if got := tr.PlainText(); got != "hola mundo" {
		t.Fatalf("plain text mismatch: %q", got)
	}
}

func TestHTMLEntityReplacer(t *testing.T) {
	got := htmlEntity.Replace("dad &#39;s rules &amp; rights")
	if !strings.Contains(got, "dad 's rules & rights") {
		t.Fatalf("entity unescape failed: %q", got)
	}
}
