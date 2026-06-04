// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBurn_AggregatesByTag(t *testing.T) {
	seedClips(t, []map[string]any{
		{"id": "1", "tags": "lofi, chill"},
		{"id": "2", "tags": "lofi"},
		{"id": "3", "tags": "metal"},
	})
	cmd := newBurnCmd(&rootFlags{asJSON: true})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--by", "tag"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("burn: %v", err)
	}
	var rows []burnRow
	if err := json.Unmarshal([]byte(out.String()), &rows); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	byTag := map[string]burnRow{}
	for _, r := range rows {
		byTag[r.GroupValue] = r
	}
	if byTag["lofi"].GenerationCount != 2 {
		t.Fatalf("expected lofi count 2, got %d", byTag["lofi"].GenerationCount)
	}
	if byTag["lofi"].EstimatedCredits != 20 {
		t.Fatalf("expected lofi credits 20 (10/gen), got %d", byTag["lofi"].EstimatedCredits)
	}
}
