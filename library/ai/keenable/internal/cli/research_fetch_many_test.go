// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/keenable/internal/store"
)

// TestNovelResearchFetchManyHelpWires smoke-tests that the research fetch-many command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelResearchFetchManyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"research", "fetch-many", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("research fetch-many --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "fetch-many"} {
		if !strings.Contains(help, want) {
			t.Fatalf("research fetch-many --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestPersistResearchSnapshotIsAtomic ensures a failed page write rolls back
// the snapshot row and every earlier page in the same persistence operation.
func TestPersistResearchSnapshotIsAtomic(t *testing.T) {
	s, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "research.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	_, err = s.DB().Exec(`
		CREATE TRIGGER fail_research_pages
		BEFORE INSERT ON resources
		WHEN NEW.resource_type = 'research_pages'
		BEGIN
			SELECT RAISE(ABORT, 'forced research page write failure');
		END
	`)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	snap := researchSnapshot{ID: "snapshot-atomic-test", Query: "fetch-many", CreatedAt: "now"}
	err = persistResearchSnapshot(s, snap, nil, []researchPage{{URL: "https://example.invalid", Content: "body"}})
	if err == nil || !strings.Contains(err.Error(), "forced research page write failure") {
		t.Fatalf("persistResearchSnapshot error = %v, want forced page-write failure", err)
	}

	if _, err := s.Get("research_snapshots", snap.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("snapshot lookup error = %v, want sql.ErrNoRows after rollback", err)
	}
	pages, err := s.List("research_pages", 0)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("persisted %d pages after failed snapshot write, want 0", len(pages))
	}
}
