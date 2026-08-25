// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Pure parsing helpers for the mail_meta sync path: From-header splitting,
// CATEGORY_* label derivation, and Gmail message -> store.MailMeta mapping.
// Kept free of I/O so they are table-testable.

package cli

import (
	"net/mail"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// parseFromHeader splits an RFC 5322 From header into (email, name).
// Handles `Name <addr>`, `"Quoted, Name" <addr>`, RFC 2047 encoded-word
// names, bare addresses, and angle-only forms. The email is lowercased so
// sender aggregation groups case-variant spellings of the same mailbox.
// Malformed input degrades honestly: an embedded <addr> or a bare token
// containing '@' is recovered as the email; otherwise the raw text is kept
// as the name with an empty email.
func parseFromHeader(h string) (email, name string) {
	h = strings.TrimSpace(h)
	if h == "" {
		return "", ""
	}
	if a, err := mail.ParseAddress(h); err == nil {
		return strings.ToLower(a.Address), strings.TrimSpace(a.Name)
	}
	// Fallback for headers net/mail rejects (unquoted commas, stray
	// punctuation, half-broken clients).
	if open := strings.LastIndex(h, "<"); open >= 0 {
		if close := strings.Index(h[open:], ">"); close > 1 {
			addr := strings.TrimSpace(h[open+1 : open+close])
			nm := strings.Trim(strings.TrimSpace(h[:open]), `"' `)
			if strings.Contains(addr, "@") {
				return strings.ToLower(addr), nm
			}
		}
	}
	if strings.Contains(h, "@") && !strings.ContainsAny(h, " \t<>") {
		return strings.ToLower(h), ""
	}
	return "", h
}

// gmailCategoryLabels maps Gmail's CATEGORY_* system labels to the category
// vocabulary mail_meta stores. CATEGORY_PERSONAL is the Primary tab.
// Checked in this fixed order so a message carrying several category labels
// (rare, but Gmail permits it during re-categorization) derives
// deterministically.
var gmailCategoryLabels = []struct{ label, category string }{
	{"CATEGORY_PERSONAL", "primary"},
	{"CATEGORY_PROMOTIONS", "promotions"},
	{"CATEGORY_SOCIAL", "social"},
	{"CATEGORY_UPDATES", "updates"},
	{"CATEGORY_FORUMS", "forums"},
}

// deriveCategory maps a message's labelIds to a category: the CATEGORY_*
// label when present, else "primary" when the message sits in INBOX
// (uncategorized inbox mail is the Primary tab), else "".
func deriveCategory(labelIDs []string) string {
	set := make(map[string]bool, len(labelIDs))
	for _, l := range labelIDs {
		set[l] = true
	}
	for _, m := range gmailCategoryLabels {
		if set[m.label] {
			return m.category
		}
	}
	if set["INBOX"] {
		return "primary"
	}
	return ""
}

// hasLabel reports whether labelIDs contains the given label.
func hasLabel(labelIDs []string, label string) bool {
	for _, l := range labelIDs {
		if l == label {
			return true
		}
	}
	return false
}

// gmailMessageMeta mirrors the fields of a users.messages.get
// format=metadata response that mail_meta stores.
type gmailMessageMeta struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId"`
	LabelIDs     []string `json:"labelIds"`
	Snippet      string   `json:"snippet"`
	SizeEstimate int64    `json:"sizeEstimate"`
	InternalDate string   `json:"internalDate"` // ms since epoch, as a string
	Payload      struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}

// mailMetaFromMessage flattens a Gmail metadata message into a MailMeta row
// for the given account.
func mailMetaFromMessage(account string, msg *gmailMessageMeta) store.MailMeta {
	m := store.MailMeta{
		Account:      account,
		ID:           msg.ID,
		ThreadID:     msg.ThreadID,
		Snippet:      msg.Snippet,
		SizeEstimate: msg.SizeEstimate,
		LabelIDs:     msg.LabelIDs,
		Category:     deriveCategory(msg.LabelIDs),
		Unread:       hasLabel(msg.LabelIDs, "UNREAD"),
	}
	if msg.LabelIDs == nil {
		m.LabelIDs = []string{}
	}
	if ms, err := strconv.ParseInt(msg.InternalDate, 10, 64); err == nil {
		m.InternalDate = ms
	}
	for _, h := range msg.Payload.Headers {
		switch {
		case strings.EqualFold(h.Name, "From"):
			m.FromEmail, m.FromName = parseFromHeader(h.Value)
		case strings.EqualFold(h.Name, "Subject"):
			m.Subject = h.Value
		case strings.EqualFold(h.Name, "List-Unsubscribe"):
			m.ListUnsubscribe = strings.TrimSpace(h.Value)
		case strings.EqualFold(h.Name, "List-Unsubscribe-Post"):
			m.ListUnsubscribePost = strings.TrimSpace(h.Value)
		case strings.EqualFold(h.Name, "Authentication-Results"):
			// Raw header value; the unsubscribe engine parses it for
			// dkim=pass verification. ListUnsubDomain stays '' at sync
			// time — that engine owns its extraction.
			m.AuthResults = strings.TrimSpace(h.Value)
		}
	}
	return m
}
