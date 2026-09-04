// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestEmitStructured_CSVTopLevelArray(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	flags := &rootFlags{csv: true}

	in := []map[string]any{{"name": "a", "amount": 2.5}, {"name": "b", "amount": 3.0}}
	if err := flags.emitStructured(cmd, in); err != nil {
		t.Fatalf("emitStructured error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.Contains(out, "amount,name") {
		t.Fatalf("expected CSV header, got %q", out)
	}
	if !strings.Contains(out, "2.5,a") || !strings.Contains(out, "3,b") {
		t.Fatalf("expected CSV rows, got %q", out)
	}
}

func TestEmitStructured_CSVUnwrapSingleArrayEnvelope(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	flags := &rootFlags{csv: true}

	in := map[string]any{
		"items":            []map[string]any{{"description": "rent", "total": 100.0}},
		"scanned_expenses": 5,
	}
	if err := flags.emitStructured(cmd, in); err != nil {
		t.Fatalf("emitStructured error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.Contains(out, "description,total") {
		t.Fatalf("expected unwrapped CSV header, got %q", out)
	}
	if strings.Contains(out, "scanned_expenses") {
		t.Fatalf("did not expect envelope metadata in CSV output, got %q", out)
	}
}

func TestEmitStructured_CSVMultiArrayFallsBackToJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	flags := &rootFlags{csv: true}

	in := map[string]any{
		"by_currency": []map[string]any{{"currency": "USD", "net": 4.0}},
		"friends":     []map[string]any{{"name": "alex", "amount": 4.0}},
	}
	if err := flags.emitStructured(cmd, in); err != nil {
		t.Fatalf("emitStructured error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") {
		t.Fatalf("expected JSON fallback, got %q", out)
	}
}

func TestEmitStructured_CSVHonoursSelect(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	flags := &rootFlags{csv: true, selectFields: "transfers"}
	in := map[string]any{
		"transfers": []map[string]any{{"x": 1}},
		"friends":   []map[string]any{{"name": "Example Friend"}},
	}
	if err := flags.emitStructured(cmd, in); err != nil {
		t.Fatal(err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "x\n1" {
		t.Fatalf("selected CSV = %q", out)
	}
}

func TestEmitStructured_PlainMultiArrayFallsBackToJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	flags := &rootFlags{plain: true}
	in := map[string]any{"first": []map[string]any{{"x": 1}}, "second": []map[string]any{{"y": 2}}}
	if err := flags.emitStructured(cmd, in); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil || len(got) != 2 {
		t.Fatalf("plain fallback = %q, err=%v", buf.String(), err)
	}
}

func TestEmitStructured_CSVIgnoresEmptyAndMetadataArrays(t *testing.T) {
	for name, in := range map[string]map[string]any{
		"null":     {"transfers": []map[string]any{{"x": 1}}, "skipped": nil},
		"warnings": {"transfers": []map[string]any{{"x": 1}}, "warnings": []any{}},
	} {
		t.Run(name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			cmd := &cobra.Command{}
			cmd.SetOut(buf)
			if err := (&rootFlags{csv: true}).emitStructured(cmd, in); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(buf.String()); got != "x\n1" {
				t.Fatalf("CSV = %q", got)
			}
		})
	}
}

func TestEmitStructured_PlainTopLevelArray(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	flags := &rootFlags{plain: true}

	in := []map[string]any{{"name": "a", "amount": 2.5}, {"name": "b", "amount": 3.0}}
	if err := flags.emitStructured(cmd, in); err != nil {
		t.Fatalf("emitStructured error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.Contains(out, "amount\tname") {
		t.Fatalf("expected tab header, got %q", out)
	}
	if !strings.Contains(out, "2.5\ta") || !strings.Contains(out, "3\tb") {
		t.Fatalf("expected tab rows, got %q", out)
	}
}

func TestEmitStructured_DefaultMatchesPrintJSON(t *testing.T) {
	bufEmit := &bytes.Buffer{}
	cmdEmit := &cobra.Command{}
	cmdEmit.SetOut(bufEmit)
	flags := &rootFlags{}

	in := map[string]any{"items": []map[string]any{{"id": 1, "name": "x"}}}
	if err := flags.emitStructured(cmdEmit, in); err != nil {
		t.Fatalf("emitStructured error: %v", err)
	}

	bufJSON := &bytes.Buffer{}
	cmdJSON := &cobra.Command{}
	cmdJSON.SetOut(bufJSON)
	if err := flags.printJSON(cmdJSON, in); err != nil {
		t.Fatalf("printJSON error: %v", err)
	}

	var got any
	if err := json.Unmarshal(bufEmit.Bytes(), &got); err != nil {
		t.Fatalf("emit output is not valid JSON: %v", err)
	}
	var want any
	if err := json.Unmarshal(bufJSON.Bytes(), &want); err != nil {
		t.Fatalf("printJSON output is not valid JSON: %v", err)
	}
	gotNorm, _ := json.Marshal(got)
	wantNorm, _ := json.Marshal(want)
	if string(gotNorm) != string(wantNorm) {
		t.Fatalf("default emit JSON mismatch: got %s want %s", string(gotNorm), string(wantNorm))
	}
}
