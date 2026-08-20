// Behavioral tests for the --plain output mode wired through
// printOutputWithFlags. --plain forces tab-separated rows regardless of
// TTY so the output stays scriptable when piped.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintPlainArray(t *testing.T) {
	t.Parallel()
	data := json.RawMessage(`[{"id":"evt_1","name":"Show A","status":"on_sale"},{"id":"evt_2","name":"Show B","status":"sold_out"}]`)
	var buf bytes.Buffer
	if err := printPlain(&buf, data); err != nil {
		t.Fatalf("printPlain returned error: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 1 header + 2 rows, got %d lines: %q", len(lines), out)
	}
	// Header and rows must be tab-separated, not space-padded.
	if !strings.Contains(lines[0], "\t") {
		t.Errorf("header row is not tab-separated: %q", lines[0])
	}
	if !strings.Contains(lines[1], "evt_1") || !strings.Contains(lines[1], "Show A") {
		t.Errorf("first data row missing expected values: %q", lines[1])
	}
}

func TestPrintPlainObject(t *testing.T) {
	t.Parallel()
	data := json.RawMessage(`{"id":"evt_1","name":"Show A"}`)
	var buf bytes.Buffer
	if err := printPlain(&buf, data); err != nil {
		t.Fatalf("printPlain returned error: %v", err)
	}
	out := buf.String()
	// Single object emits key<TAB>value pairs, one per line, sorted by key.
	if !strings.Contains(out, "id\tevt_1") {
		t.Errorf("expected 'id\\tevt_1' pair, got: %q", out)
	}
	if !strings.Contains(out, "name\tShow A") {
		t.Errorf("expected 'name\\tShow A' pair, got: %q", out)
	}
}

func TestPrintOutputWithFlagsPlainRoute(t *testing.T) {
	t.Parallel()
	// --plain routes through printPlain even when stdout is not a TTY.
	flags := &rootFlags{plain: true}
	data := json.RawMessage(`[{"id":"evt_1","name":"Show A"}]`)
	var buf bytes.Buffer
	if err := printOutputWithFlags(&buf, data, flags); err != nil {
		t.Fatalf("printOutputWithFlags returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "evt_1") {
		t.Errorf("plain route produced no recognizable output: %q", buf.String())
	}
}

func TestPrintOutputWithFlagsJSONWinsOverPlain(t *testing.T) {
	t.Parallel()
	// --json is the more explicit machine format and wins when both are set.
	flags := &rootFlags{plain: true, asJSON: true}
	data := json.RawMessage(`{"id":"evt_1"}`)
	var buf bytes.Buffer
	if err := printOutputWithFlags(&buf, data, flags); err != nil {
		t.Fatalf("printOutputWithFlags returned error: %v", err)
	}
	// JSON output is brace-wrapped; plain key<TAB>value output is not.
	if !strings.Contains(buf.String(), "{") {
		t.Errorf("expected JSON output when --json set alongside --plain, got: %q", buf.String())
	}
}

func TestPrintOutputWithFlagsJSONWinsOverCSV(t *testing.T) {
	t.Parallel()
	// --json is the more explicit machine format and wins when both are set.
	flags := &rootFlags{csv: true, asJSON: true}
	data := json.RawMessage(`[{"id":"evt_1","name":"Show A"}]`)
	var buf bytes.Buffer
	if err := printOutputWithFlags(&buf, data, flags); err != nil {
		t.Fatalf("printOutputWithFlags returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "{") || strings.Contains(out, "id,name\n") {
		t.Errorf("expected JSON output when --json set alongside --csv, got: %q", out)
	}
}
