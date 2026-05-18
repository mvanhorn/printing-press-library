// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package ingest

import (
	"io"
	"strings"
	"testing"
)

func TestNormalizeNNumber(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"12345", "N12345"},
		{"N12345", "N12345"},
		{"n12345", "N12345"},
		{" 628TS ", "N628TS"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeNNumber(c.in); got != c.want {
			t.Errorf("normalizeNNumber(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFAADate(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"20260512", "2026-05-12"},
		{"", ""},
		{"badinput", "badinput"}, // passes through so callers can debug
		{"  20150101  ", "2015-01-01"},
	}
	for _, c := range cases {
		if got := faaDate(c.in); got != c.want {
			t.Errorf("faaDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestColumnIndex(t *testing.T) {
	idx := columnIndex([]string{"N-NUMBER", " serial number ", "Year MFR"})
	if idx["N-NUMBER"] != 0 {
		t.Errorf("N-NUMBER index = %d, want 0", idx["N-NUMBER"])
	}
	if idx["SERIAL NUMBER"] != 1 {
		t.Errorf("SERIAL NUMBER index = %d, want 1 (trimmed/upper)", idx["SERIAL NUMBER"])
	}
	if idx["YEAR MFR"] != 2 {
		t.Errorf("YEAR MFR index = %d, want 2", idx["YEAR MFR"])
	}
}

func TestColGetters(t *testing.T) {
	row := []string{"N12345", "  SN-1  ", "1976", ""}
	idx := map[string]int{"REG": 0, "SERIAL": 1, "YEAR": 2, "MISSING": 3}

	if got := col(row, idx, "SERIAL"); got != "SN-1" {
		t.Errorf("col(SERIAL) = %q, want %q", got, "SN-1")
	}
	if got := col(row, idx, "MISSING"); got != "" {
		t.Errorf("col(MISSING) = %q, want empty", got)
	}
	if got := col(row, idx, "NOT_IN_HEADER"); got != "" {
		t.Errorf("col(NOT_IN_HEADER) = %q, want empty", got)
	}

	if v, ok := colInt(row, idx, "YEAR"); !ok || v != 1976 {
		t.Errorf("colInt(YEAR) = (%d, %v), want (1976, true)", v, ok)
	}
	if _, ok := colInt(row, idx, "MISSING"); ok {
		t.Errorf("colInt(MISSING) should be (_, false)")
	}
	if _, ok := colInt(row, idx, "SERIAL"); ok {
		t.Errorf("colInt(SERIAL) should be (_, false) — not a number")
	}

	if got := nullableInt(row, idx, "YEAR"); got != 1976 {
		t.Errorf("nullableInt(YEAR) = %v, want 1976", got)
	}
	if got := nullableInt(row, idx, "MISSING"); got != nil {
		t.Errorf("nullableInt(MISSING) = %v, want nil", got)
	}

	if got := nullableStr("foo"); got != "foo" {
		t.Errorf("nullableStr(\"foo\") = %v, want \"foo\"", got)
	}
	if got := nullableStr(""); got != nil {
		t.Errorf("nullableStr(\"\") = %v, want nil", got)
	}
}

func TestBomStripper(t *testing.T) {
	// With BOM
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello,world")...)
	b := &bomStripper{r: strings.NewReader(string(withBOM))}
	got, err := io.ReadAll(b)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "hello,world" {
		t.Errorf("with BOM: got %q, want %q", string(got), "hello,world")
	}

	// Without BOM
	b2 := &bomStripper{r: strings.NewReader("plain,text")}
	got2, err := io.ReadAll(b2)
	if err != nil {
		t.Fatalf("ReadAll plain: %v", err)
	}
	if string(got2) != "plain,text" {
		t.Errorf("without BOM: got %q, want %q", string(got2), "plain,text")
	}

	// Short read shorter than BOM length — exercises the io.ErrUnexpectedEOF branch
	b3 := &bomStripper{r: strings.NewReader("hi")}
	got3, err := io.ReadAll(b3)
	if err != nil {
		t.Fatalf("ReadAll short: %v", err)
	}
	if string(got3) != "hi" {
		t.Errorf("short read: got %q, want %q", string(got3), "hi")
	}
}
