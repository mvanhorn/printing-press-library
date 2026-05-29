// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONL_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONLWriter(&buf)
	if err := jw.WriteOK("a", json.RawMessage(`{"x":1}`)); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := jw.WriteError("b", &simpleErr{"boom"}); err != nil {
		t.Fatalf("write b: %v", err)
	}
	if err := jw.WriteOK("c", json.RawMessage(`{"y":"z"}`)); err != nil {
		t.Fatalf("write c: %v", err)
	}

	got := 0
	for entry := range ReadJSONLInputs(&buf) {
		if entry.Err != nil {
			t.Fatalf("unexpected scan err: %v", entry.Err)
		}
		got++
		// Each round-tripped record has its id field; verify the raw
		// payload is parseable JSON too.
		var probe map[string]any
		if err := json.Unmarshal(entry.Input.Payload, &probe); err != nil {
			t.Fatalf("payload not parseable JSON: %v", err)
		}
	}
	if got != 3 {
		t.Fatalf("expected 3 round-trip lines, got %d", got)
	}
}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

func TestJSONL_MalformedLineSurfaces(t *testing.T) {
	in := `{"id":"good1","payload":1}
this-is-not-json
{"id":"good2","payload":2}
`
	var good, bad int
	for entry := range ReadJSONLInputs(strings.NewReader(in)) {
		if entry.Err != nil {
			bad++
			continue
		}
		good++
		if entry.Input.ID != "good1" && entry.Input.ID != "good2" {
			t.Fatalf("unexpected id %q", entry.Input.ID)
		}
	}
	if good != 2 {
		t.Fatalf("expected 2 good lines, got %d", good)
	}
	if bad != 1 {
		t.Fatalf("expected 1 bad line, got %d", bad)
	}
}

func TestJSONL_EmptyInput(t *testing.T) {
	n := 0
	for range ReadJSONLInputs(strings.NewReader("")) {
		n++
	}
	if n != 0 {
		t.Fatalf("expected 0 entries from empty input, got %d", n)
	}
}

func TestJSONL_SkipsBlankLines(t *testing.T) {
	in := "\n\n   \n{\"id\":\"a\"}\n\n{\"id\":\"b\"}\n\n"
	got := 0
	for entry := range ReadJSONLInputs(strings.NewReader(in)) {
		if entry.Err != nil {
			t.Fatalf("unexpected err: %v", entry.Err)
		}
		got++
	}
	if got != 2 {
		t.Fatalf("expected 2 inputs, got %d", got)
	}
}

func TestJSONL_MissingIDSurfaces(t *testing.T) {
	in := `{"not_id":"x"}
{"id":"keeps-flowing"}
`
	var good, bad int
	for entry := range ReadJSONLInputs(strings.NewReader(in)) {
		if entry.Err != nil {
			bad++
			continue
		}
		good++
	}
	if good != 1 || bad != 1 {
		t.Fatalf("good=%d bad=%d", good, bad)
	}
}

func TestJSONL_WriterEnvelopeShape(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONLWriter(&buf)
	if err := jw.Write(JSONLResult{ID: "x", OK: true, Data: json.RawMessage(`{"a":1}`)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := jw.Write(JSONLResult{ID: "y", OK: false, Error: "nope"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %q", len(lines), buf.String())
	}
	// Each line is one JSON object — easy to parse.
	for i, line := range lines {
		var r JSONLResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("line %d not valid JSON: %v (line=%q)", i, err, line)
		}
	}
}
