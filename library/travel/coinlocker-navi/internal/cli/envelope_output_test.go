package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPrintQuietSingleRecordEnvelope(t *testing.T) {
	data := json.RawMessage(`{"meta":{"command":"locker"},"results":{"id":"2685","name":"x","neighbours":[{"id":"1"},{"id":"2"}]}}`)
	var buf bytes.Buffer
	if err := printQuiet(&buf, data); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "2685\n" {
		t.Fatalf("quiet output = %q, want %q", got, "2685\n")
	}
}

func TestPrintQuietListEnvelope(t *testing.T) {
	data := json.RawMessage(`{"meta":{"command":"near"},"results":[{"id":"3"},{"id":"151"}]}`)
	var buf bytes.Buffer
	if err := printQuiet(&buf, data); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "3\n151\n" {
		t.Fatalf("quiet output = %q, want %q", got, "3\n151\n")
	}
}

func TestWrapSelectedEnvelopeKeepsMetaPath(t *testing.T) {
	selected := json.RawMessage(`{"meta":{"fetched_at":"2026-10-10T00:00:00+09:00"}}`)
	out, err := wrapSelectedEnvelope(selected, map[string]any{"source": "live"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Meta    map[string]any  `json:"meta"`
		Results json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Meta["fetched_at"] != "2026-10-10T00:00:00+09:00" {
		t.Fatalf("meta.fetched_at missing: %s", out)
	}
	if got.Meta["source"] != "live" {
		t.Fatalf("meta.source = %v, want live: %s", got.Meta["source"], out)
	}
	if got.Results != nil {
		t.Fatalf("unexpected results: %s", out)
	}
}
