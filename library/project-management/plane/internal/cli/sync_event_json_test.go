// Copyright 2026 The plane-pp-cli authors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Dependent resources store NUL-composite ids (<uuid>\x00<parent_uuid>);
// emitting one raw into a hand-assembled NDJSON event line breaks the stream
// (dogfood json_fidelity flags the whole sync output as invalid JSON).
// jsonString must yield a valid JSON literal that round-trips the id exactly.
func TestJSONStringEscapesNULCompositeIDs(t *testing.T) {
	composite := "bf6d1b61-4e83-4f7a-8633-3f8a8dce9b09\x001c3e6858-2b6b-42e0-a8d2-2b0dc70798f4"
	line := fmt.Sprintf(
		`{"event":"sync_warning","resource":"%s","parent":%s,"reason":"max_pages_cap_hit","message":"reached cap"}`,
		"cycles_cycle_issues", jsonString(composite))
	if !json.Valid([]byte(line)) {
		t.Fatalf("warning line with NUL-composite parent id is not valid JSON: %q", line)
	}
	var parsed struct {
		Parent string `json:"parent"`
	}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Parent != composite {
		t.Fatalf("parent id did not round-trip: got %q want %q", parsed.Parent, composite)
	}
	if got := jsonString("plain-id"); got != `"plain-id"` {
		t.Fatalf("plain id should render as a bare quoted literal, got %s", got)
	}
}
