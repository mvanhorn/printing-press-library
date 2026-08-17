// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"
)

// When both --regex and --tense are given, hits must match BOTH (the regex
// must not be silently discarded in favor of the tense preset).
func TestConcordanceRegexAndTenseCombine(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "corpus.db")

	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cues := []store.TranscriptCue{
		{VideoID: "vid1", Index: 0, StartMS: 0, EndMS: 1000, Text: "yo hablaba con ella"},
		// Matches the imperfect tense preset but NOT the regex below. Under
		// the old tense-overrides-regex behavior this cue would be returned.
		{VideoID: "vid1", Index: 1, StartMS: 2000, EndMS: 3000, Text: "ella comía pan"},
	}
	if err := s.ReplaceTranscript("vid1", cues); err != nil {
		t.Fatalf("ReplaceTranscript: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := &rootFlags{asJSON: true}
	cmd := newNovelConcordanceCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--regex", `\bhablaba\b`, "--tense", "imperfect", "--db", dbPath})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("concordance: %v", err)
	}

	var hits []store.ConcordanceHit
	if err := json.Unmarshal(out.Bytes(), &hits); err != nil {
		t.Fatalf("unmarshal output %q: %v", out.String(), err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want exactly 1 (regex AND tense): %s", len(hits), out.String())
	}
	if !strings.Contains(hits[0].Text, "hablaba") {
		t.Fatalf("hit %q does not match the --regex; regex was discarded", hits[0].Text)
	}
}
