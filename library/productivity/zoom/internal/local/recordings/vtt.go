// Package recordings walks the local Zoom recordings folder, parses VTT
// transcripts, and exposes structured access to the data the desktop client
// writes to disk after each meeting. Used by `recordings local sync`, `find`,
// `recordings analyze`, `storage`, and `recordings export`.
package recordings

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// Cue is one parsed VTT entry. Zoom labels each cue with a speaker via the
// pattern "Speaker Name: utterance" inside the cue body — we extract the
// speaker into its own field when present.
type Cue struct {
	Index   int           `json:"index"`
	Start   time.Duration `json:"start"`
	End     time.Duration `json:"end"`
	Speaker string        `json:"speaker,omitempty"`
	Text    string        `json:"text"`
}

var (
	timecodePattern = regexp.MustCompile(`^(\d{2}):(\d{2}):(\d{2})\.(\d{3})\s+-->\s+(\d{2}):(\d{2}):(\d{2})\.(\d{3})`)
	// Zoom prefixes the cue body with "Speaker Name: " when known. We accept
	// any colon-separated leader where the speaker is non-empty and does not
	// look like a timecode fragment.
	speakerPrefix = regexp.MustCompile(`^([^:\n]{1,80}):\s+(.+)$`)
)

// ParseVTT reads a WebVTT file and returns the parsed cues. Empty input,
// missing WEBVTT header, or malformed timecodes return an error; individual
// cue parsing errors skip the cue and continue.
func ParseVTT(r io.Reader) ([]Cue, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	// First non-empty line should be the WEBVTT header.
	saw := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "WEBVTT") {
			return nil, fmt.Errorf("vtt: missing WEBVTT header (got %q)", line)
		}
		saw = true
		break
	}
	if !saw {
		return nil, errors.New("vtt: empty file")
	}

	var cues []Cue
	idx := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Skip cue identifier line if present (a bare integer or string with
		// no "-->"). The next non-empty line will be the timecode.
		if !timecodePattern.MatchString(line) {
			// Could be a cue id; read the next line and try again.
			if !sc.Scan() {
				break
			}
			line = strings.TrimSpace(sc.Text())
			if !timecodePattern.MatchString(line) {
				// Not a cue at all (NOTE block or stray text). Skip.
				continue
			}
		}
		m := timecodePattern.FindStringSubmatch(line)
		start := mustDur(m[1], m[2], m[3], m[4])
		end := mustDur(m[5], m[6], m[7], m[8])
		// Read the cue body until a blank line.
		var body strings.Builder
		for sc.Scan() {
			b := sc.Text()
			if strings.TrimSpace(b) == "" {
				break
			}
			if body.Len() > 0 {
				body.WriteByte(' ')
			}
			body.WriteString(strings.TrimSpace(b))
		}
		text := body.String()
		speaker := ""
		if mm := speakerPrefix.FindStringSubmatch(text); mm != nil && !timecodePattern.MatchString(mm[1]) {
			speaker = strings.TrimSpace(mm[1])
			text = strings.TrimSpace(mm[2])
		}
		idx++
		cues = append(cues, Cue{
			Index: idx, Start: start, End: end, Speaker: speaker, Text: text,
		})
	}
	if err := sc.Err(); err != nil {
		return cues, fmt.Errorf("vtt: scan: %w", err)
	}
	return cues, nil
}

// ParseVTTFile opens path and calls ParseVTT.
func ParseVTTFile(path string) ([]Cue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseVTT(f)
}

func mustDur(h, m, s, ms string) time.Duration {
	var hh, mm, ss, mss int
	fmt.Sscanf(h, "%d", &hh)
	fmt.Sscanf(m, "%d", &mm)
	fmt.Sscanf(s, "%d", &ss)
	fmt.Sscanf(ms, "%d", &mss)
	return time.Duration(hh)*time.Hour +
		time.Duration(mm)*time.Minute +
		time.Duration(ss)*time.Second +
		time.Duration(mss)*time.Millisecond
}
