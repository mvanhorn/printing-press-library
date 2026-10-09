// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"testing"
)

func TestStripControl(t *testing.T) {
	cases := map[string]string{
		"東京駅\x1b[31m赤\x1b[0m": "東京駅[31m赤[0m",
		"a\u009b2Jb":          "a2Jb",
		"tab\tok\nline":       "tab\tok\nline",
		"bell\x07del\x7f":     "belldel",
		"bad\xffbyte":         "badbyte",
		"a\u202eevil\u202cb":  "aevilb",
	}
	for in, want := range cases {
		if got := stripControl(in); got != want {
			t.Errorf("stripControl(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeTextWriterSplitRune(t *testing.T) {
	var out bytes.Buffer
	w := newSafeTextWriter(&out)
	b := []byte("駅\x1bX")
	// Split inside the 3-byte rune.
	if _, err := w.Write(b[:2]); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(b[2:]); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "駅X" {
		t.Fatalf("got %q", got)
	}
}

// Remote text must not carry terminal escapes into --plain, --csv or --quiet
// output either; those modes print decoded strings, not escaped JSON.
func TestMachineTextModesStripControl(t *testing.T) {
	view := map[string]any{
		"meta":    map[string]any{"command": "near"},
		"results": []map[string]any{{"id": "1\x1b]0;x\x07", "name": "東京駅\x1b[2J‮B1F", "note": "a\u009bb"}},
	}
	for _, mode := range []string{"plain", "csv", "quiet"} {
		flags := &rootFlags{}
		switch mode {
		case "plain":
			flags.plain = true
		case "csv":
			flags.csv = true
		case "quiet":
			flags.quiet = true
		}
		var out bytes.Buffer
		if err := printJSONFiltered(&out, view, flags); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		got := out.String()
		if got == "" {
			t.Fatalf("%s: no output", mode)
		}
		if stripControl(got) != got {
			t.Errorf("%s output keeps control characters: %q", mode, got)
		}
		if mode != "quiet" && !bytes.Contains(out.Bytes(), []byte("東京駅[2JB1F")) {
			t.Errorf("%s output lost the visible text: %q", mode, got)
		}
	}
}
