// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode"
)

// Remote text reaches --csv, --plain and --quiet through the generic
// printers. A terminal must not receive escape sequences, C1 controls or
// bidi overrides from it, whatever output mode the caller picks.
const hostileCell = "red\x1b[31m!\x1b[0m \u009b2J\u202eevil\rX\x07"

func assertNoControl(t *testing.T, mode, out string) {
	t.Helper()
	for _, r := range out {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatalf("%s output keeps control or format rune %U:\n%q", mode, r, out)
		}
	}
}

func TestTabularPrintersStripControlCharacters(t *testing.T) {
	rows, _ := json.Marshal([]map[string]any{
		{"name": hostileCell, "nested": map[string]any{"note": hostileCell}, "key\x1b[2Jx": "v"},
	})
	single, _ := json.Marshal(map[string]any{"title": hostileCell})
	scalar, _ := json.Marshal(hostileCell)

	for _, tc := range []struct {
		mode  string
		print func(*bytes.Buffer, json.RawMessage) error
	}{
		{"csv", func(b *bytes.Buffer, d json.RawMessage) error { return printCSV(b, d) }},
		{"plain", func(b *bytes.Buffer, d json.RawMessage) error { return printPlain(b, d) }},
		{"quiet", func(b *bytes.Buffer, d json.RawMessage) error { return printQuiet(b, d) }},
	} {
		for _, data := range []json.RawMessage{rows, single, scalar} {
			var buf bytes.Buffer
			if err := tc.print(&buf, data); err != nil {
				t.Fatalf("%s: %v", tc.mode, err)
			}
			out := buf.String()
			assertNoControl(t, tc.mode, out)
			if !strings.Contains(out, "evil") {
				t.Fatalf("%s output lost the visible text: %q", tc.mode, out)
			}
		}
	}
}

// Each --plain row must stay one line with one tab per column gap, so a
// remote value cannot add rows or columns.
func TestPlainKeepsRowAndColumnShape(t *testing.T) {
	data, _ := json.Marshal([]map[string]any{{"a": "x\ty\nz", "b": "ok"}})
	var buf bytes.Buffer
	if err := printPlain(&buf, data); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("plain output has %d lines, want header + 1 row: %q", len(lines), buf.String())
	}
	for _, l := range lines {
		if n := strings.Count(l, "\t"); n != 1 {
			t.Fatalf("plain line %q has %d tabs, want 1", l, n)
		}
	}
}

// The source command's JSON carries page text. JSON escapes C0 controls but
// not C1 controls or bidi overrides, so extraction must remove them.
func TestHTMLPageExtractionStripsControlCharacters(t *testing.T) {
	page := "<html><head><title>T\u009b2J‮itle</title>" +
		"<meta name=\"description\" content=\"d&#x9b;esc‎\"></head>" +
		"<body><a href=\"/en/x\">li\u009bnk</a></body></html>"
	out, err := extractHTMLResponse([]byte(page), htmlExtractionOptions{Mode: "page", BaseURL: "https://example.test/", ContentType: "text/html; charset=utf-8"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	assertNoControl(t, "source page", text)
	for _, want := range []string{`"title":"T2Jitle"`, `"description":"d›esc"`, `"name":"link"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("extraction output %s does not contain %s", text, want)
		}
	}
}

// This CLI has no 'list' command; a 404 hint must point at a command that
// exists.
func TestNotFoundHintNamesARealCommand(t *testing.T) {
	err := classifyAPIErrorOnly(errors.New("GET /en/tickets/: HTTP 404"))
	msg := err.Error()
	if strings.Contains(msg, "'list'") {
		t.Fatalf("404 hint names a command this CLI does not have: %q", msg)
	}
	if !strings.Contains(msg, "doctor") {
		t.Fatalf("404 hint is not actionable: %q", msg)
	}
	if RootCmd() == nil {
		t.Fatal("no root command")
	}
	if c, _, findErr := RootCmd().Find([]string{"doctor"}); findErr != nil || c.Name() != "doctor" {
		t.Fatalf("hint names 'doctor', but the command is missing: %v", findErr)
	}
}
