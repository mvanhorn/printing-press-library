// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Gmail message model, base64url codecs, MIME payload walking,
// and HTML-to-text conversion shared by read/send/pull/triage commands.
package gmailmail

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
)

// Header is one RFC 822 header as the Gmail API represents it.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PartBody is the body element of a MIME part.
type PartBody struct {
	AttachmentID string `json:"attachmentId,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Data         string `json:"data,omitempty"`
}

// Part is one node of the recursive MIME tree in messages.get format=full.
type Part struct {
	PartID   string   `json:"partId,omitempty"`
	MimeType string   `json:"mimeType,omitempty"`
	Filename string   `json:"filename,omitempty"`
	Headers  []Header `json:"headers,omitempty"`
	Body     PartBody `json:"body,omitempty"`
	Parts    []Part   `json:"parts,omitempty"`
}

// Message mirrors the Gmail API message resource for the fields this CLI uses.
type Message struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId,omitempty"`
	LabelIDs     []string `json:"labelIds,omitempty"`
	Snippet      string   `json:"snippet,omitempty"`
	HistoryID    string   `json:"historyId,omitempty"`
	InternalDate string   `json:"internalDate,omitempty"`
	SizeEstimate int64    `json:"sizeEstimate,omitempty"`
	Payload      *Part    `json:"payload,omitempty"`
	Raw          string   `json:"raw,omitempty"`
}

// AttachmentInfo describes one downloadable attachment discovered in a payload.
type AttachmentInfo struct {
	Filename     string `json:"filename"`
	MimeType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	AttachmentID string `json:"attachment_id"`
}

// Header returns the first header matching name, case-insensitively.
func (m *Message) Header(name string) string {
	if m == nil || m.Payload == nil {
		return ""
	}
	for _, h := range m.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// InternalTime converts the epoch-millisecond InternalDate to a time.Time.
func (m *Message) InternalTime() (time.Time, bool) {
	if m == nil || m.InternalDate == "" {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(m.InternalDate, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(ms).UTC(), true
}

// HasLabel reports whether the message carries the given label ID.
func (m *Message) HasLabel(id string) bool {
	for _, l := range m.LabelIDs {
		if l == id {
			return true
		}
	}
	return false
}

var addrRe = regexp.MustCompile(`<([^<>\s]+@[^<>\s]+)>`)

// ExtractEmail pulls the bare address out of a display-name form like
// `Jane Doe <jane@x.com>`. A bare address passes through lowercased.
func ExtractEmail(fromHeader string) string {
	s := strings.TrimSpace(fromHeader)
	if m := addrRe.FindStringSubmatch(s); m != nil {
		return strings.ToLower(m[1])
	}
	s = strings.Trim(s, `"' `)
	if strings.Contains(s, "@") && !strings.ContainsAny(s, " \t") {
		return strings.ToLower(s)
	}
	return strings.ToLower(s)
}

// DecodeB64URL decodes Gmail's base64url payloads, tolerating missing padding
// and the standard alphabet some proxies rewrite to.
func DecodeB64URL(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(strings.TrimRight(s, "=")); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// EncodeB64URL encodes bytes in the unpadded base64url form Gmail expects.
func EncodeB64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// ExtractBody walks the MIME tree and returns the concatenated text/plain and
// text/html bodies, decoded from base64url.
func ExtractBody(p *Part) (text string, html string) {
	if p == nil {
		return "", ""
	}
	var walk func(part *Part)
	walk = func(part *Part) {
		if part == nil {
			return
		}
		mt := strings.ToLower(part.MimeType)
		if part.Body.Data != "" && part.Filename == "" {
			if decoded, err := DecodeB64URL(part.Body.Data); err == nil {
				switch {
				case strings.HasPrefix(mt, "text/plain"):
					text += string(decoded)
				case strings.HasPrefix(mt, "text/html"):
					html += string(decoded)
				}
			}
		}
		for i := range part.Parts {
			walk(&part.Parts[i])
		}
	}
	walk(p)
	return text, html
}

// BodyText returns the best plain-text rendering of the message body:
// the text/plain part when present, otherwise HTML converted to text.
func (m *Message) BodyText() string {
	if m == nil {
		return ""
	}
	text, html := ExtractBody(m.Payload)
	if strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	if html != "" {
		return HTMLToText(html)
	}
	return ""
}

// Attachments scans the MIME tree for downloadable attachments.
func Attachments(p *Part) []AttachmentInfo {
	var out []AttachmentInfo
	if p == nil {
		return out
	}
	var walk func(part *Part)
	walk = func(part *Part) {
		if part == nil {
			return
		}
		if part.Filename != "" && (part.Body.AttachmentID != "" || part.Body.Data != "") {
			mt := part.MimeType
			if mt == "" {
				mt = "application/octet-stream"
			}
			out = append(out, AttachmentInfo{
				Filename:     part.Filename,
				MimeType:     mt,
				Size:         part.Body.Size,
				AttachmentID: part.Body.AttachmentID,
			})
		}
		for i := range part.Parts {
			walk(&part.Parts[i])
		}
	}
	walk(p)
	return out
}

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style|head)\b.*?</(script|style|head)>`)
	blockTagRe    = regexp.MustCompile(`(?i)<(/?)(p|div|br|li|tr|h[1-6]|blockquote|table|ul|ol)\b[^>]*>`)
	anyTagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	blankRunRe    = regexp.MustCompile(`\n{3,}`)
	spaceRunRe    = regexp.MustCompile(`[ \t]{2,}`)
)

// HTMLToText converts an HTML email body to readable plain text: block
// elements become newlines, tags are stripped, and entities are decoded.
// No competitor tool in the Gmail ecosystem does this conversion.
func HTMLToText(s string) string {
	s = scriptStyleRe.ReplaceAllString(s, " ")
	s = blockTagRe.ReplaceAllString(s, "\n")
	s = anyTagRe.ReplaceAllString(s, "")
	s = cliutil.CleanText(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(spaceRunRe.ReplaceAllString(l, " "))
	}
	s = strings.Join(lines, "\n")
	s = blankRunRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Flatten produces the JSON document pull stores in the local messages table:
// the native Gmail fields plus flattened convenience fields analytics can
// reach with json_extract, without walking payload.headers arrays in SQL.
func (m *Message) Flatten(bodyText string) (json.RawMessage, error) {
	doc := map[string]any{
		"id":           m.ID,
		"threadId":     m.ThreadID,
		"labelIds":     m.LabelIDs,
		"snippet":      m.Snippet,
		"historyId":    m.HistoryID,
		"internalDate": m.InternalDate,
		"sizeEstimate": m.SizeEstimate,
	}
	from := m.Header("From")
	doc["from"] = from
	doc["from_email"] = ExtractEmail(from)
	doc["to"] = m.Header("To")
	doc["subject"] = m.Header("Subject")
	doc["date"] = m.Header("Date")
	doc["message_id_header"] = m.Header("Message-ID")
	if lu := m.Header("List-Unsubscribe"); lu != "" {
		doc["list_unsubscribe"] = lu
	}
	if lid := m.Header("List-Id"); lid != "" {
		doc["list_id"] = lid
	}
	if t, ok := m.InternalTime(); ok {
		doc["internal_ts"] = t.Format(time.RFC3339)
	}
	doc["unread"] = m.HasLabel("UNREAD")
	if bodyText != "" {
		doc["body_text"] = bodyText
	}
	return json.Marshal(doc)
}
