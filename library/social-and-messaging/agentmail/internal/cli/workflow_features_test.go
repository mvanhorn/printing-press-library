package cli

import (
	"testing"
	"time"
)

func TestWorkflowFamilyAndDirection(t *testing.T) {
	tests := []struct{ name, typ, direction, status, wantFamily, wantDirection string }{
		{"inbox scoped message", "inboxes_messages", "inbound", "", "messages", "inbound"},
		{"pod draft", "pods_drafts", "", "drafting", "drafts", "unknown"},
		{"sent status", "messages", "", "sent", "messages", "outbound"},
		{"explicit outgoing", "messages", "outbound", "", "messages", "outbound"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := workflowResource{Type: tt.typ, Data: map[string]any{"direction": tt.direction, "status": tt.status}}
			if !workflowFamily(r, tt.wantFamily) {
				t.Fatalf("workflowFamily(%q) = false", tt.typ)
			}
			if got := workflowDirection(r.Data); got != tt.wantDirection {
				t.Fatalf("workflowDirection() = %q, want %q", got, tt.wantDirection)
			}
		})
	}
}

func TestWorkflowSince(t *testing.T) {
	tests := []struct {
		name, value string
		valid       bool
		minimum     time.Duration
	}{{"days", "7d", true, 6 * 24 * time.Hour}, {"hours", "24h", true, 23 * time.Hour}, {"duration", "90m", true, 89 * time.Minute}, {"invalid", "later", false, 0}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workflowSince(tt.value)
			if (err == nil) != tt.valid {
				t.Fatalf("workflowSince(%q) err = %v, valid=%v", tt.value, err, tt.valid)
			}
			if tt.valid && time.Since(got) < tt.minimum {
				t.Fatalf("workflowSince(%q) too recent: %s", tt.value, got)
			}
		})
	}
}

func TestWorkflowListFieldsStayInitialized(t *testing.T) {
	got := wfStringList(map[string]any{}, "labels")
	if got == nil {
		t.Fatal("wfStringList returned nil; JSON list fields must be initialized")
	}
	if len(got) != 0 {
		t.Fatalf("wfStringList empty object = %#v", got)
	}
}

func TestWorkflowDirectionUsesAgentMailAddresses(t *testing.T) {
	inbox := map[string]any{"email": "support@example.com"}
	tests := []struct {
		name    string
		message map[string]any
		want    string
	}{
		{"inbound from customer", map[string]any{"from": map[string]any{"email": "customer@example.net"}, "to": []any{inbox}, "timestamp": "2026-08-26T10:00:00Z"}, "inbound"},
		{"outbound from inbox", map[string]any{"from": map[string]any{"email": "support@example.com"}, "to": []any{map[string]any{"email": "customer@example.net"}}, "timestamp": "2026-08-26T10:01:00Z"}, "outbound"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workflowDirectionFor(tt.message, wfString(inbox, "email")); got != tt.want {
				t.Fatalf("workflowDirectionFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkflowDraftAndAttachmentParentLinkage(t *testing.T) {
	resources := []workflowResource{
		{ID: "message-1", Type: "inboxes_messages", Data: map[string]any{"message_id": "message-1", "thread_id": "thread-1"}},
		{ID: "draft-1", Type: "inboxes_drafts", Data: map[string]any{"draft_id": "draft-1", "in_reply_to": "message-1"}},
	}
	if got := workflowDraftThread(resources[1], resources); got != "thread-1" {
		t.Fatalf("workflowDraftThread() = %q, want thread-1", got)
	}
	attachment := workflowResource{ID: "draft-1\x00attachment-1", Type: "inboxes_drafts_attachments", Data: map[string]any{"id": "attachment-1"}}
	if !workflowAttachmentBelongs(attachment, "draft-1") {
		t.Fatal("workflowAttachmentBelongs did not match composite parent ID")
	}
}

func TestWorkflowIDPrefersMessageIDAndStripsCompositeSuffix(t *testing.T) {
	r := workflowResource{ID: "message-1\x00inbox-1", Type: "inboxes_messages", Data: map[string]any{"message_id": "message-1"}}
	if got := wfID(r); got != "message-1" {
		t.Fatalf("wfID() = %q, want message-1", got)
	}
}

func TestWorkflowTimestampRejectsMalformedSchedule(t *testing.T) {
	if got, err := parseWorkflowTimeStrict("not-a-time"); err == nil || !got.IsZero() {
		t.Fatalf("malformed schedule = %v, err=%v", got, err)
	}
	if got, err := parseWorkflowTimeStrict("2026-08-26T10:00:00Z"); err != nil || got.IsZero() {
		t.Fatalf("valid schedule = %v, err=%v", got, err)
	}
}

func TestWorkflowExtractedContentFallback(t *testing.T) {
	message := map[string]any{"extracted_text": "reply text", "extracted_html": "<p>reply</p>"}
	if got := wfString(message, "text", "body", "content", "extracted_text", "extracted_html"); got != "reply text" {
		t.Fatalf("extracted content = %q", got)
	}
}

func TestWorkflowOutboundCountExcludesInbound(t *testing.T) {
	resources := []workflowResource{
		{ID: "inbox-1", Type: "inboxes", Data: map[string]any{"id": "inbox-1", "email": "support@example.com"}},
		{ID: "in-1", Type: "messages", Data: map[string]any{"id": "in-1", "inbox_id": "inbox-1", "from": "customer@example.net", "to": []any{"support@example.com"}}},
		{ID: "out-1", Type: "messages", Data: map[string]any{"id": "out-1", "inbox_id": "inbox-1", "from": "support@example.com", "to": []any{"customer@example.net"}}},
	}
	if got := workflowOutboundCount(resources[1:], resources); got != 1 {
		t.Fatalf("workflowOutboundCount = %d, want 1", got)
	}
}
