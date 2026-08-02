// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: RFC 2822 message construction for send/reply/forward/schedule.
package gmailmail

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Compose describes an outbound message before RFC 2822 encoding.
type Compose struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Text        string
	HTML        string
	Attachments []string // local file paths
	MessageID   string   // RFC 5322 Message-ID for this message; also an idempotency key
	InReplyTo   string   // Message-ID header value of the message being replied to
	References  string   // References header chain for threading
	Date        time.Time
}

// wrap76 hard-wraps a base64 string at 76 columns per RFC 2045.
func wrap76(s string) string {
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76])
		b.WriteString("\r\n")
		s = s[76:]
	}
	b.WriteString(s)
	return b.String()
}

func encodeHeaderWord(s string) string {
	return mime.QEncoding.Encode("UTF-8", s)
}

// headerCtl strips the characters that would terminate a header line. Reply
// and forward derive To/Cc/In-Reply-To/References from the *original* message,
// so these values are chosen by whoever sent that mail: an embedded CRLF would
// let them append headers (a silent Bcc, say) to messages the user sends.
var headerCtl = strings.NewReplacer("\r", "", "\n", "", "\x00", "")

func sanitizeHeaderValue(s string) string { return headerCtl.Replace(s) }

// sanitizeAddressList re-renders addresses through net/mail so display names
// are quoted and malformed input cannot smuggle header syntax. Entries that
// do not parse fall back to a control-stripped copy rather than being dropped,
// because Gmail accepts some forms net/mail rejects.
func sanitizeAddressList(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		a = sanitizeHeaderValue(a)
		if strings.TrimSpace(a) == "" {
			continue
		}
		if parsed, err := mail.ParseAddress(a); err == nil {
			out = append(out, parsed.String())
			continue
		}
		out = append(out, a)
	}
	return out
}

// sanitizeMessageIDList keeps only well-formed <...> tokens, which is all the
// In-Reply-To and References headers may contain.
func sanitizeMessageIDList(s string) string {
	var kept []string
	for _, tok := range strings.Fields(sanitizeHeaderValue(s)) {
		if strings.HasPrefix(tok, "<") && strings.HasSuffix(tok, ">") && len(tok) > 2 &&
			!strings.ContainsAny(tok[1:len(tok)-1], "<> ") {
			kept = append(kept, tok)
		}
	}
	return strings.Join(kept, " ")
}

// randomBoundary returns a per-message MIME boundary. A fixed literal could be
// reproduced inside a forwarded body or attachment filename, which would let a
// sender inject extra MIME parts into mail sent from the user's account.
func randomBoundary(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "-gmail-pp-cli-fallback"
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

// sanitizeFilename strips control characters so the name cannot break out of
// its MIME parameter or inject a boundary line.
func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, filepath.Base(name))
}

// BuildRFC2822 renders the message as RFC 2822 bytes ready for base64url
// encoding into the Gmail API's raw field. Bodies are base64 transfer-encoded
// so arbitrary UTF-8 survives every relay; attachments ride multipart/mixed.
func (c Compose) BuildRFC2822() ([]byte, error) {
	if len(c.To) == 0 && len(c.Cc) == 0 && len(c.Bcc) == 0 {
		return nil, fmt.Errorf("at least one recipient (to, cc, or bcc) is required")
	}
	if strings.TrimSpace(c.Text) == "" && strings.TrimSpace(c.HTML) == "" {
		return nil, fmt.Errorf("message body is empty; provide text or HTML content")
	}

	var b strings.Builder
	// Every value is control-stripped before it reaches the wire: reply and
	// forward source these from the original message, so a CRLF in a remote
	// header would otherwise append headers to mail the user sends.
	writeHeader := func(name, value string) {
		value = sanitizeHeaderValue(value)
		if strings.TrimSpace(value) != "" {
			b.WriteString(name + ": " + value + "\r\n")
		}
	}
	writeHeader("From", c.From)
	writeHeader("To", strings.Join(sanitizeAddressList(c.To), ", "))
	writeHeader("Cc", strings.Join(sanitizeAddressList(c.Cc), ", "))
	writeHeader("Bcc", strings.Join(sanitizeAddressList(c.Bcc), ", "))
	writeHeader("Subject", encodeHeaderWord(c.Subject))
	if !c.Date.IsZero() {
		writeHeader("Date", c.Date.Format(time.RFC1123Z))
	}
	writeHeader("In-Reply-To", sanitizeMessageIDList(c.InReplyTo))
	writeHeader("References", sanitizeMessageIDList(c.References))
	writeHeader("Message-ID", sanitizeMessageIDList(c.MessageID))
	writeHeader("MIME-Version", "1.0")

	bodyPart := func(mimeType, content string) string {
		enc := base64.StdEncoding.EncodeToString([]byte(content))
		return "Content-Type: " + mimeType + "; charset=\"UTF-8\"\r\n" +
			"Content-Transfer-Encoding: base64\r\n\r\n" +
			wrap76(enc) + "\r\n"
	}

	// Inner body: single part, or multipart/alternative when both forms exist.
	var inner string
	switch {
	case c.Text != "" && c.HTML != "":
		altBoundary := randomBoundary("alt")
		inner = "Content-Type: multipart/alternative; boundary=\"" + altBoundary + "\"\r\n\r\n" +
			"--" + altBoundary + "\r\n" + bodyPart("text/plain", c.Text) +
			"--" + altBoundary + "\r\n" + bodyPart("text/html", c.HTML) +
			"--" + altBoundary + "--\r\n"
	case c.HTML != "":
		inner = bodyPart("text/html", c.HTML)
	default:
		inner = bodyPart("text/plain", c.Text)
	}

	if len(c.Attachments) == 0 {
		b.WriteString(inner)
		return []byte(b.String()), nil
	}

	mixedBoundary := randomBoundary("mixed")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"" + mixedBoundary + "\"\r\n\r\n")
	b.WriteString("--" + mixedBoundary + "\r\n" + inner)
	for _, path := range c.Attachments {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading attachment %s: %w", path, err)
		}
		// Forwarded attachment names come from the remote message, so encode
		// the MIME parameters instead of interpolating them: a name containing
		// a quote or a boundary line could otherwise inject a whole new part.
		name := sanitizeFilename(path)
		if name == "" {
			name = "attachment"
		}
		ct := mime.TypeByExtension(filepath.Ext(name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		ctHeader := mime.FormatMediaType(ct, map[string]string{"name": name})
		if ctHeader == "" {
			ctHeader = "application/octet-stream"
		}
		cdHeader := mime.FormatMediaType("attachment", map[string]string{"filename": name})
		if cdHeader == "" {
			cdHeader = "attachment"
		}
		b.WriteString("--" + mixedBoundary + "\r\n")
		b.WriteString("Content-Type: " + ctHeader + "\r\n")
		b.WriteString("Content-Disposition: " + cdHeader + "\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		b.WriteString(wrap76(base64.StdEncoding.EncodeToString(data)) + "\r\n")
	}
	b.WriteString("--" + mixedBoundary + "--\r\n")
	return []byte(b.String()), nil
}

// BuildRaw renders the message and returns Gmail's base64url raw form.
func (c Compose) BuildRaw() (string, error) {
	msg, err := c.BuildRFC2822()
	if err != nil {
		return "", err
	}
	return EncodeB64URL(msg), nil
}

// ReplySubject prefixes Re: exactly once.
func ReplySubject(subject string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(subject)), "re:") {
		return subject
	}
	return "Re: " + subject
}

// ForwardSubject prefixes Fwd: exactly once.
func ForwardSubject(subject string) string {
	low := strings.ToLower(strings.TrimSpace(subject))
	if strings.HasPrefix(low, "fwd:") || strings.HasPrefix(low, "fw:") {
		return subject
	}
	return "Fwd: " + subject
}

// QuoteOriginal renders the >-quoted body of the message being replied to.
func QuoteOriginal(fromHeader, dateHeader, body string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\r\n\r\nOn %s, %s wrote:\r\n", dateHeader, fromHeader))
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		b.WriteString("> " + line + "\r\n")
	}
	return b.String()
}

// ReferencesChain appends the replied-to Message-ID to its References header,
// producing the child's References value per RFC 5322 threading rules.
func ReferencesChain(parentReferences, parentMessageID string) string {
	parentMessageID = strings.TrimSpace(parentMessageID)
	if parentMessageID == "" {
		return strings.TrimSpace(parentReferences)
	}
	if strings.TrimSpace(parentReferences) == "" {
		return parentMessageID
	}
	return strings.TrimSpace(parentReferences) + " " + parentMessageID
}

// NewMessageID mints an RFC 5322 Message-ID. Setting it before a send makes
// the send verifiable after a crash: Gmail indexes the header, so
// `rfc822msgid:<id>` answers "did this actually go out?" without guessing.
func NewMessageID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("<%s.gmail-pp-cli@localhost>", hex.EncodeToString(b[:])), nil
}
