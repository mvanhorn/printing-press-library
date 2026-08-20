// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBudgetCapExceeded_DailyOverCap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()

	set := newBudgetCmd(&rootFlags{asJSON: true})
	set.SetOut(&strings.Builder{})
	set.SetArgs([]string{"set", "daily", "10"})
	if err := set.Execute(); err != nil {
		t.Fatalf("budget set: %v", err)
	}

	s, err := openDefaultStore(ctx)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := s.Upsert("clips", "today", mustJSON(map[string]any{
		"id": "today", "created_at": time.Now().UTC().Format(time.RFC3339),
	})); err != nil {
		t.Fatalf("seed: %v", err)
	}
	capVal, period, exceeded, err := budgetCapExceeded(ctx, s)
	s.Close()
	if err != nil {
		t.Fatalf("budgetCapExceeded: %v", err)
	}
	if !exceeded || period != "daily" || capVal != 10 {
		t.Fatalf("expected daily cap 10 exceeded, got cap=%d period=%q exceeded=%v", capVal, period, exceeded)
	}
}

func TestBudgetCapExceeded_NoCapIsNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	s, err := openDefaultStore(ctx)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()
	_, _, exceeded, err := budgetCapExceeded(ctx, s)
	if err != nil {
		t.Fatalf("budgetCapExceeded: %v", err)
	}
	if exceeded {
		t.Fatal("no cap set must never block")
	}
}
