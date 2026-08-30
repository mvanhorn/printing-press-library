// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/store"
)

func TestNovelHotelsAliasHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"hotels", "alias", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hotels alias --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "alias"} {
		if !strings.Contains(help, want) {
			t.Fatalf("hotels alias --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestAliasResolutionInStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open temp store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Test 1: Resolve all-digits raw ID
	id, err := s.ResolveHotelID(ctx, "123456")
	if err != nil {
		t.Errorf("expected no error for raw numeric ID: %v", err)
	}
	if id != "123456" {
		t.Errorf("expected 123456, got %s", id)
	}

	// Test 2: Resolve non-existing alias
	_, err = s.ResolveHotelID(ctx, "made-nyc")
	if err == nil {
		t.Errorf("expected error for non-existent alias")
	}

	// Test 3: Add and resolve alias
	err = s.UpsertHotelAlias(ctx, "made-nyc", "102306")
	if err != nil {
		t.Fatalf("upsert alias: %v", err)
	}

	id, err = s.ResolveHotelID(ctx, "made-nyc")
	if err != nil {
		t.Fatalf("resolve existing alias: %v", err)
	}
	if id != "102306" {
		t.Errorf("expected 102306, got %s", id)
	}

	// Test 4: List aliases
	list, err := s.ListHotelAliases(ctx)
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 alias, got %d", len(list))
	}
	if list[0].Alias != "made-nyc" || list[0].HotelID != "102306" {
		t.Errorf("unexpected list item: %+v", list[0])
	}

	// Test 5: Remove alias
	removed, err := s.RemoveHotelAlias(ctx, "made-nyc")
	if err != nil {
		t.Fatalf("remove alias: %v", err)
	}
	if !removed {
		t.Errorf("expected removed to be true")
	}

	_, err = s.ResolveHotelID(ctx, "made-nyc")
	if err == nil {
		t.Errorf("expected error after removing alias")
	}
}
