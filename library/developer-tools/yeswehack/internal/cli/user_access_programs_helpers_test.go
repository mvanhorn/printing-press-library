package cli

import (
	"encoding/json"
	"testing"
)

func TestAnnotateHunterAccessProgramsAddsResourceScopedID(t *testing.T) {
	raw := json.RawMessage(`{"items":[{"slug":"program-one","title":"Program One"}]}`)

	annotated := annotateHunterAccessPrograms(raw)

	var envelope struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(annotated, &envelope); err != nil {
		t.Fatalf("unmarshal annotated access programs: %v", err)
	}
	if len(envelope.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(envelope.Items))
	}
	if got := stringField(envelope.Items[0], "id"); got != "hunter-access-programs|program-one" {
		t.Fatalf("id = %q, want hunter-access-programs|program-one", got)
	}
	if got := stringField(envelope.Items[0], "program_slug"); got != "program-one" {
		t.Fatalf("program_slug = %q, want program-one", got)
	}
}

func TestPrepareSyncItemsAnnotatesHunterAccessPrograms(t *testing.T) {
	items := []json.RawMessage{json.RawMessage(`{"slug":"program-one"}`)}

	prepared := prepareSyncItems("hunter-access-programs", items)

	var row map[string]any
	if err := json.Unmarshal(prepared[0], &row); err != nil {
		t.Fatalf("unmarshal prepared item: %v", err)
	}
	if got := stringField(row, "id"); got != "hunter-access-programs|program-one" {
		t.Fatalf("id = %q, want hunter-access-programs|program-one", got)
	}
}
