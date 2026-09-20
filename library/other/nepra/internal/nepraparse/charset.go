package nepraparse

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ErrCharset is returned when the workbook declares a character set this
// package cannot decode. It is deliberately an error rather than a
// best-effort guess: silently mis-decoding a byte is how plant names and
// sentinels turn into mojibake that later compares unequal.
var ErrCharset = errors.New("nepraparse: unsupported document charset")

// CharsetWindows1252 is the charset every sampled NEPRA generation workbook
// declares in-document. The HTTP response declares none.
const CharsetWindows1252 = "windows-1252"

// metaTagRE matches a whole <meta ...> tag. The charset token is only read
// from inside one of these, so that prose in the document body mentioning a
// charset — or a stylesheet's @charset rule — cannot be mistaken for the
// document's own declaration.
var metaTagRE = regexp.MustCompile(`(?is)<meta\s[^>]*>`)

// charsetAttrRE pulls the charset token out of a meta tag. It covers both
// forms: the HTML5 <meta charset=windows-1252> and Excel's
// <meta http-equiv=Content-Type content="text/html; charset=windows-1252">.
// Excel writes some attributes unquoted, so quoting is optional.
var charsetAttrRE = regexp.MustCompile(`(?i)charset\s*=\s*["']?([a-z0-9_.:-]+)`)

// charsetScanLimit caps how far into the document DeclaredCharset looks when
// the document has no </head>. The declaration belongs in <head> — measured
// at byte offset 244 in all three sampled workbooks — but a truncated or
// head-less document should not silently lose it, so the cap is generous
// rather than tight. Restricting matches to <meta> tags, not the cap, is what
// prevents a false positive.
const charsetScanLimit = 64 << 10

// DeclaredCharset returns the charset declared by a <meta> tag in the
// document's head, lowercased, or "" when the document declares none.
//
// The scan is byte-oriented and ASCII-only, so it is safe to run before any
// decoding. It stops at </head> when there is one, and otherwise at
// [charsetScanLimit].
func DeclaredCharset(data []byte) string {
	for _, tag := range metaTagRE.FindAll(headRegion(data), -1) {
		if m := charsetAttrRE.FindSubmatch(tag); m != nil {
			return strings.ToLower(string(m[1]))
		}
	}
	return ""
}

// headRegion is the byte range DeclaredCharset is allowed to look at.
func headRegion(data []byte) []byte {
	region := data
	if len(region) > charsetScanLimit {
		region = region[:charsetScanLimit]
	}
	if i := bytes.Index(bytes.ToLower(region), []byte("</head>")); i >= 0 {
		return region[:i]
	}
	return region
}

// cp1252High maps windows-1252 bytes 0x80..0x9F to their Unicode code points.
// Everything below 0x80 is ASCII and everything from 0xA0 up is Latin-1, so
// this 32-entry table is the entire difference between windows-1252 and
// ISO-8859-1. A value of 0xFFFD marks the five unassigned positions.
var cp1252High = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
}

// DecodeWindows1252 converts windows-1252 bytes to a UTF-8 string. Every byte
// has a defined mapping except five unassigned positions in 0x80..0x9F, which
// become U+FFFD; the sampled workbooks contain none of them (their only
// non-ASCII byte is 0xA0).
func DecodeWindows1252(data []byte) string {
	if isASCII(data) {
		return string(data)
	}
	var b strings.Builder
	b.Grow(len(data) + len(data)/8)
	for _, c := range data {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xA0:
			b.WriteRune(cp1252High[c-0x80])
		default:
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

func isASCII(data []byte) bool {
	for _, c := range data {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

// Decode turns raw workbook bytes into a UTF-8 string, honouring the
// in-document charset declaration. It returns the charset actually applied and
// whether the document declared it.
//
// windows-1252 is assumed when no charset is declared, because that is what
// Excel's "Save as Web Page" writes and what every sampled year declares.
// The assumption is reported through the separate `declared` return rather
// than by decorating the charset name, so that a caller comparing
// `charset == CharsetWindows1252` gets the right answer on BOTH paths. An
// earlier version returned "windows-1252 (assumed; document declared none)"
// here, which made that comparison silently false whenever the declaration
// was missing — the one case a caller most wants to detect.
//
// A declared utf-8 document is validated rather than trusted: invalid UTF-8
// is an error, not a stream of replacement characters.
func Decode(data []byte) (text string, charset string, declared bool, err error) {
	dc := DeclaredCharset(data)
	switch dc {
	case "", "windows-1252", "cp1252", "iso-8859-1", "latin1", "us-ascii", "ascii":
		applied := dc
		if applied == "" {
			applied = CharsetWindows1252
		}
		return DecodeWindows1252(data), applied, dc != "", nil
	case "utf-8", "utf8":
		if !utf8.Valid(data) {
			return "", dc, true, fmt.Errorf("%w: document declares utf-8 but the bytes are not valid utf-8", ErrCharset)
		}
		return string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})), dc, true, nil
	default:
		return "", dc, true, fmt.Errorf("%w: %q", ErrCharset, dc)
	}
}
