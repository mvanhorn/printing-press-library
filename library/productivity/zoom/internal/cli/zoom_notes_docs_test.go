package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/docsbridge"
)

func sampleTranscript() *docsbridge.Transcript {
	return &docsbridge.Transcript{
		MeetingID:        "KNnF+0YXT26CAnB3p8f9gw==",
		MeetingStartTime: "1785166367943",
		Items: []docsbridge.TranscriptItem{
			{Text: "How was your weekend?", StartTime: "00:00:53.000", EndTime: "00:00:55.000", UserID: "1"},
			{Text: "Pretty good.", StartTime: "00:01:03.000", EndTime: "00:01:05.000", UserID: "2"},
			{Text: "TODO: file the retro issue", StartTime: "00:01:05.000", EndTime: "00:01:08.000", UserID: "2"},
		},
		Speakers: []docsbridge.TranscriptSpeaker{
			{UserID: "1", Username: "marius", SpeakerName: "Marius Florescu"},
			{UserID: "2", Username: "austin"},
		},
	}
}

func TestTranscriptToVTT(t *testing.T) {
	out := transcriptToVTT(sampleTranscript())
	for _, want := range []string{
		"WEBVTT",
		"00:00:53.000 --> 00:00:55.000",
		"Marius Florescu: How was your weekend?",
		// speakerName is empty for this speaker, so username is the fallback.
		"austin: Pretty good.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("VTT output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestVTTTimestampPadsFraction(t *testing.T) {
	cases := map[string]string{
		"00:00:53":        "00:00:53.000",
		"00:00:53.5":      "00:00:53.500",
		"00:00:53.123456": "00:00:53.123",
		"":                "00:00:00.000",
	}
	for in, want := range cases {
		if got := vttTimestamp(in); got != want {
			t.Errorf("vttTimestamp(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranscriptToMarkdownGroupsBySpeaker(t *testing.T) {
	out := transcriptToMarkdown(sampleTranscript(), "CB Platform Sync")
	if !strings.Contains(out, "# CB Platform Sync") {
		t.Errorf("markdown missing title heading\ngot:\n%s", out)
	}
	if !strings.Contains(out, "**Marius Florescu** (00:00:53.000)") {
		t.Errorf("markdown missing speaker heading with timestamp\ngot:\n%s", out)
	}
	// Consecutive utterances from one speaker share a single heading.
	if got := strings.Count(out, "**austin**"); got != 1 {
		t.Errorf("consecutive turns produced %d headings for one speaker, want 1", got)
	}
}

func TestTranscriptToIngestedNoteSegmentsAndTodos(t *testing.T) {
	note := docsbridge.Note{ID: "J8FjimDWSWS0t4qXexE3cQ", Title: "CB Platform Sync", MeetingID: "KNnF+0YXT26CAnB3p8f9gw=="}
	got := transcriptToIngestedNote(note, sampleTranscript())

	if got.SourceFile != "zoom-docs://J8FjimDWSWS0t4qXexE3cQ" {
		t.Errorf("SourceFile = %q", got.SourceFile)
	}
	if got.FileFormat != "zoom-docs-transcript" {
		t.Errorf("FileFormat = %q", got.FileFormat)
	}
	if got.MeetingTopic != "CB Platform Sync" || got.MeetingID != "KNnF+0YXT26CAnB3p8f9gw==" {
		t.Errorf("meeting fields = %q / %q", got.MeetingTopic, got.MeetingID)
	}
	if got.StartTime.IsZero() {
		t.Error("StartTime not parsed from meetingStartTime epoch millis")
	}

	// One segment per speaker turn, headed by the speaker.
	if len(got.Segments) != 2 {
		t.Fatalf("got %d segments, want 2 (one per speaker turn)", len(got.Segments))
	}
	if got.Segments[0].Heading != "Marius Florescu" || got.Segments[0].Text != "How was your weekend?" {
		t.Errorf("segment 0 = %+v", got.Segments[0])
	}
	if got.Segments[1].Heading != "austin" {
		t.Errorf("segment 1 heading = %q", got.Segments[1].Heading)
	}

	// The shared action-item extractor sees one line per turn.
	if len(got.Todos) != 1 {
		t.Fatalf("got %d todos, want 1", len(got.Todos))
	}
	if got.Todos[0].Text != "file the retro issue" {
		t.Errorf("todo text = %q", got.Todos[0].Text)
	}
}

func TestTranscriptSpeakerNameFallsBackToUnknown(t *testing.T) {
	if got := sampleTranscript().SpeakerName("999"); got != "Unknown speaker" {
		t.Errorf("SpeakerName for unknown id = %q", got)
	}
}
