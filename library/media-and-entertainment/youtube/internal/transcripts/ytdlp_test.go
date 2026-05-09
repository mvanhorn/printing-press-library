package transcripts

import (
	"strings"
	"testing"
)

func TestPlainText_NilAndEmpty(t *testing.T) {
	if got := (*Result)(nil).PlainText(); got != "" {
		t.Errorf("nil receiver = %q; want empty string", got)
	}
	r := &Result{}
	if got := r.PlainText(); got != "" {
		t.Errorf("empty Content = %q; want empty string", got)
	}
}

func TestPlainText_StripsVTTCuesAndHeaders(t *testing.T) {
	const vtt = `WEBVTT
Kind: captions
Language: en

00:00:00.000 --> 00:00:02.500
Hello world

00:00:02.500 --> 00:00:05.000
This is a transcript

NOTE this is editor notation
00:00:05.000 --> 00:00:07.000
Final line
`
	r := &Result{Content: vtt}
	got := r.PlainText()
	want := "Hello world\nThis is a transcript\nFinal line"
	if got != want {
		t.Errorf("PlainText() =\n%q\nwant\n%q", got, want)
	}
}

func TestPlainText_StripsInlineTimestampsAndTags(t *testing.T) {
	const vtt = `WEBVTT

00:00:00.000 --> 00:00:02.500
<00:00:00.500><c>Inline</c> <b>tags</b> and timestamps
`
	r := &Result{Content: vtt}
	got := r.PlainText()
	if strings.Contains(got, "<") || strings.Contains(got, "00:00:00.500") {
		t.Errorf("PlainText() did not strip inline tags or timestamps: %q", got)
	}
	if !strings.Contains(got, "Inline") || !strings.Contains(got, "tags") {
		t.Errorf("PlainText() over-stripped content: %q", got)
	}
}

func TestPlainText_DeduplicatesAdjacentLines(t *testing.T) {
	const vtt = `WEBVTT

00:00:00.000 --> 00:00:02.500
duplicate line

00:00:02.500 --> 00:00:05.000
duplicate line

00:00:05.000 --> 00:00:07.000
distinct line
`
	r := &Result{Content: vtt}
	got := r.PlainText()
	// "duplicate line" should appear only once because the second
	// occurrence is adjacent to the first.
	if strings.Count(got, "duplicate line") != 1 {
		t.Errorf("PlainText() did not deduplicate adjacent lines: %q", got)
	}
	if !strings.Contains(got, "distinct line") {
		t.Errorf("PlainText() dropped distinct line: %q", got)
	}
}

func TestEnsureYTDLP_ErrorMessageIsActionable(t *testing.T) {
	// We can't easily ensure yt-dlp is missing in the test environment, so
	// we only verify the error message shape if the lookup happens to fail.
	_, err := EnsureYTDLP()
	if err == nil {
		// yt-dlp is installed — nothing to assert here, just confirm no panic.
		return
	}
	msg := err.Error()
	if !strings.Contains(msg, "yt-dlp") || !strings.Contains(msg, "install") {
		t.Errorf("EnsureYTDLP error %q should mention yt-dlp and install hint", msg)
	}
}
