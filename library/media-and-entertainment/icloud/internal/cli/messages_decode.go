// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(messages-attributedbody-heuristic): vendored heuristic typedstream
// parser for the attributedBody column of macOS chat.db.
//
// Algorithmic sources (all MIT-compatible for reference):
//   - teslashibe/imessage-go (MIT): the closest Go reference, particularly its
//     length-prefix decoding (1/2/4-byte little-endian variants).
//   - dgelessus/python-typedstream (MIT): the canonical algorithmic reference
//     for NSArchiver's NXTypedStream format.
//   - Chris Sardegna, "Reverse Engineering Apple's typedstream Format"
//     (https://chrissardegna.com/blog/reverse-engineering-apples-typedstream-format/)
//
// This is a heuristic. It targets the dominant case where a message body is
// an NSMutableAttributedString wrapping an NSString of UTF-8 text. It does not
// attempt full typedstream deserialization (which would require a full graph
// walk and Foundation class registry). Undecodable input returns "unrecoverable"
// rather than panicking; callers should fall back to message.text.

package cli

import (
	"bytes"
	"encoding/binary"
	"unicode/utf8"
)

// Text-source classification surfaced in JSON output so callers can tell
// whether a row's text was recovered from the typedstream blob, from the
// message.text column, or could not be decoded at all.
const (
	textSourceDecoded       = "decoded"
	textSourceTextColumn    = "text_column"
	textSourceUnrecoverable = "unrecoverable"
)

// typedstream type tag for a UTF-8 string primitive. The byte value is ASCII '+'.
const typedStreamStringTag = 0x2B

// Length-prefix marker bytes. Lengths < 0x81 are encoded directly as a single
// byte. 0x81/0x82/0x83 introduce 1, 2, or 4 little-endian length bytes
// respectively. See Sardegna's blog for the full encoding.
const (
	lengthPrefix1Byte = 0x81
	lengthPrefix2Byte = 0x82
	lengthPrefix4Byte = 0x83
)

// Maximum plausible message length. iMessage messages can be long but a single
// message body north of a megabyte is almost certainly a parse error, not a
// real message. Used as a sanity gate to avoid OOB reads on malformed input.
const maxMessageBytes = 1 << 20 // 1 MiB

// Window of bytes at the start of the blob to scan for the "streamtyped"
// marker. The canonical Apple typedstream prefix is `\x04\x0Bstreamtyped`
// (marker at offset 2); 20 bytes is generous slack for any future header
// padding without admitting random binary that happens to embed the string.
const typedStreamPrefixScanWindow = 20

// decodeAttributedBody extracts the human-readable text from a chat.db
// attributedBody blob (NSArchiver typedstream wrapping an NSMutableAttributedString).
//
// Returns (text, source) where source is one of:
//   - "decoded"        — recovered text from the typedstream
//   - "unrecoverable"  — blob was empty, malformed, or used an unhandled shape
//
// Never panics. Safe to call with nil or truncated input.
func decodeAttributedBody(blob []byte) (text string, source string) {
	if len(blob) == 0 {
		return "", textSourceUnrecoverable
	}

	// PATCH(messages-attributedbody-prefix-scan): the canonical NSArchiver
	// typedstream emitted by Apple begins with a 2-byte header `\x04\x0B`
	// (version + length-of-"streamtyped") followed by the literal ASCII
	// "streamtyped" — so the marker sits at offset 2, not offset 0. The
	// earlier strict HasPrefix check rejected every real chat.db row.
	// Scan the first ~20 bytes for the marker instead; the downstream
	// string-tag scan remains the authoritative validity gate.
	headWindow := blob
	if len(headWindow) > typedStreamPrefixScanWindow {
		headWindow = headWindow[:typedStreamPrefixScanWindow]
	}
	if !bytes.Contains(headWindow, []byte("streamtyped")) {
		return "", textSourceUnrecoverable
	}

	// Strategy: scan for the UTF-8 string tag (0x2B) and try each candidate
	// occurrence. The first one that successfully decodes into a UTF-8
	// string with a reasonable control-byte ratio wins. This is the same
	// shape used by teslashibe/imessage-go and timelinize's nsstring.go.
	for i := 0; i < len(blob); i++ {
		if blob[i] != typedStreamStringTag {
			continue
		}
		text, ok := readStringAfterTag(blob, i+1)
		if !ok {
			continue
		}
		if !looksLikeMessageText(text) {
			continue
		}
		return text, textSourceDecoded
	}

	return "", textSourceUnrecoverable
}

// readStringAfterTag reads a length-prefixed UTF-8 string starting at offset.
// Returns (text, ok). On any failure (truncated, length out of range,
// not valid UTF-8) returns ("", false).
func readStringAfterTag(blob []byte, offset int) (string, bool) {
	if offset >= len(blob) {
		return "", false
	}

	first := blob[offset]
	var length int
	var dataStart int

	switch {
	case first < lengthPrefix1Byte:
		// Direct single-byte length. Tag bytes used as lengths are not
		// plausible message lengths; require a non-zero length.
		if first == 0 {
			return "", false
		}
		length = int(first)
		dataStart = offset + 1

	case first == lengthPrefix1Byte:
		// One length byte follows.
		if offset+1 >= len(blob) {
			return "", false
		}
		length = int(blob[offset+1])
		dataStart = offset + 2

	case first == lengthPrefix2Byte:
		// Two length bytes follow, little-endian.
		if offset+2 >= len(blob) {
			return "", false
		}
		length = int(binary.LittleEndian.Uint16(blob[offset+1 : offset+3]))
		dataStart = offset + 3

	case first == lengthPrefix4Byte:
		// Four length bytes follow, little-endian.
		if offset+4 >= len(blob) {
			return "", false
		}
		length = int(binary.LittleEndian.Uint32(blob[offset+1 : offset+5]))
		dataStart = offset + 5

	default:
		return "", false
	}

	if length <= 0 || length > maxMessageBytes {
		return "", false
	}
	if dataStart+length > len(blob) {
		return "", false
	}

	candidate := blob[dataStart : dataStart+length]
	if !utf8.Valid(candidate) {
		return "", false
	}

	return string(candidate), true
}

// looksLikeMessageText rejects candidates that are obviously not human-readable
// text. The typedstream contains class names ("NSString", "NSMutableString",
// "NSMutableAttributedString"), keys, and other ASCII metadata that would
// otherwise pass utf8.Valid but is not the message body.
func looksLikeMessageText(s string) bool {
	if s == "" {
		return false
	}

	// Reject known Foundation class and key names that get embedded in the
	// typedstream alongside the actual message text.
	switch s {
	case
		"NSString",
		"NSMutableString",
		"NSAttributedString",
		"NSMutableAttributedString",
		"NSDictionary",
		"NSMutableDictionary",
		"NSArray",
		"NSMutableArray",
		"NSObject",
		"NSNumber",
		"NSValue",
		"NSData",
		"NSDate",
		"NSURL",
		"NSColor",
		"NSFont":
		return false
	}

	// Reject candidates with too many control characters. Genuine messages
	// (including emoji and CJK) have very few; metadata blobs often
	// contain runs of control characters. Threshold: 25%.
	//
	// PATCH(messages-control-byte-rune-denominator): count runes in both
	// numerator and denominator. Using len(s) (bytes) as the denominator
	// inflates with multi-byte runes (emoji, CJK) and slackens the threshold
	// below intent.
	control := 0
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			control++
		}
	}
	if runeCount := utf8.RuneCountInString(s); runeCount > 0 && control*4 > runeCount {
		return false
	}

	return true
}
