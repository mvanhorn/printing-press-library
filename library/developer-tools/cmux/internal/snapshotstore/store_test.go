// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package snapshotstore

import (
	"context"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "snap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecordObservationTransitions(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	transitioned, err := s.RecordObservation(ctx, "workspace:6", "claude_code", "Running", "bolt.fill", "#222", "working", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if !transitioned {
		t.Fatalf("first observation should be a transition")
	}

	transitioned, err = s.RecordObservation(ctx, "workspace:6", "claude_code", "Running", "bolt.fill", "#222", "working", 2.0)
	if err != nil {
		t.Fatal(err)
	}
	if transitioned {
		t.Fatalf("same value should NOT be a transition")
	}

	transitioned, err = s.RecordObservation(ctx, "workspace:6", "claude_code", "Needs input", "bell.fill", "#4C8DFF", "awaiting", 3.0)
	if err != nil {
		t.Fatal(err)
	}
	if !transitioned {
		t.Fatalf("value change should be a transition")
	}

	changes, err := s.Changes(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(changes))
	}
	if changes[0].Value != "Needs input" || changes[0].PrevValue != "Running" {
		t.Fatalf("latest transition wrong: %+v", changes[0])
	}
}

func TestLatestPerWorkspaceKey(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_, _ = s.RecordObservation(ctx, "workspace:1", "claude_code", "Running", "", "", "working", 1.0)
	_, _ = s.RecordObservation(ctx, "workspace:1", "claude_code", "Needs input", "", "", "awaiting", 2.0)
	_, _ = s.RecordObservation(ctx, "workspace:2", "claude_code", "Running", "", "", "working", 3.0)
	latest, err := s.LatestPerWorkspaceKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("expected 2 latest rows, got %d", len(latest))
	}
}

func TestAlertRules(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id, err := s.AddAlertRule(ctx, "workspace:1", "claude_code", "awaiting", "stdout:", "Tuck awaiting")
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}
	rules, err := s.ListAlertRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].WorkspaceRef != "workspace:1" || rules[0].OnState != "awaiting" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
	removed, err := s.RemoveAlertRule(ctx, id)
	if err != nil || removed != 1 {
		t.Fatalf("removal failed: %d %v", removed, err)
	}
}

func TestPaneContentFTS(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.RecordPaneSample(ctx, "workspace:1", "surface:128", "Claude Code is waiting for your input on the WAF cookie investigation")
	_ = s.RecordPaneSample(ctx, "workspace:2", "surface:240", "Running tests; everything green")
	hits, err := s.SearchPaneContent(ctx, "WAF", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].WorkspaceRef != "workspace:1" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}
