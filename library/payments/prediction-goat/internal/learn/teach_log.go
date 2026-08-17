// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// teach_log.go is the JSONL writer/reader for teach-time warnings.
//
// Why a structured log alongside the existing plain-text teach.log:
// the existing helper in internal/cli/teach.go writes "%s %s\n" lines
// for backgrounded error diagnostics (DB open failures, missing
// --query, etc). The U6 warnings carry structured fields (code,
// resource, suggested-substitute) the LLM and `learnings list
// --warnings` need to read back, so they ship as JSONL.
//
// The two formats coexist in the same file: writers always append
// well-formed JSON objects ending in '\n', and the reader defensively
// skips any line that doesn't start with '{' so the legacy plain-text
// entries don't break parsing.
//
// File location: ~/.local/share/prediction-goat-pp-cli/teach.log. The
// directory is created on first write with mode 0o700 to mirror the
// existing learnings.jsonl convention.
//
// Per the U6 section of
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md.

package learn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// teachLogStateDirName is the per-user state directory under
// $HOME/.local/share. Kept as a constant so tests that set $HOME
// against a temp dir see the same path.
const teachLogStateDirName = "prediction-goat-pp-cli"

// teachLogFileName is the on-disk log name. Shared with the plain-text
// error log written from the CLI side -- structured JSONL warnings and
// plain-text errors coexist; the reader skips non-JSON lines.
const teachLogFileName = "teach.log"

// TeachLogEntry is the on-disk record shape for one structured teach-
// log warning. Round-trippable: writers marshal, readers unmarshal.
// Added fields must keep `omitempty` so older readers parsing a
// newer-shape entry still see a valid record.
type TeachLogEntry struct {
	// TS is the RFC3339 UTC timestamp of the write.
	TS string `json:"ts"`

	// Action is the originating CLI action ("teach", for now). Future
	// validators that emit warnings from other commands (e.g.,
	// recipe-extraction at sync time) can set their own action label.
	Action string `json:"action"`

	// Query is the original user-facing query the teach call carried.
	// Kept verbatim (no normalization) so a human can grep for the
	// exact phrasing.
	Query string `json:"query,omitempty"`

	// Resource is the resource_id the warning is about.
	Resource string `json:"resource,omitempty"`

	// Warning is the warning code from learn.Warning.Code.
	Warning string `json:"warning"`

	// Detail is the human-readable explanation.
	Detail string `json:"detail,omitempty"`

	// Suggested optionally names the substitute resource_id the LLM
	// could have used.
	Suggested string `json:"suggested,omitempty"`
}

// TeachLogPath returns the on-disk path to the structured teach log.
// Creates the parent directory on first call (mode 0o700) so the
// caller doesn't have to. Exported so tests in other packages can
// point at the same file the writer uses.
func TeachLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("teach log: resolve home: %w", err)
	}
	dir := filepath.Join(home, ".local", "share", teachLogStateDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("teach log: mkdir %s: %w", dir, err)
	}
	return filepath.Join(dir, teachLogFileName), nil
}

// AppendTeachLogWarning writes one structured warning to the teach
// log. Best-effort: returns an error so callers that want to know
// can act on it, but the canonical teach-time hook ignores the error
// (the teach itself has already succeeded by the time this runs).
//
// File handling: opens with O_APPEND|O_CREATE|O_WRONLY at 0o600.
// POSIX guarantees atomic appends for writes <= PIPE_BUF, and one
// JSON entry plus newline always fits inside that budget for the
// fields we serialize. This is the same atomic-append pattern the
// existing learnings.jsonl writer uses.
func AppendTeachLogWarning(action, query string, w Warning) error {
	if w.Code == "" {
		return fmt.Errorf("teach log: warning code is required")
	}
	entry := TeachLogEntry{
		TS:        time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		Query:     query,
		Resource:  w.Resource,
		Warning:   w.Code,
		Detail:    w.Detail,
		Suggested: w.Suggested,
	}
	return appendTeachLogEntry(entry)
}

// appendTeachLogEntry is the marshal-and-write path shared by the
// production writer and tests that want to seed the log directly.
// Unexported because callers should go through AppendTeachLogWarning;
// exposing it would invite shape drift.
func appendTeachLogEntry(entry TeachLogEntry) error {
	p, err := TeachLogPath()
	if err != nil {
		return err
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("teach log: marshal: %w", err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("teach log: open %s: %w", p, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("teach log: write: %w", err)
	}
	return nil
}

// ReadTeachLogWarnings returns every structured warning recorded in
// the teach log, optionally filtered by resource_id. Non-JSON lines
// (the legacy plain-text format the existing CLI helper writes for
// error diagnostics) are silently skipped so the two formats can
// share a file without confusing the reader.
//
// Missing file returns (nil, nil) -- not an error. Callers expect
// "no warnings yet" to be the common case on a fresh install.
//
// When resourceIDs is empty, all entries are returned. When one or
// more IDs are supplied, only entries whose Resource field is in the
// filter set are returned.
func ReadTeachLogWarnings(resourceIDs ...string) ([]TeachLogEntry, error) {
	p, err := TeachLogPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p) // #nosec G304 -- path is per-user state under $HOME.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("teach log: open %s: %w", p, err)
	}
	defer f.Close()

	filter := make(map[string]struct{}, len(resourceIDs))
	for _, id := range resourceIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		filter[id] = struct{}{}
	}
	wantFilter := len(filter) > 0

	var out []TeachLogEntry
	scanner := bufio.NewScanner(f)
	// Bump the buffer so long detail strings don't overflow the
	// default 64KiB token limit. JSONL entries are kept compact, but
	// future entries with embedded URLs or stack traces would
	// otherwise truncate silently.
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip blank lines and legacy plain-text error log lines.
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var entry TeachLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// A malformed JSON line is informational, not fatal.
			// Skipping it lets a future corrupt entry not block the
			// whole inspection surface.
			continue
		}
		if wantFilter {
			if _, ok := filter[entry.Resource]; !ok {
				continue
			}
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("teach log: scan: %w", err)
	}
	return out, nil
}
