// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
package gmailmail

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRFC2822PlainText(t *testing.T) {
	c := Compose{
		To:      []string{"dest@example.com"},
		Subject: "Hello",
		Text:    "line one\nline two",
	}
	raw, err := c.BuildRFC2822()
	if err != nil {
		t.Fatalf("BuildRFC2822 error = %v", err)
	}
	s := string(raw)
	// net/mail canonicalizes a bare address into angle-bracket form.
	if !strings.Contains(s, "To: <dest@example.com>\r\n") {
		t.Fatalf("missing To header: %s", s)
	}
	if !strings.Contains(s, "Subject: Hello\r\n") {
		t.Fatalf("missing Subject header: %s", s)
	}
	if !strings.Contains(s, "Content-Type: text/plain") {
		t.Fatalf("missing text/plain content type: %s", s)
	}
	// Body is base64 transfer-encoded; verify round-trip.
	idx := strings.LastIndex(s, "\r\n\r\n")
	body := strings.ReplaceAll(s[idx+4:], "\r\n", "")
	decoded, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("body is not valid base64: %v", err)
	}
	if string(decoded) != "line one\nline two" {
		t.Fatalf("body round-trip = %q", decoded)
	}
}

func TestBuildRFC2822RequiresRecipientAndBody(t *testing.T) {
	if _, err := (Compose{Subject: "x", Text: "y"}).BuildRFC2822(); err == nil {
		t.Fatal("expected error with no recipients")
	}
	if _, err := (Compose{To: []string{"a@b.c"}}).BuildRFC2822(); err == nil {
		t.Fatal("expected error with empty body")
	}
}

func TestBuildRFC2822ThreadingHeaders(t *testing.T) {
	c := Compose{
		To:         []string{"dest@example.com"},
		Subject:    "Re: Hello",
		Text:       "reply",
		InReplyTo:  "<orig@mail.gmail.com>",
		References: "<root@mail.gmail.com> <orig@mail.gmail.com>",
	}
	raw, err := c.BuildRFC2822()
	if err != nil {
		t.Fatalf("BuildRFC2822 error = %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "In-Reply-To: <orig@mail.gmail.com>\r\n") {
		t.Fatalf("missing In-Reply-To: %s", s)
	}
	if !strings.Contains(s, "References: <root@mail.gmail.com> <orig@mail.gmail.com>\r\n") {
		t.Fatalf("missing References: %s", s)
	}
}

func TestBuildRFC2822WithAttachment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("attachment payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := Compose{
		To:          []string{"dest@example.com"},
		Subject:     "With file",
		Text:        "see attached",
		Attachments: []string{path},
	}
	raw, err := c.BuildRFC2822()
	if err != nil {
		t.Fatalf("BuildRFC2822 error = %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "multipart/mixed") {
		t.Fatalf("missing multipart/mixed: %s", s)
	}
	// mime.FormatMediaType omits quotes when the value needs none.
	if !strings.Contains(s, "filename=note.txt") && !strings.Contains(s, `filename="note.txt"`) {
		t.Fatalf("missing attachment disposition: %s", s)
	}
	wantB64 := base64.StdEncoding.EncodeToString([]byte("attachment payload"))
	if !strings.Contains(strings.ReplaceAll(s, "\r\n", ""), wantB64) {
		t.Fatalf("missing attachment payload")
	}
}

func TestBuildRawIsBase64URL(t *testing.T) {
	c := Compose{To: []string{"a@b.c"}, Subject: "s", Text: "body"}
	raw, err := c.BuildRaw()
	if err != nil {
		t.Fatalf("BuildRaw error = %v", err)
	}
	if strings.ContainsAny(raw, "+/=") {
		t.Fatalf("BuildRaw output not base64url: %q", raw[:40])
	}
	if _, err := DecodeB64URL(raw); err != nil {
		t.Fatalf("BuildRaw output does not round-trip: %v", err)
	}
}

func TestSubjectPrefixes(t *testing.T) {
	if got := ReplySubject("Hello"); got != "Re: Hello" {
		t.Fatalf("ReplySubject = %q", got)
	}
	if got := ReplySubject("Re: Hello"); got != "Re: Hello" {
		t.Fatalf("ReplySubject double-prefixed: %q", got)
	}
	if got := ForwardSubject("Fwd: Hello"); got != "Fwd: Hello" {
		t.Fatalf("ForwardSubject double-prefixed: %q", got)
	}
	if got := ForwardSubject("Hello"); got != "Fwd: Hello" {
		t.Fatalf("ForwardSubject = %q", got)
	}
}

func TestReferencesChain(t *testing.T) {
	if got := ReferencesChain("", "<a@x>"); got != "<a@x>" {
		t.Fatalf("ReferencesChain empty parent = %q", got)
	}
	if got := ReferencesChain("<root@x>", "<a@x>"); got != "<root@x> <a@x>" {
		t.Fatalf("ReferencesChain = %q", got)
	}
}

func TestQuoteOriginal(t *testing.T) {
	q := QuoteOriginal("Jane <j@example.com>", "Mon, 1 Jan 2026", "hello\nworld")
	if !strings.Contains(q, "> hello\r\n> world") {
		t.Fatalf("QuoteOriginal = %q", q)
	}
}

// countHeader counts header lines of the given name at the start of a line.
func countHeader(msg, name string) int {
	n := 0
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(line, name+":") {
			n++
		}
	}
	return n
}

// Reply and forward take To/Cc/In-Reply-To/References from the *original*
// message, so those values are chosen by whoever sent that mail. A CRLF in one
// of them must never be able to append a header to what the user sends.
func TestBuildRFC2822RejectsHeaderInjection(t *testing.T) {
	t.Run("recipient CRLF cannot add Bcc", func(t *testing.T) {
		c := Compose{
			To:      []string{"good@example.com\r\nBcc: attacker@evil.com"},
			Subject: "hi",
			Text:    "body",
		}
		raw, err := c.BuildRFC2822()
		if err != nil {
			t.Fatalf("BuildRFC2822 error = %v", err)
		}
		s := string(raw)
		if strings.Contains(s, "attacker@evil.com") && countHeader(s, "Bcc") > 0 {
			t.Fatalf("header injection succeeded:\n%s", s)
		}
		if countHeader(s, "To") != 1 {
			t.Fatalf("expected exactly one To header, got %d:\n%s", countHeader(s, "To"), s)
		}
	})

	t.Run("subject CRLF does not split headers", func(t *testing.T) {
		c := Compose{
			To:      []string{"a@b.com"},
			Subject: "hi\r\nX-Injected: yes",
			Text:    "body",
		}
		raw, err := c.BuildRFC2822()
		if err != nil {
			t.Fatalf("BuildRFC2822 error = %v", err)
		}
		if countHeader(string(raw), "X-Injected") > 0 {
			t.Fatalf("subject injection succeeded:\n%s", raw)
		}
	})

	t.Run("References keeps only well-formed message ids", func(t *testing.T) {
		c := Compose{
			To:         []string{"a@b.com"},
			Subject:    "s",
			Text:       "body",
			InReplyTo:  "<ok@mail>\r\nBcc: evil@example.com",
			References: "<root@mail> garbage-token <second@mail>",
		}
		raw, err := c.BuildRFC2822()
		if err != nil {
			t.Fatalf("BuildRFC2822 error = %v", err)
		}
		s := string(raw)
		if countHeader(s, "Bcc") > 0 {
			t.Fatalf("In-Reply-To injection succeeded:\n%s", s)
		}
		if !strings.Contains(s, "References: <root@mail> <second@mail>\r\n") {
			t.Fatalf("References not normalized:\n%s", s)
		}
	})
}

// Forwarded attachment names come from the remote message.
func TestBuildRFC2822HostileAttachmentName(t *testing.T) {
	dir := t.TempDir()
	hostile := `evil".pdf`
	path := filepath.Join(dir, hostile)
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Skipf("filesystem rejects the hostile name: %v", err)
	}
	c := Compose{
		To:          []string{"a@b.com"},
		Subject:     "s",
		Text:        "body",
		Attachments: []string{path},
	}
	raw, err := c.BuildRFC2822()
	if err != nil {
		t.Fatalf("BuildRFC2822 error = %v", err)
	}
	s := string(raw)
	// Exactly one body part and one attachment part: the name must not have
	// introduced an extra boundary or a second Content-Disposition.
	if got := strings.Count(s, "Content-Disposition:"); got != 1 {
		t.Fatalf("expected 1 Content-Disposition, got %d:\n%s", got, s)
	}
	if got := strings.Count(s, "Content-Transfer-Encoding:"); got != 2 {
		t.Fatalf("expected 2 parts, got %d encodings:\n%s", got, s)
	}
}

// A fixed boundary could be reproduced inside a forwarded body.
func TestBuildRFC2822BoundariesAreUnpredictable(t *testing.T) {
	mk := func() string {
		c := Compose{To: []string{"a@b.com"}, Subject: "s", Text: "t", HTML: "<p>t</p>"}
		raw, err := c.BuildRFC2822()
		if err != nil {
			t.Fatalf("BuildRFC2822 error = %v", err)
		}
		return string(raw)
	}
	if mk() == mk() {
		t.Fatal("multipart boundary is identical across messages; it must be random")
	}
}
