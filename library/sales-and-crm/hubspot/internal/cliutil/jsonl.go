// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// JSONL stdin/stdout envelope for agent-friendly bulk operations.
//
// The shape: each input line is a JSON object with a required "id" field plus
// arbitrary payload; each output line is a JSONLResult envelope reporting OK
// + Data or Error against the same id. Order is preserved when the caller
// loops serially over the input channel and writes one result per iteration.
//
// JSONL is the agent contract: agents (and humans) can stream rows on stdin,
// stream results on stdout, and never have to buffer the whole batch. A
// malformed input line yields a per-line error envelope on stdout instead of
// aborting the batch — the per-line envelope IS the error path.

package cliutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// JSONLInput is one parsed input line. ID is required (extracted from the
// "id" field); Payload is the raw JSON of the entire line so the caller can
// re-unmarshal into its own typed struct without re-reading stdin.
type JSONLInput struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"-"`
}

// JSONLInputOrError carries either a successfully-parsed input or a parse
// error that the caller should route directly to the output writer as a
// per-line error envelope. Either Input or Err is set; the other is zero.
//
// LineNum is 1-based and records the input line that produced this entry,
// useful for error envelopes when ID couldn't be parsed.
type JSONLInputOrError struct {
	LineNum int
	Input   JSONLInput
	Err     error
}

// JSONLResult is the per-line output envelope. Exactly one of Data or Error
// should be set: callers use OK=true + Data on success and OK=false + Error
// on failure. The id round-trips so an agent can correlate input rows with
// output rows even when results arrive out of order (concurrent workers).
type JSONLResult struct {
	ID    string          `json:"id"`
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// ReadJSONLInputs scans r line-by-line, returning one channel entry per
// non-empty line. Channel-based (not slice-based) so callers can start
// processing while stdin is still streaming — agents may feed tens of
// thousands of rows and buffering the whole input would defeat the purpose
// of the JSONL contract.
//
// Empty / whitespace-only lines are skipped. A line that fails JSON parsing
// is surfaced as a JSONLInputOrError with Err set and Input zero; the caller
// should route it straight to the output writer as an error envelope and
// continue. A line that parses as JSON but lacks an "id" field is also
// surfaced as an Err entry (no id means no correlation, no usable input).
//
// The returned channel is closed when r reaches EOF or returns an error.
// Scanner read errors (not parse errors) cause the channel to close after
// emitting one final error entry with LineNum set to the last line read.
func ReadJSONLInputs(r io.Reader) <-chan JSONLInputOrError {
	out := make(chan JSONLInputOrError, 32)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(r)
		// Allow long JSON lines: HubSpot batch upserts with verbose
		// properties can easily blow the default 64KB scanner buffer.
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			raw := scanner.Bytes()
			// Skip empty / whitespace-only lines silently — they're a
			// common artifact of human-curated JSONL files (trailing
			// newline, blank separator between batches).
			if isBlank(raw) {
				continue
			}
			// Copy: scanner.Bytes() reuses its internal buffer on the
			// next Scan(). The channel may outlive the scan call.
			line := make([]byte, len(raw))
			copy(line, raw)

			var probe struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(line, &probe); err != nil {
				out <- JSONLInputOrError{
					LineNum: lineNum,
					Err:     fmt.Errorf("line %d: invalid JSON: %w", lineNum, err),
				}
				continue
			}
			if probe.ID == "" {
				out <- JSONLInputOrError{
					LineNum: lineNum,
					Err:     fmt.Errorf("line %d: missing required \"id\" field", lineNum),
				}
				continue
			}
			out <- JSONLInputOrError{
				LineNum: lineNum,
				Input:   JSONLInput{ID: probe.ID, Payload: json.RawMessage(line)},
			}
		}
		if err := scanner.Err(); err != nil {
			out <- JSONLInputOrError{
				LineNum: lineNum,
				Err:     fmt.Errorf("reading input after line %d: %w", lineNum, err),
			}
		}
	}()
	return out
}

func isBlank(b []byte) bool {
	for _, c := range b {
		switch c {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return false
		}
	}
	return true
}

// JSONLWriter writes JSONLResult envelopes one-per-line. Thread-safe via an
// internal mutex so concurrent workers can call Write without coordination,
// but output order then reflects write order, not input order — typical
// pattern is a single serial loop over ReadJSONLInputs that calls Write per
// iteration, preserving input order trivially.
type JSONLWriter struct {
	mu sync.Mutex
	w  io.Writer
	// enc holds a json.Encoder over w; reuse avoids allocating a new
	// encoder per call. Encoder.Encode writes the trailing newline for us.
	enc *json.Encoder
}

// NewJSONLWriter wraps w in a thread-safe JSONL writer. The encoder
// disables HTML-escaping so payload strings containing `<`, `>`, `&` round-
// trip unchanged — agents reading the stream don't want their URLs
// gratuitously escaped.
func NewJSONLWriter(w io.Writer) *JSONLWriter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONLWriter{w: w, enc: enc}
}

// Write emits one result as a single line plus newline. Returns any
// underlying writer error verbatim so callers can decide to abort the batch.
func (jw *JSONLWriter) Write(result JSONLResult) error {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	return jw.enc.Encode(result)
}

// WriteError is a convenience wrapper for the common "failed-row" path. It
// composes the envelope from id + err so callers don't have to construct
// JSONLResult{OK: false, Error: err.Error()} at every error site.
func (jw *JSONLWriter) WriteError(id string, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return jw.Write(JSONLResult{ID: id, OK: false, Error: msg})
}

// WriteOK is a convenience wrapper for the common success path.
func (jw *JSONLWriter) WriteOK(id string, data json.RawMessage) error {
	return jw.Write(JSONLResult{ID: id, OK: true, Data: data})
}
