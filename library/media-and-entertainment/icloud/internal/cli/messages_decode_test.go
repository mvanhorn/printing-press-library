// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/binary"
	"strings"
	"testing"
)

// buildSimpleBlob constructs a minimal typedstream-prefixed blob containing a
// single UTF-8 string with the given length-prefix encoding. Used by tests
// to exercise the decoder against known-shape inputs without depending on
// captured chat.db rows.
//
// This shape places the "streamtyped" marker at offset 0, exercising the
// backward-compatible path of the prefix scan. For the canonical Apple
// format (marker at offset 2 behind a \x04\x0B version/length pair), see
// buildCanonicalBlob.
func buildSimpleBlob(text string, prefixVariant byte) []byte {
	var b []byte
	b = append(b, []byte("streamtyped")...)
	// Pad with a few non-tag bytes so the scanner has to skip past the header
	// before finding the string tag.
	b = append(b, 0x04, 0x0b, 0x06)
	b = append(b, typedStreamStringTag)

	textBytes := []byte(text)
	switch prefixVariant {
	case 0: // direct single-byte length (length < 0x81)
		b = append(b, byte(len(textBytes)))
	case lengthPrefix1Byte:
		b = append(b, lengthPrefix1Byte, byte(len(textBytes)))
	case lengthPrefix2Byte:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(len(textBytes)))
		b = append(b, lengthPrefix2Byte)
		b = append(b, buf...)
	case lengthPrefix4Byte:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(len(textBytes)))
		b = append(b, lengthPrefix4Byte)
		b = append(b, buf...)
	}
	b = append(b, textBytes...)
	return b
}

func TestDecodeAttributedBody_Empty(t *testing.T) {
	text, source := decodeAttributedBody(nil)
	if text != "" || source != textSourceUnrecoverable {
		t.Errorf("nil blob: got (%q, %q), want (\"\", %q)", text, source, textSourceUnrecoverable)
	}

	text, source = decodeAttributedBody([]byte{})
	if text != "" || source != textSourceUnrecoverable {
		t.Errorf("empty blob: got (%q, %q), want (\"\", %q)", text, source, textSourceUnrecoverable)
	}
}

func TestDecodeAttributedBody_NonTypedStream(t *testing.T) {
	blob := []byte("this is not a typedstream blob")
	text, source := decodeAttributedBody(blob)
	if text != "" || source != textSourceUnrecoverable {
		t.Errorf("non-typedstream prefix: got (%q, %q), want (\"\", %q)", text, source, textSourceUnrecoverable)
	}
}

func TestDecodeAttributedBody_ShortASCII(t *testing.T) {
	blob := buildSimpleBlob("hello world", 0)
	text, source := decodeAttributedBody(blob)
	if text != "hello world" || source != textSourceDecoded {
		t.Errorf("short ASCII: got (%q, %q), want (\"hello world\", %q)", text, source, textSourceDecoded)
	}
}

func TestDecodeAttributedBody_MultiByteUTF8(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"emoji", "🎉🎊👋"},
		{"accented", "café résumé"},
		{"cjk", "你好世界"},
		{"mixed", "hi 👋 from café 你好"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blob := buildSimpleBlob(tc.in, 0)
			text, source := decodeAttributedBody(blob)
			if text != tc.in || source != textSourceDecoded {
				t.Errorf("got (%q, %q), want (%q, %q)", text, source, tc.in, textSourceDecoded)
			}
		})
	}
}

func TestDecodeAttributedBody_Length1Byte(t *testing.T) {
	// Build a string long enough to require the 1-byte length prefix
	// (length 0x90 = 144, which is >= 0x81 so direct encoding can't be used).
	msg := strings.Repeat("a", 144)
	blob := buildSimpleBlob(msg, lengthPrefix1Byte)
	text, source := decodeAttributedBody(blob)
	if text != msg || source != textSourceDecoded {
		t.Errorf("1-byte length: got source=%q text-len=%d, want source=%q text-len=%d",
			source, len(text), textSourceDecoded, len(msg))
	}
}

func TestDecodeAttributedBody_Length2Byte(t *testing.T) {
	// Build a longer string that requires the 2-byte length prefix.
	msg := strings.Repeat("hello ", 100) // 600 bytes
	blob := buildSimpleBlob(msg, lengthPrefix2Byte)
	text, source := decodeAttributedBody(blob)
	if text != msg || source != textSourceDecoded {
		t.Errorf("2-byte length: got source=%q text-len=%d, want source=%q text-len=%d",
			source, len(text), textSourceDecoded, len(msg))
	}
}

func TestDecodeAttributedBody_Length4Byte(t *testing.T) {
	// Very large message requiring the 4-byte length prefix.
	msg := strings.Repeat("xyz ", 20000) // 80000 bytes
	blob := buildSimpleBlob(msg, lengthPrefix4Byte)
	text, source := decodeAttributedBody(blob)
	if text != msg || source != textSourceDecoded {
		t.Errorf("4-byte length: got source=%q text-len=%d, want source=%q text-len=%d",
			source, len(text), textSourceDecoded, len(msg))
	}
}

func TestDecodeAttributedBody_TruncatedMidLength(t *testing.T) {
	// Build a 2-byte-prefix blob then chop off in the middle of the length.
	blob := buildSimpleBlob("some message body", lengthPrefix2Byte)
	// Find the prefix marker and truncate to immediately after it.
	idx := -1
	for i := 0; i < len(blob)-1; i++ {
		if blob[i] == typedStreamStringTag && blob[i+1] == lengthPrefix2Byte {
			idx = i + 2 // keep tag + marker, drop length bytes
			break
		}
	}
	if idx < 0 {
		t.Fatal("test setup error: prefix marker not found")
	}
	truncated := blob[:idx]
	text, source := decodeAttributedBody(truncated)
	// We don't require unrecoverable specifically — the scanner may find an
	// earlier 0x2B byte and try to decode there too. The contract is just
	// "don't crash and don't return obviously wrong content".
	if source == textSourceDecoded && text == "some message body" {
		t.Errorf("truncated blob should not have decoded full text, got (%q, %q)", text, source)
	}
}

func TestDecodeAttributedBody_LengthExceedsBuffer(t *testing.T) {
	// Construct a blob whose claimed length is larger than remaining bytes.
	var blob []byte
	blob = append(blob, []byte("streamtyped")...)
	blob = append(blob, 0x04, 0x0b)
	blob = append(blob, typedStreamStringTag)
	blob = append(blob, lengthPrefix2Byte)
	// Claim 10000 bytes but only provide 5.
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, 10000)
	blob = append(blob, buf...)
	blob = append(blob, []byte("short")...)

	text, source := decodeAttributedBody(blob)
	if source == textSourceDecoded {
		t.Errorf("over-length claim should not decode, got (%q, %q)", text, source)
	}
}

func TestDecodeAttributedBody_InvalidUTF8(t *testing.T) {
	// Build a blob with a valid prefix but invalid UTF-8 bytes.
	var blob []byte
	blob = append(blob, []byte("streamtyped")...)
	blob = append(blob, 0x04, 0x0b)
	blob = append(blob, typedStreamStringTag)
	blob = append(blob, 0x05)                         // direct length 5
	blob = append(blob, 0xff, 0xfe, 0xfd, 0xfc, 0xfb) // invalid UTF-8
	text, source := decodeAttributedBody(blob)
	if source == textSourceDecoded {
		t.Errorf("invalid UTF-8 should not decode, got (%q, %q)", text, source)
	}
}

func TestDecodeAttributedBody_FiltersClassName(t *testing.T) {
	// "NSMutableAttributedString" is a known class name that should be
	// filtered out by looksLikeMessageText. Build a blob that has it
	// as the first decodable candidate and ensure the decoder rejects it
	// (or skips past it to find no further valid string).
	blob := buildSimpleBlob("NSMutableAttributedString", 0)
	text, source := decodeAttributedBody(blob)
	if source == textSourceDecoded {
		t.Errorf("class name should be filtered, got (%q, %q)", text, source)
	}
}

func TestDecodeAttributedBody_LooksLikeMessageText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"normal text", "hey are you free for lunch", true},
		{"with emoji", "yes! 🎉", true},
		{"empty", "", false},
		{"class name NSString", "NSString", false},
		{"class name NSMutableAttributedString", "NSMutableAttributedString", false},
		{"control chars dominant", "\x01\x02\x03\x04", false},
		{"low control byte ratio", "hello\nworld\tfine", true},
		// Rune-denominator regression coverage (PATCH messages-control-byte-rune-denominator):
		// a short CJK string with a single control byte must be rejected on rune
		// count, not on byte length where multi-byte runes hid the ratio.
		{"cjk with one control", "你好\x01", false},
		// All-emoji text with no control bytes must still pass.
		{"emoji only", "🎉🎊👋🚀", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikeMessageText(tc.in)
			if got != tc.want {
				t.Errorf("looksLikeMessageText(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// buildCanonicalBlob constructs a typedstream-prefixed blob in the canonical
// Apple NSArchiver shape: a leading 2-byte version+length header (`\x04\x0B`)
// followed by the literal "streamtyped" marker, then a representative
// NSAttributedString → NSObject → NSString class chain (synthesized from the
// public format spec, not captured from any chat.db), then the string tag
// and length-prefixed UTF-8 text. Used to exercise the prefix-scan code path
// that real chat.db rows hit in production.
func buildCanonicalBlob(text string, prefixVariant byte) []byte {
	var b []byte
	// Canonical Apple typedstream header: version 4, length of "streamtyped" (11),
	// then the marker.
	b = append(b, 0x04, 0x0b)
	b = append(b, []byte("streamtyped")...)
	// System version + a short, synthetic class-chain pattern matching the
	// documented NSArchiver structure for NSAttributedString. Padding bytes
	// here are arbitrary non-tag values; the decoder's tag scan navigates
	// past them to find the 0x2B string tag.
	b = append(b, 0x81, 0xe8, 0x03, 0x84, 0x01, 0x40, 0x84, 0x84, 0x84, 0x12)
	b = append(b, []byte("NSAttributedString")...)
	b = append(b, 0x00, 0x84, 0x84, 0x08)
	b = append(b, []byte("NSObject")...)
	b = append(b, 0x00, 0x85, 0x92, 0x84, 0x84, 0x84, 0x08)
	b = append(b, []byte("NSString")...)
	b = append(b, 0x01, 0x94, 0x84, 0x01)
	b = append(b, typedStreamStringTag)

	textBytes := []byte(text)
	switch prefixVariant {
	case 0: // direct single-byte length (length < 0x81)
		b = append(b, byte(len(textBytes)))
	case lengthPrefix1Byte:
		b = append(b, lengthPrefix1Byte, byte(len(textBytes)))
	case lengthPrefix2Byte:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(len(textBytes)))
		b = append(b, lengthPrefix2Byte)
		b = append(b, buf...)
	case lengthPrefix4Byte:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(len(textBytes)))
		b = append(b, lengthPrefix4Byte)
		b = append(b, buf...)
	}
	b = append(b, textBytes...)
	return b
}

func TestDecodeAttributedBody_CanonicalFormat(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		variant byte
	}{
		{"direct length ASCII", "hello canonical", 0},
		{"direct length emoji", "ship it 🚀", 0},
		{"1-byte length", strings.Repeat("a", 200), lengthPrefix1Byte},
		{"2-byte length", strings.Repeat("hi ", 500), lengthPrefix2Byte},
		{"4-byte length", strings.Repeat("xyz ", 20000), lengthPrefix4Byte},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blob := buildCanonicalBlob(tc.text, tc.variant)
			text, source := decodeAttributedBody(blob)
			if text != tc.text || source != textSourceDecoded {
				t.Errorf("canonical %s: got source=%q text-len=%d, want source=%q text-len=%d",
					tc.name, source, len(text), textSourceDecoded, len(tc.text))
			}
		})
	}
}

func TestDecodeAttributedBody_MarkerOutsideScanWindow(t *testing.T) {
	// Place the "streamtyped" marker at offset 30 — past the prefix-scan
	// window. Decoder must not pick it up.
	var blob []byte
	blob = append(blob, make([]byte, 30)...)
	blob = append(blob, []byte("streamtyped")...)
	blob = append(blob, 0x04, 0x0b, 0x06)
	blob = append(blob, typedStreamStringTag)
	blob = append(blob, 0x05)
	blob = append(blob, []byte("hello")...)

	text, source := decodeAttributedBody(blob)
	if source == textSourceDecoded {
		t.Errorf("marker outside scan window should not decode, got (%q, %q)", text, source)
	}
}

func TestDecodeAttributedBody_BackwardCompatBareMarker(t *testing.T) {
	// Blobs constructed with the bare-"streamtyped" prefix (no leading
	// \x04\x0B) must still decode under the permissive scan. Locks the
	// backward-compat case so a future refactor doesn't quietly tighten
	// the prefix check.
	blob := buildSimpleBlob("backward compat", 0)
	text, source := decodeAttributedBody(blob)
	if text != "backward compat" || source != textSourceDecoded {
		t.Errorf("bare-marker blob: got (%q, %q), want (\"backward compat\", %q)", text, source, textSourceDecoded)
	}
}
