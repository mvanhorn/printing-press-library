// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package spotify

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// Fixture captured 2026-05-17 from a logged-in Spotify Premium session against
// `transcript-read-along/v2/episode/5PwtWcgg71nSkb63ZV4hGX` (Acquired's "10
// Years of Acquired (with Michael Lewis)"). First 12 sections; highlight arrays
// stripped to keep the fixture compact.
const fixtureJSON = `{
  "version": "1.0",
  "transcriptUri": "spotify:transcript:2moFbgSvzv0GcIwiFOBTxp",
  "publishedAt": "2026-05-17T19:33:06.742395014Z",
  "language": "en-us",
  "showName": "Acquired",
  "episodeName": "10 Years of Acquired (with Michael Lewis)",
  "shareable": true,
  "timeSyncedStatus": "SYLLABLE_SYNCED",
  "section": [
    {"startMs": 680,  "title": {"title": "Celebrating 10 Years in Google's Original Garage"}},
    {"startMs": 680,  "text":  {"sentence": {"startMs": 680,  "text": "Happy 10 years."}}},
    {"startMs": 1600, "title": {"title": "Speaker 2"}},
    {"startMs": 1600, "text":  {"sentence": {"startMs": 1600, "text": "Happy 10 year anniversary, Ben."}}},
    {"startMs": 3080, "text":  {"sentence": {"startMs": 3080, "text": "It's."}}},
    {"startMs": 3560, "title": {"title": "Speaker 1"}},
    {"startMs": 3560, "text":  {"sentence": {"startMs": 3560, "text": "Crazy it's been 10 years."}}},
    {"startMs": 4800, "title": {"title": "Speaker 2"}},
    {"startMs": 4800, "text":  {"sentence": {"startMs": 4800, "text": "I know here we are, brought you down here to Silicon Valley to record our 10 year anniversary holiday special here."}}},
    {"startMs": 12000, "text": {"sentence": {"startMs": 12000, "text": "I wanted a special place."}}}
  ]
}`

func TestExtractEpisodeID(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://open.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX", "5PwtWcgg71nSkb63ZV4hGX"},
		{"https://open.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX?si=abc", "5PwtWcgg71nSkb63ZV4hGX"},
		{"https://play.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX", "5PwtWcgg71nSkb63ZV4hGX"},
		{"https://open.spotify.com/embed/episode/5PwtWcgg71nSkb63ZV4hGX", "5PwtWcgg71nSkb63ZV4hGX"},
		{"https://open.spotify.com/show/3HdNbDtBuzPjOSWk5gqsHN", ""},
		{"https://www.dwarkesh.com/p/anything", ""},
	}
	for _, c := range cases {
		got := extractEpisodeID(c.url)
		if got != c.want {
			t.Errorf("extractEpisodeID(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestParseTranscriptJSON_Fixture(t *testing.T) {
	url := "https://open.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX"
	tr, err := parseTranscriptJSON([]byte(fixtureJSON), url, "5PwtWcgg71nSkb63ZV4hGX")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	// Top-level fields.
	if tr.Show != "acquired" {
		t.Errorf("Show = %q, want acquired", tr.Show)
	}
	if tr.Title != "10 Years of Acquired (with Michael Lewis)" {
		t.Errorf("Title = %q", tr.Title)
	}
	if tr.URL != url {
		t.Errorf("URL = %q", tr.URL)
	}
	if tr.Source != "spotify" || tr.Provider != "spotify-readalong" {
		t.Errorf("provider mismatch: source=%q provider=%q", tr.Source, tr.Provider)
	}
	if tr.Tier != transcript.TierCookie {
		t.Errorf("Tier = %q, want cookie", tr.Tier)
	}

	// Section header captured.
	if len(tr.SectionTimestamps) != 1 || !strings.Contains(tr.SectionTimestamps[0].Title, "Celebrating") {
		t.Errorf("section timestamp not captured: %+v", tr.SectionTimestamps)
	}

	// Segments: 4 in the fixture (one per Speaker turn, with the trailing
	// Speaker 2 paragraph merged into a single segment).
	if len(tr.Segments) != 4 {
		t.Fatalf("Segments len = %d, want 4: %+v", len(tr.Segments), tr.Segments)
	}

	// First segment is the "Happy 10 years." line attributed to the default
	// "Speaker 1" (no prior label, so the first sentence falls under the
	// initial speaker).
	if !strings.Contains(tr.Segments[0].Text, "Happy 10 years") {
		t.Errorf("segment[0].Text = %q", tr.Segments[0].Text)
	}

	// Second segment is Speaker 2: "Happy 10 year anniversary, Ben. It's."
	// (two sentences merged into one segment until the next speaker turn).
	s2 := tr.Segments[1]
	if s2.Speaker != "Speaker 2" {
		t.Errorf("segment[1].Speaker = %q, want Speaker 2", s2.Speaker)
	}
	if !strings.Contains(s2.Text, "Happy 10 year anniversary, Ben") || !strings.Contains(s2.Text, "It's.") {
		t.Errorf("segment[1].Text not merged correctly: %q", s2.Text)
	}

	// Third segment is Speaker 1: "Crazy it's been 10 years."
	if tr.Segments[2].Speaker != "Speaker 1" {
		t.Errorf("segment[2].Speaker = %q, want Speaker 1", tr.Segments[2].Speaker)
	}

	// Fourth segment is Speaker 2 with the merged "I know..." + "I wanted..." paragraph.
	s4 := tr.Segments[3]
	if s4.Speaker != "Speaker 2" {
		t.Errorf("segment[3].Speaker = %q, want Speaker 2", s4.Speaker)
	}
	if !strings.Contains(s4.Text, "I know here we are") || !strings.Contains(s4.Text, "I wanted a special place") {
		t.Errorf("segment[3].Text not merged correctly: %q", s4.Text)
	}
	if s4.TsSec != 4 {
		t.Errorf("segment[3].TsSec = %d, want 4 (4800ms / 1000)", s4.TsSec)
	}
}

func TestAdapterMatch(t *testing.T) {
	a := New()
	if !a.Match("https://open.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX") {
		t.Error("episode URL should match")
	}
	if a.Match("https://www.dwarkesh.com/p/karpathy") {
		t.Error("non-Spotify URL should not match")
	}
}
