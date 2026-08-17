// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
)

func TestRunLedgerSearchAndVault(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run := RunRecord{
		ID: "run_test", Command: "ask", Prompt: "Find revenue leakage",
		Answer: "Check churn cohorts", Reasoning: "Revenue changed after discounting",
		Model: "qwen/qwen3.5-4b-free", RawJSON: json.RawMessage(`{"ok":true}`),
	}
	if err := s.SaveRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	found, err := s.SearchRuns(ctx, "revenue", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID != "run_test" {
		t.Fatalf("SearchRuns returned %#v", found)
	}
	if err := s.SaveVaultEntry(ctx, VaultEntry{Token: "EMAIL_1", Value: "cathryn@example.com", Kind: "EMAIL"}); err != nil {
		t.Fatal(err)
	}
	entry, ok, err := s.TokenForValue(ctx, "EMAIL", "cathryn@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || entry.Token != "EMAIL_1" {
		t.Fatalf("TokenForValue = %#v, %v", entry, ok)
	}
}

func TestNewIDDoesNotCollideInTightLoop(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 10000; i++ {
		id := NewID("run")
		if seen[id] {
			t.Fatalf("NewID collision at iteration %d: %s", i, id)
		}
		seen[id] = true
	}
}

func TestEnsureVaultEntryUsesLessGuessableTokens(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	entry, err := s.EnsureVaultEntry(ctx, "EMAIL", "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^EMAIL_[0-9a-f]{8}$`).MatchString(entry.Token) {
		t.Fatalf("token = %q, want EMAIL_<8 hex chars>", entry.Token)
	}
	if entry.Token == "EMAIL_1" {
		t.Fatalf("token = %q, want non-sequential token", entry.Token)
	}
}

func TestEnsureVaultEntryConcurrentStoreHandles(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	seed, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	seed.Close()

	const writers = 16
	start := make(chan struct{})
	errCh := make(chan error, writers)
	entryCh := make(chan VaultEntry, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := Open(dbPath)
			if err != nil {
				errCh <- fmt.Errorf("open writer %d: %w", i, err)
				return
			}
			defer s.Close()
			<-start
			entry, err := s.EnsureVaultEntry(ctx, "EMAIL", fmt.Sprintf("person%d@example.com", i))
			if err != nil {
				errCh <- fmt.Errorf("ensure writer %d: %w", i, err)
				return
			}
			entryCh <- entry
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)
	close(entryCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	tokens := map[string]bool{}
	for entry := range entryCh {
		if tokens[entry.Token] {
			t.Fatalf("duplicate vault token assigned: %s", entry.Token)
		}
		tokens[entry.Token] = true
	}
	if len(tokens) != writers {
		t.Fatalf("tokens = %d, want %d", len(tokens), writers)
	}
}
