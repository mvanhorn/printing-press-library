package cli

import (
	"encoding/json"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

// gmailMsgData is a partial decode of the Gmail API message JSON stored in the
// messages.data column. Only fields used by novel commands are included.
type gmailMsgData struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId"`
	LabelIDs     []string `json:"labelIds"`
	Snippet      string   `json:"snippet"`
	InternalDate string   `json:"internalDate"`
	SizeEstimate int      `json:"sizeEstimate"`
	HistoryID    string   `json:"historyId"`
	Payload      struct {
		Headers []gmailHeader `json:"headers"`
		Parts   []gmailPart   `json:"parts"`
	} `json:"payload"`
}

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type gmailPart struct {
	PartID   string        `json:"partId"`
	MimeType string        `json:"mimeType"`
	Filename string        `json:"filename"`
	Body     gmailPartBody `json:"body"`
	Parts    []gmailPart   `json:"parts"`
}

type gmailPartBody struct {
	AttachmentID string `json:"attachmentId"`
	Size         int    `json:"size"`
	Data         string `json:"data"`
}

// header returns the first value for a case-insensitive header name.
func (m *gmailMsgData) header(name string) string {
	lower := strings.ToLower(name)
	for _, h := range m.Payload.Headers {
		if strings.ToLower(h.Name) == lower {
			return h.Value
		}
	}
	return ""
}

// internalTime converts the Gmail internalDate (Unix ms string) to a time.Time.
func (m *gmailMsgData) internalTime() time.Time {
	if m.InternalDate == "" {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(m.InternalDate, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

// hasLabel reports whether the message has a given Gmail label ID (case-sensitive).
func (m *gmailMsgData) hasLabel(label string) bool {
	for _, l := range m.LabelIDs {
		if l == label {
			return true
		}
	}
	return false
}

// parseGmailMsg unmarshals a raw data column value into gmailMsgData.
func parseGmailMsg(dataJSON string) (*gmailMsgData, error) {
	var msg gmailMsgData
	if err := json.Unmarshal([]byte(dataJSON), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// allParts returns a flat list of all leaf parts in a message (recursive).
func allParts(parts []gmailPart) []gmailPart {
	var out []gmailPart
	for _, p := range parts {
		if len(p.Parts) > 0 {
			out = append(out, allParts(p.Parts)...)
		} else {
			out = append(out, p)
		}
	}
	return out
}

// extractEmailDomain pulls the domain from an RFC 5322 address string.
// Returns empty string when parsing fails.
func extractEmailDomain(addr string) string {
	a, err := mail.ParseAddress(addr)
	if err != nil {
		// fall back to simple split
		if i := strings.LastIndex(addr, "@"); i >= 0 {
			return strings.ToLower(strings.TrimSpace(addr[i+1:]))
		}
		return ""
	}
	if i := strings.LastIndex(a.Address, "@"); i >= 0 {
		return strings.ToLower(a.Address[i+1:])
	}
	return ""
}

// normalizeFrom returns a canonical "Name <email>" or just "email" for display.
func normalizeFrom(from string) string {
	a, err := mail.ParseAddress(from)
	if err != nil {
		return strings.TrimSpace(from)
	}
	if a.Name != "" {
		return a.Name + " <" + a.Address + ">"
	}
	return a.Address
}

// ageBucket classifies a message age into one of five display buckets.
func ageBucket(t time.Time) string {
	since := time.Since(t)
	switch {
	case since < 24*time.Hour:
		return "today"
	case since < 7*24*time.Hour:
		return "1-7d"
	case since < 30*24*time.Hour:
		return "8-30d"
	case since < 90*24*time.Hour:
		return "30-90d"
	default:
		return "90d+"
	}
}
