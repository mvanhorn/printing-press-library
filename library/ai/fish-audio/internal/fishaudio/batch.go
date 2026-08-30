// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package fishaudio

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// BatchLine is one unit of work parsed from a batch input file.
type BatchLine struct {
	// LineNo is the 1-based source line, used to name the output file and to
	// point at the offending line in an error.
	LineNo int `json:"line_no"`
	// Text is the text to render.
	Text string `json:"text"`
	// Voice overrides the batch-level --voice for this line. Empty means use
	// the batch default.
	Voice string `json:"voice,omitempty"`
	// Speaker is the label from a `Name: line` dialogue script.
	Speaker string `json:"speaker,omitempty"`
}

// speakerLine matches a dialogue script line: a short label, a colon, then the
// spoken text. The label is deliberately narrow so a line of ordinary prose
// containing a colon is not mistaken for a speaker turn.
var speakerLine = regexp.MustCompile(`^([\p{L}\p{N}][\p{L}\p{N} '._-]{0,39}):[ \t]+(\S.*)$`)

// ParseBatchInput reads a batch input file into lines. Three shapes are
// accepted, decided per line:
//
//   - a JSON object, `{"text": "...", "voice": "..."}` (JSONL)
//   - a dialogue turn, `Alice: Hello there` (only when dialogue is true)
//   - plain text, the whole line
//
// Blank lines and lines starting with `#` are skipped.
func ParseBatchInput(content string, dialogue bool) ([]BatchLine, error) {
	lines := make([]BatchLine, 0)
	for i, raw := range strings.Split(content, "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			var obj struct {
				Text    string `json:"text"`
				Voice   string `json:"voice"`
				Speaker string `json:"speaker"`
			}
			if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
				return nil, fmt.Errorf("line %d: parsing JSONL record: %w", lineNo, err)
			}
			if strings.TrimSpace(obj.Text) == "" {
				return nil, fmt.Errorf("line %d: JSONL record has no %q field", lineNo, "text")
			}
			lines = append(lines, BatchLine{LineNo: lineNo, Text: obj.Text, Voice: obj.Voice, Speaker: obj.Speaker})
			continue
		}
		if dialogue {
			if m := speakerLine.FindStringSubmatch(trimmed); m != nil {
				lines = append(lines, BatchLine{LineNo: lineNo, Speaker: strings.TrimSpace(m[1]), Text: strings.TrimSpace(m[2])})
				continue
			}
			return nil, fmt.Errorf("line %d: --dialogue expects %q lines; got %q", lineNo, "Speaker: text", trimmed)
		}
		lines = append(lines, BatchLine{LineNo: lineNo, Text: trimmed})
	}
	return lines, nil
}

// ParseSpeakerMap turns repeated `--speaker-map name=model_id` values into a
// lookup.
func ParseSpeakerMap(pairs []string) (map[string]string, error) {
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		name, id, ok := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		id = strings.TrimSpace(id)
		if !ok || name == "" || id == "" {
			return nil, fmt.Errorf("invalid --speaker-map value %q: expected name=model_id", pair)
		}
		out[name] = id
	}
	return out, nil
}

// Dialogue is one multi-speaker request: a single tagged text plus the
// reference_id array whose positions the tags index.
type Dialogue struct {
	Text         string   `json:"text"`
	ReferenceIDs []string `json:"reference_ids"`
	Speakers     []string `json:"speakers"`
}

// BuildDialogue folds dialogue lines into one `<|speaker:N|>`-tagged request.
// Speakers are numbered in order of first appearance and each one must have a
// --speaker-map entry, so an unmapped name fails locally rather than
// rendering in the wrong voice.
func BuildDialogue(lines []BatchLine, speakerMap map[string]string) (Dialogue, error) {
	var d Dialogue
	d.ReferenceIDs = make([]string, 0)
	d.Speakers = make([]string, 0)
	index := map[string]int{}
	var parts []string
	for _, line := range lines {
		speaker := line.Speaker
		if speaker == "" {
			return Dialogue{}, fmt.Errorf("line %d: --dialogue requires a speaker label", line.LineNo)
		}
		idx, seen := index[speaker]
		if !seen {
			id := line.Voice
			if id == "" {
				id = speakerMap[speaker]
			}
			if id == "" {
				return Dialogue{}, fmt.Errorf("speaker %q has no voice: pass --speaker-map %s=<model_id>", speaker, speaker)
			}
			idx = len(d.ReferenceIDs)
			index[speaker] = idx
			d.ReferenceIDs = append(d.ReferenceIDs, id)
			d.Speakers = append(d.Speakers, speaker)
		}
		parts = append(parts, "<|speaker:"+strconv.Itoa(idx)+"|>"+line.Text)
	}
	d.Text = strings.Join(parts, "\n")
	return d, nil
}

// BatchOutputName builds the numbered output filename for one batch line.
func BatchOutputName(index int, format string) string {
	return fmt.Sprintf("%04d.%s", index, format)
}
