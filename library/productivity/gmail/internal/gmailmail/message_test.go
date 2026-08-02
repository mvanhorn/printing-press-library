// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
package gmailmail

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeB64URL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"unpadded url alphabet", "aGVsbG8gd29ybGQ", "hello world"},
		{"padded url alphabet", "aGVsbG8gd29ybGQ=", "hello world"},
		{"url specials", "P8O8YsOkci1fIMOfIQ", "?übär-_ ß!"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeB64URL(tc.in)
			if err != nil {
				t.Fatalf("DecodeB64URL(%q) error = %v", tc.in, err)
			}
			if string(got) != tc.want {
				t.Fatalf("DecodeB64URL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractEmail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Jane Doe <Jane@Example.com>", "jane@example.com"},
		{"jane@example.com", "jane@example.com"},
		{`"Doe, Jane" <jane+tag@example.com>`, "jane+tag@example.com"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ExtractEmail(tc.in); got != tc.want {
			t.Fatalf("ExtractEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func nestedFixture() *Part {
	textData := EncodeB64URL([]byte("plain body"))
	htmlData := EncodeB64URL([]byte("<p>html body</p>"))
	return &Part{
		MimeType: "multipart/mixed",
		Parts: []Part{
			{
				MimeType: "multipart/alternative",
				Parts: []Part{
					{MimeType: "text/plain", Body: PartBody{Data: textData}},
					{MimeType: "text/html", Body: PartBody{Data: htmlData}},
				},
			},
			{
				MimeType: "application/pdf",
				Filename: "invoice.pdf",
				Body:     PartBody{AttachmentID: "att-123", Size: 2048},
			},
		},
	}
}

func TestExtractBodyNestedMultipart(t *testing.T) {
	text, html := ExtractBody(nestedFixture())
	if text != "plain body" {
		t.Fatalf("text = %q, want %q", text, "plain body")
	}
	if html != "<p>html body</p>" {
		t.Fatalf("html = %q, want %q", html, "<p>html body</p>")
	}
}

func TestAttachmentsNested(t *testing.T) {
	atts := Attachments(nestedFixture())
	if len(atts) != 1 {
		t.Fatalf("Attachments() len = %d, want 1", len(atts))
	}
	if atts[0].Filename != "invoice.pdf" || atts[0].AttachmentID != "att-123" || atts[0].Size != 2048 {
		t.Fatalf("Attachments()[0] = %+v", atts[0])
	}
}

func TestHTMLToText(t *testing.T) {
	in := `<html><head><style>p{color:red}</style></head><body><p>Hello &amp; welcome</p><div>Second line</div><script>alert(1)</script></body></html>`
	got := HTMLToText(in)
	if !strings.Contains(got, "Hello & welcome") {
		t.Fatalf("HTMLToText missing entity-decoded text: %q", got)
	}
	if !strings.Contains(got, "Second line") {
		t.Fatalf("HTMLToText missing block content: %q", got)
	}
	if strings.Contains(got, "alert(1)") || strings.Contains(got, "color:red") {
		t.Fatalf("HTMLToText leaked script/style content: %q", got)
	}
	if strings.Contains(got, "<") {
		t.Fatalf("HTMLToText left tags behind: %q", got)
	}
}

func TestBodyTextPrefersPlainFallsBackToHTML(t *testing.T) {
	msgPlain := &Message{Payload: nestedFixture()}
	if got := msgPlain.BodyText(); got != "plain body" {
		t.Fatalf("BodyText = %q, want plain body", got)
	}
	htmlOnly := &Message{Payload: &Part{
		MimeType: "text/html",
		Body:     PartBody{Data: EncodeB64URL([]byte("<p>only html</p>"))},
	}}
	if got := htmlOnly.BodyText(); got != "only html" {
		t.Fatalf("BodyText html fallback = %q, want only html", got)
	}
}

func TestHeaderAndInternalTime(t *testing.T) {
	m := &Message{
		InternalDate: "1722550000000",
		Payload: &Part{Headers: []Header{
			{Name: "From", Value: "A <a@example.com>"},
			{Name: "subject", Value: "Hi"},
		}},
	}
	if got := m.Header("Subject"); got != "Hi" {
		t.Fatalf("Header(Subject) = %q (case-insensitive lookup failed)", got)
	}
	ts, ok := m.InternalTime()
	if !ok || ts.Year() != 2024 {
		t.Fatalf("InternalTime = %v ok=%v, want 2024 timestamp", ts, ok)
	}
}

func TestFlatten(t *testing.T) {
	m := &Message{
		ID:           "m1",
		ThreadID:     "t1",
		LabelIDs:     []string{"INBOX", "UNREAD"},
		SizeEstimate: 512,
		InternalDate: "1722550000000",
		Payload: &Part{Headers: []Header{
			{Name: "From", Value: "Jane <jane@example.com>"},
			{Name: "Subject", Value: "Report"},
			{Name: "List-Unsubscribe", Value: "<mailto:leave@list.com>"},
		}},
	}
	raw, err := m.Flatten("body here")
	if err != nil {
		t.Fatalf("Flatten error = %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("Flatten produced invalid JSON: %v", err)
	}
	if doc["from_email"] != "jane@example.com" {
		t.Fatalf("from_email = %v", doc["from_email"])
	}
	if doc["unread"] != true {
		t.Fatalf("unread = %v, want true", doc["unread"])
	}
	if doc["list_unsubscribe"] != "<mailto:leave@list.com>" {
		t.Fatalf("list_unsubscribe = %v", doc["list_unsubscribe"])
	}
	if doc["body_text"] != "body here" {
		t.Fatalf("body_text = %v", doc["body_text"])
	}
}
