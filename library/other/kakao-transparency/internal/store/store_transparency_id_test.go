// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored test for the transparency composite period id.

package store

import "testing"

func TestExtractResourceIDTransparencyCompositePeriod(t *testing.T) {
	flat := map[string]any{"year": "2025", "halfYearId": 1}
	if got := ExtractResourceID("transparency", flat); got != "2025-1" {
		t.Errorf("flat report: got %q, want 2025-1", got)
	}
	enveloped := map[string]any{
		"success": true,
		"data":    map[string]any{"year": "2024", "halfYearId": 2},
	}
	if got := ExtractResourceID("transparency", enveloped); got != "2024-2" {
		t.Errorf("enveloped report: got %q, want 2024-2", got)
	}
	if got := ExtractResourceID("transparency", map[string]any{"success": false}); got != "" {
		t.Errorf("missing period fields: got %q, want empty", got)
	}
}
