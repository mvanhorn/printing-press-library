package nepraparse

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// TestFixturesAreVerbatimPublishedBytes pins the fixtures to the published
// files, so every other measured number in this suite is being asserted
// against the real thing and not against a re-encoded copy.
func TestFixturesAreVerbatimPublishedBytes(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			data := fixture(t, y.name)
			if len(data) != y.bytes {
				t.Errorf("byte size = %d, want %d", len(data), y.bytes)
			}
			if got := bytes.Count(data, []byte{0xA0}); got != y.nbspBytes {
				t.Errorf("0xA0 (NBSP) byte count = %d, want %d", got, y.nbspBytes)
			}
			// 0xA0 is the ONLY non-ASCII byte in these files.
			for i, b := range data {
				if b >= 0x80 && b != 0xA0 {
					t.Fatalf("unexpected non-ASCII byte %#x at offset %d", b, i)
					break
				}
			}
		})
	}
}

// TestStrictUTF8ReadFails is the encoding trap. The HTTP response declares no
// charset, so a client that assumes UTF-8 is decoding invalid bytes: the
// 0xA0s are not valid UTF-8 sequences and a strict decode fails outright.
func TestStrictUTF8ReadFails(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			data := fixture(t, y.name)
			if utf8.Valid(data) {
				t.Fatal("fixture is valid UTF-8; the windows-1252 handling in this package would be untested")
			}
			// Only the in-document meta declares the charset.
			if got := DeclaredCharset(data); got != CharsetWindows1252 {
				t.Errorf("DeclaredCharset = %q, want %q", got, CharsetWindows1252)
			}
			text, charset, _, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if charset != CharsetWindows1252 {
				t.Errorf("applied charset = %q, want %q", charset, CharsetWindows1252)
			}
			if !utf8.ValidString(text) {
				t.Error("decoded text is not valid UTF-8")
			}
			if got := strings.Count(text, " "); got != y.nbspBytes {
				t.Errorf("decoded NBSP runes = %d, want %d", got, y.nbspBytes)
			}
			// The decode must not disturb anything else: ASCII length is
			// preserved and only the NBSPs grew from one byte to two.
			if want := y.bytes + y.nbspBytes; len(text) != want {
				t.Errorf("decoded length = %d bytes, want %d", len(text), want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantText     string
		wantCharset  string
		wantDeclared bool
		wantErr      error
	}{
		{
			name:         "declared windows-1252 with NBSP",
			in:           `<meta http-equiv=Content-Type content="text/html; charset=windows-1252"><td>\xa0</td>`,
			wantText:     "<meta http-equiv=Content-Type content=\"text/html; charset=windows-1252\"><td> </td>",
			wantCharset:  "windows-1252",
			wantDeclared: true,
		},
		{
			name:        "no declaration falls back to windows-1252, reporting the assumption separately",
			in:          `<html><td>\xa0</td></html>`,
			wantText:    "<html><td> </td></html>",
			wantCharset: CharsetWindows1252,
			// The name must still compare equal to CharsetWindows1252 — decorating
			// it with "(assumed)" made that check silently false in exactly the
			// case a caller most wants to detect.
			wantDeclared: false,
		},
		{
			name:         "charset mentioned in body prose is NOT a declaration",
			in:           `<html><body>This file uses charset=shift_jis for its labels.<td>\xa0</td></body></html>`,
			wantText:     "<html><body>This file uses charset=shift_jis for its labels.<td> </td></body></html>",
			wantCharset:  CharsetWindows1252,
			wantDeclared: false,
		},
		{
			name:         "a declaration past the old 4096-byte scan limit is still found",
			in:           "<html><head>" + strings.Repeat("<!-- pad -->", 500) + `<meta charset=iso-8859-1></head><td>\xa0</td></html>`,
			wantText:     "<html><head>" + strings.Repeat("<!-- pad -->", 500) + "<meta charset=iso-8859-1></head><td> </td></html>",
			wantCharset:  "iso-8859-1",
			wantDeclared: true,
		},
		{
			name:         "a charset named only after </head> is not the declaration",
			in:           `<html><head><title>t</title></head><body>charset=shift_jis<td>\xa0</td></body></html>`,
			wantText:     "<html><head><title>t</title></head><body>charset=shift_jis<td> </td></body></html>",
			wantCharset:  CharsetWindows1252,
			wantDeclared: false,
		},
		{
			name:         "declared iso-8859-1 decodes the same way",
			in:           `<meta charset=iso-8859-1><td>\xa0</td>`,
			wantText:     "<meta charset=iso-8859-1><td> </td>",
			wantCharset:  "iso-8859-1",
			wantDeclared: true,
		},
		{
			name:         "declared utf-8 that is valid is passed through",
			in:           `<meta charset="utf-8"><td>ok</td>`,
			wantText:     `<meta charset="utf-8"><td>ok</td>`,
			wantCharset:  "utf-8",
			wantDeclared: true,
		},
		{
			name:    "declared utf-8 that is invalid is an error, not mojibake",
			in:      `<meta charset="utf-8"><td>\xa0</td>`,
			wantErr: ErrCharset,
		},
		{
			name:    "an unsupported charset is an error rather than a guess",
			in:      `<meta charset="shift_jis"><td>x</td>`,
			wantErr: ErrCharset,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(strings.ReplaceAll(tc.in, `\xa0`, "\xa0"))
			text, charset, declared, err := Decode(in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
			if charset != tc.wantCharset {
				t.Errorf("charset = %q, want %q", charset, tc.wantCharset)
			}
			if declared != tc.wantDeclared {
				t.Errorf("declared = %v, want %v", declared, tc.wantDeclared)
			}
		})
	}
}

// TestDecodeWindows1252HighBytes covers the 0x80..0x9F range that is the only
// real difference between windows-1252 and ISO-8859-1. The sampled workbooks
// contain none of these bytes, but a future one might.
func TestDecodeWindows1252HighBytes(t *testing.T) {
	tests := []struct {
		in   byte
		want string
	}{
		{0x80, "€"}, // euro sign
		{0x92, "’"}, // right single quote — Excel's apostrophe
		{0x96, "–"}, // en dash
		{0xa0, " "}, // NBSP, the only high byte these files actually use
		{0xe9, "é"}, // Latin-1 range passes through
		{0x41, "A"},
	}
	for _, tc := range tests {
		if got := DecodeWindows1252([]byte{tc.in}); got != tc.want {
			t.Errorf("DecodeWindows1252(%#x) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNBSPBlankIsNeverZero pins the blank-vs-zero trap, and pins it to the
// step where it actually lives.
//
// An earlier version of this test asserted that NBSP "survives"
// strings.TrimSpace. That is FALSE: unicode.IsSpace covers U+00A0, so
// TrimSpace strips a correctly decoded NBSP, and the assertion sat inside an
// `if` that could never fire — a test that documented a belief instead of
// measuring behaviour.
//
// The real trap is one step earlier. 0xA0 is not valid UTF-8, so a file read
// WITHOUT windows-1252 decoding yields U+FFFD, which is not space-classed and
// therefore survives every trim in the standard library. Decoding is what
// makes the blank recognisable; this test asserts both halves so neither can
// regress into the other.
func TestNBSPBlankIsNeverZero(t *testing.T) {
	const decoded = "\u00a0"
	undecoded := string([]byte{0xA0})

	// Half one: once decoded, U+00A0 IS space-classed.
	if !unicode.IsSpace('\u00a0') {
		t.Error("unicode.IsSpace(U+00A0) = false; this test's premise about Go has changed")
	}
	if got := strings.TrimSpace(decoded); got != "" {
		t.Errorf("strings.TrimSpace(decoded NBSP) = %q, want \"\"", got)
	}

	// Half two: the undecoded byte is NOT, and that is the actual hazard.
	if unicode.IsSpace('\uFFFD') {
		t.Error("unicode.IsSpace(U+FFFD) = true; the decode hazard described here would not apply")
	}
	if got := strings.TrimSpace(undecoded); got == "" {
		t.Error("strings.TrimSpace(raw 0xA0) emptied the string; the decode-first requirement would be unnecessary")
	} else if got != "\xa0" {
		t.Errorf("strings.TrimSpace(raw 0xA0) = %q, want %q", got, "\xa0")
	}

	// And the parser's answer, which is what actually protects the data: an
	// NBSP-only cell is an ABSENT measurement, never a zero.
	v := ParseValue(decoded)
	if v.State() != StateNotReported {
		t.Errorf("ParseValue(NBSP).State() = %v, want %v", v.State(), StateNotReported)
	}
	if _, ok := v.Float64(); ok {
		t.Error("an NBSP cell must not yield a number")
	}
	if v.Present() {
		t.Error("an NBSP cell must not be Present")
	}
	// A measured zero is the thing it must stay distinguishable from.
	z := ParseValue("0.00")
	if !z.Present() {
		t.Error(`ParseValue("0.00") must be Present: a measured zero is data`)
	}
	if mw, ok := z.Float64(); !ok || mw != 0 {
		t.Errorf(`ParseValue("0.00").Float64() = (%v,%v), want (0,true)`, mw, ok)
	}
	if v.State() == z.State() {
		t.Error("an NBSP blank and a measured 0.00 report the same state")
	}
}
