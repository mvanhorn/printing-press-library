// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func TestExtractProjectClipIDs(t *testing.T) {
	cases := map[string]string{
		"nested clip object": `{"project_clips":[{"clip":{"id":"c1"}},{"clip":{"id":"c2"}}]}`,
		"clip_id scalar":     `{"project_clips":[{"clip_id":"c1"},{"clip_id":"c2"}]}`,
		"bare clips array":   `{"clips":[{"id":"c1"},{"id":"c2"}]}`,
	}
	for name, body := range cases {
		ids := extractProjectClipIDs(json.RawMessage(body))
		sort.Strings(ids)
		if len(ids) != 2 || ids[0] != "c1" || ids[1] != "c2" {
			t.Errorf("%s: got %v, want [c1 c2]", name, ids)
		}
	}
	if ids := extractProjectClipIDs(json.RawMessage(`{"name":"empty"}`)); len(ids) != 0 {
		t.Errorf("no clips: got %v, want empty", ids)
	}
}

func TestWorkspaceMembership_EndToEnd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	s, err := openDefaultStore(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if _, _, err := s.UpsertBatch("workspace", []json.RawMessage{
		json.RawMessage(`{"id":"w1","name":"Chill"}`),
		json.RawMessage(`{"id":"w2","name":"Hype"}`),
	}); err != nil {
		t.Fatalf("seed workspaces: %v", err)
	}
	if _, _, err := s.UpsertBatch("clips", []json.RawMessage{
		json.RawMessage(`{"id":"c1","duration":100,"play_count":5}`),
		json.RawMessage(`{"id":"c2","duration":200,"play_count":3}`),
		json.RawMessage(`{"id":"c3","duration":50,"play_count":1}`),
	}); err != nil {
		t.Fatalf("seed clips: %v", err)
	}

	// c2 belongs to both workspaces; c3 to none.
	if err := s.ReplaceWorkspaceMembership("w1", []string{"c1", "c2"}); err != nil {
		t.Fatalf("membership w1: %v", err)
	}
	if err := s.ReplaceWorkspaceMembership("w2", []string{"c2"}); err != nil {
		t.Fatalf("membership w2: %v", err)
	}

	labels, err := s.WorkspaceLabelsForClips([]string{"c1", "c2", "c3"})
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	if labels["c1"] != "Chill" {
		t.Errorf("c1 label = %q, want Chill", labels["c1"])
	}
	if !strings.Contains(labels["c2"], "Chill") || !strings.Contains(labels["c2"], "Hype") {
		t.Errorf("c2 label = %q, want both Chill and Hype", labels["c2"])
	}
	if _, ok := labels["c3"]; ok {
		t.Errorf("c3 should have no membership, got %q", labels["c3"])
	}

	// Replace semantics: re-set w1 to only c1 removes c2 from w1.
	if err := s.ReplaceWorkspaceMembership("w1", []string{"c1"}); err != nil {
		t.Fatalf("re-membership w1: %v", err)
	}
	labels, _ = s.WorkspaceLabelsForClips([]string{"c2"})
	if labels["c2"] != "Hype" {
		t.Errorf("after replace, c2 = %q, want Hype only", labels["c2"])
	}

	// analytics --group-by project rolls up via membership (back to w1={c1,c2}).
	if err := s.ReplaceWorkspaceMembership("w1", []string{"c1", "c2"}); err != nil {
		t.Fatalf("restore w1: %v", err)
	}
	groups, err := analyticsByProject(s, 0)
	if err != nil {
		t.Fatalf("analyticsByProject: %v", err)
	}
	counts := map[string]int64{}
	for _, g := range groups {
		counts[g.Group] = g.Count
	}
	if counts["Chill"] != 2 || counts["Hype"] != 1 {
		t.Errorf("project counts = %v, want Chill=2 Hype=1", counts)
	}
}
