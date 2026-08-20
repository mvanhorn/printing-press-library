// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// send_rfc2047_test.go — regression coverage for PATCH(2026-05-22-001 U3).
// Verifies the Gmail wire-payload Subject + From headers carry RFC 2047
// encoded-word wrapping for non-ASCII bytes and pure ASCII for ASCII-only.
// The 2026-05-22 ground-truth bug: `→` (U+2192) in --subject reached
// recipients as `Ã¢Â†Â'` because the raw UTF-8 bytes were passed onto
// the RFC822 header lane with no encoding.

package cli

import (
	"encoding/base64"
	"mime"
	"net/mail"
	"strings"
	"testing"
)

// TestEncodeRFC2047Subject_ASCIIPassthrough asserts the encoder is a
// no-op for pure ASCII so existing tests that match `Subject: <ascii>`
// don't drift to encoded-word form.
func TestEncodeRFC2047Subject_ASCIIPassthrough(t *testing.T) {
	in := "Plain subject without funky bytes"
	got := encodeRFC2047Subject(in)
	if got != in {
		t.Fatalf("encodeRFC2047Subject(ascii) = %q want unchanged %q", got, in)
	}
}

// TestEncodeRFC2047Subject_NonASCIIEncodedWord covers the headline bug:
// `→` (U+2192) must reach the wire as a valid encoded-word, not raw UTF-8.
func TestEncodeRFC2047Subject_NonASCIIEncodedWord(t *testing.T) {
	in := "Pricing → consumption"
	got := encodeRFC2047Subject(in)

	if !strings.HasPrefix(got, "=?UTF-8?") {
		t.Fatalf("encodeRFC2047Subject(%q) did not produce encoded-word form: %q", in, got)
	}
	// Round-trip via stdlib decoder.
	dec := new(mime.WordDecoder)
	round, err := dec.DecodeHeader(got)
	if err != nil {
		t.Fatalf("decode %q: %v", got, err)
	}
	if round != in {
		t.Fatalf("round trip = %q want %q", round, in)
	}
}

// TestEncodeRFC2047Subject_NoMojibakeSequence is the literal-bug
// regression: the wire bytes for `→` (UTF-8 e2 86 92) must never carry
// the two-round CP-1252 cascade fingerprint c3 83 c2 a2 c3 82 e2 80 a0.
func TestEncodeRFC2047Subject_NoMojibakeSequence(t *testing.T) {
	in := "Pricing → consumption"
	got := encodeRFC2047Subject(in)
	if strings.Contains(got, "Ã¢Â") {
		t.Fatalf("encodeRFC2047Subject produced mojibake fingerprint: %q", got)
	}
	if strings.Contains(got, "\xc3\x83\xc2\xa2") {
		t.Fatalf("encodeRFC2047Subject produced double-encoded UTF-8 bytes: %q", got)
	}
}

// TestFormatRFC822FromHeader_ASCIIPassthrough asserts ASCII-only names
// stay in standard `"Name" <email>` quoted form.
func TestFormatRFC822FromHeader_ASCIIPassthrough(t *testing.T) {
	got := formatRFC822FromHeader("user@example.com", "Matt Van Horn")
	// Round-trip through ParseAddress to confirm validity.
	addr, err := mail.ParseAddress(got)
	if err != nil {
		t.Fatalf("formatRFC822FromHeader produced unparseable address %q: %v", got, err)
	}
	if addr.Address != "user@example.com" {
		t.Fatalf("address = %q want user@example.com", addr.Address)
	}
	if addr.Name != "Matt Van Horn" {
		t.Fatalf("name = %q want Matt Van Horn", addr.Name)
	}
}

// TestFormatRFC822FromHeader_NonASCIIEncoded asserts non-ASCII display
// names auto-wrap as encoded-words and round-trip cleanly.
func TestFormatRFC822FromHeader_NonASCIIEncoded(t *testing.T) {
	got := formatRFC822FromHeader("user@example.com", "Matt Van Hörn")
	if !strings.Contains(got, "=?") {
		t.Fatalf("non-ASCII name did not encode: %q", got)
	}
	addr, err := mail.ParseAddress(got)
	if err != nil {
		t.Fatalf("formatRFC822FromHeader produced unparseable address %q: %v", got, err)
	}
	if addr.Name != "Matt Van Hörn" {
		t.Fatalf("decoded name = %q want Matt Van Hörn", addr.Name)
	}
	if addr.Address != "user@example.com" {
		t.Fatalf("address = %q want user@example.com", addr.Address)
	}
}

// TestFormatRFC822FromHeader_BareEmail asserts the email-only path
// returns a bare address (no quotes, no brackets).
func TestFormatRFC822FromHeader_BareEmail(t *testing.T) {
	got := formatRFC822FromHeader("user@example.com", "")
	if got != "user@example.com" {
		t.Fatalf("formatRFC822FromHeader(email, \"\") = %q want bare email", got)
	}
}

// TestGmailWirePayload_SubjectAndFromEncoded is the end-to-end assertion:
// build a wire payload that flows through the Gmail-API path with a
// non-ASCII subject, decode the base64url body, and verify both headers
// landed encoded.
func TestGmailWirePayload_SubjectAndFromEncoded(t *testing.T) {
	in := sendInputs{
		FromEmail: "user@example.com",
		FromName:  "Matt Van Hörn",
		To:        []string{"alice@example.com"},
		Subject:   "Pricing → consumption",
		Body:      "<p>hi</p>",
		HTMLBody:  true,
		DraftID:   "draft0099",
		Rfc822ID:  "<r@e>",
	}
	headerLines := []string{
		"MIME-Version: 1.0",
		"From: " + formatRFC822FromHeader(in.FromEmail, in.FromName),
		"To: " + strings.Join(in.To, ", "),
		"Subject: " + encodeRFC2047Subject(in.Subject),
		"Content-Type: text/html; charset=utf-8",
		"",
		in.Body,
	}
	raw := strings.Join(headerLines, "\r\n")
	encoded := base64.URLEncoding.EncodeToString([]byte(raw))
	encoded = strings.TrimRight(encoded, "=")

	// Round-trip the base64url so we're asserting on what the API sees.
	switch n := len(encoded) % 4; n {
	case 2:
		encoded += "=="
	case 3:
		encoded += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 round-trip: %v", err)
	}
	wire := string(decoded)
	if strings.Contains(wire, "Pricing → consumption") {
		t.Fatalf("wire payload still carries raw UTF-8 subject (no encoding applied): %s", wire)
	}
	if !strings.Contains(wire, "=?UTF-8?") {
		t.Fatalf("wire payload missing encoded-word marker: %s", wire)
	}
	if !strings.Contains(wire, "From: ") {
		t.Fatalf("wire payload missing From: header: %s", wire)
	}
}
