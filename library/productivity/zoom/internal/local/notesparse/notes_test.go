package notesparse

import (
	"strings"
	"testing"
)

func TestExtractTodos(t *testing.T) {
	text := `Q2 Planning Notes
Mon, May 19, 2026
Meeting ID: 851 2345 6789

DISCUSSION ITEMS

TODO: ship the new pricing page (Owner: Maya)
Action: confirm rollout window with infra
Action Item: schedule a follow-up next week
Follow up: reach out to design for assets
Next steps: align with sales
[ ] update the FAQ doc
[x] post the meeting summary to Slack
Owner: Sam
Random sentence that should not match
`
	todos := extractTodos(text)
	if len(todos) < 8 {
		t.Fatalf("expected >=8 todos, got %d: %+v", len(todos), todos)
	}
	// Owner extraction on the TODO line.
	var sawMaya bool
	for _, td := range todos {
		if td.Text == "ship the new pricing page" && td.Owner == "Maya" {
			sawMaya = true
		}
	}
	if !sawMaya {
		t.Errorf("expected TODO with Maya owner; got %+v", todos)
	}
	// Checked box detection.
	var sawChecked bool
	for _, td := range todos {
		if td.Pattern == "checkbox_done" && td.Checked {
			sawChecked = true
		}
	}
	if !sawChecked {
		t.Error("expected at least one checked-box todo")
	}
}

func TestBuildNoteDetectsTopicAndDate(t *testing.T) {
	text := "Q2 Planning\n\n2026-05-19\n\nDiscussion items here.\n\n[ ] follow up\n"
	n := buildNote("/tmp/zoom-notes-test.pdf", "pdf", text)
	if n.MeetingTopic == "" {
		t.Error("expected non-empty meeting topic")
	}
	if n.StartTime.IsZero() {
		t.Error("expected start_time to be detected from 2026-05-19")
	}
	if len(n.Segments) == 0 {
		t.Error("expected at least one segment")
	}
	if len(n.Todos) == 0 {
		t.Error("expected at least one todo")
	}
}

func TestParseUnsupportedFormat(t *testing.T) {
	if _, err := Parse("/tmp/whatever.xls"); err == nil || !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("expected unsupported-file-type error, got: %v", err)
	}
}
