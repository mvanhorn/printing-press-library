package recordings

import (
	"strings"
	"testing"
	"time"
)

const sampleVTT = `WEBVTT

1
00:00:00.000 --> 00:00:05.000
Maya: Welcome to the Q2 planning call.

2
00:00:05.500 --> 00:00:11.000
Riley: Thanks Maya. Let's start with the pricing question.

3
00:00:11.000 --> 00:00:20.500
Maya: So we'd land at $99 for the Pro tier — that's the proposal.
`

func TestParseVTT(t *testing.T) {
	cues, err := ParseVTT(strings.NewReader(sampleVTT))
	if err != nil {
		t.Fatalf("ParseVTT: %v", err)
	}
	if len(cues) != 3 {
		t.Fatalf("expected 3 cues, got %d", len(cues))
	}
	want := []Cue{
		{Index: 1, Start: 0, End: 5 * time.Second, Speaker: "Maya", Text: "Welcome to the Q2 planning call."},
		{Index: 2, Start: 5500 * time.Millisecond, End: 11 * time.Second, Speaker: "Riley", Text: "Thanks Maya. Let's start with the pricing question."},
		{Index: 3, Start: 11 * time.Second, End: 20500 * time.Millisecond, Speaker: "Maya", Text: "So we'd land at $99 for the Pro tier — that's the proposal."},
	}
	for i, w := range want {
		if cues[i] != w {
			t.Errorf("cue %d: got %+v, want %+v", i, cues[i], w)
		}
	}
}

func TestParseVTTRejectsMissingHeader(t *testing.T) {
	_, err := ParseVTT(strings.NewReader("00:00:00.000 --> 00:00:01.000\nfoo\n"))
	if err == nil {
		t.Fatal("expected error for missing WEBVTT header")
	}
}
