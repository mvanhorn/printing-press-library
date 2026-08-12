// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

// A dry run under --trust-mode strict must agree with the real invocation. The
// real one resolves TEAM-NUMBER to a UUID and then refuses a target absent from
// the pp_created ledger, so a dry run that reports "would update" for such a
// target is a lie. The ledger records the identifier as well as the UUID, which
// is what lets the dry run answer offline.
func TestTrustModeDryRunGuardChecksIdentifiers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.RecordPPFixture("11111111-1111-1111-1111-111111111111", "ENG-7", "Fixture", "sess-1"); err != nil {
		t.Fatalf("RecordPPFixture: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	strict := &rootFlags{trustMode: "strict"}
	loose := &rootFlags{}

	if err := trustModeDryRunGuard(strict, dbPath, "ENG-7"); err != nil {
		t.Fatalf("a ledger fixture named by identifier must pass: %v", err)
	}
	if err := trustModeDryRunGuard(strict, dbPath, "eng-7"); err != nil {
		t.Fatalf("identifier comparison must not be case-sensitive: %v", err)
	}
	if err := trustModeDryRunGuard(strict, dbPath, "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("a ledger fixture named by UUID must pass: %v", err)
	}

	err = trustModeDryRunGuard(strict, dbPath, "ENG-8")
	if err == nil {
		t.Fatal("an identifier absent from the ledger must be refused, not reported as a would-update")
	}
	var cerr *cliError
	if !errors.As(err, &cerr) || cerr.code != 2 {
		t.Fatalf("want exit code 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "ENG-8") {
		t.Fatalf("error must name the refused target: %v", err)
	}

	if err := trustModeDryRunGuard(loose, dbPath, "ENG-8"); err != nil {
		t.Fatalf("the guard only applies under strict mode: %v", err)
	}
}

// stubQueryer answers the single-issue read with a canned payload.
type stubQueryer struct {
	description string
	updatedAt   string
	calls       int
}

func (s *stubQueryer) QueryInto(_ string, _ map[string]any, out any) error {
	s.calls++
	payload := map[string]any{
		"issue": map[string]any{
			"id":          "11111111-1111-1111-1111-111111111111",
			"identifier":  "ENG-7",
			"description": s.description,
			"updatedAt":   s.updatedAt,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// issueUpdate carries a whole description, so an append composed from a stale
// body deletes whatever landed in between. The guard refuses that write instead
// of performing it silently.
func TestGuardStaleDescription(t *testing.T) {
	t.Parallel()

	unchanged := &stubQueryer{description: "original body", updatedAt: "2026-08-12T09:00:00Z"}
	if err := guardStaleDescription(unchanged, "11111111-1111-1111-1111-111111111111", "2026-08-12T09:00:00Z", "original body"); err != nil {
		t.Fatalf("an unchanged issue must not be refused: %v", err)
	}
	if unchanged.calls != 1 {
		t.Fatalf("guard made %d reads, want exactly 1", unchanged.calls)
	}

	// A field other than the description moved: the body is still the one the
	// append was composed from, so the write is safe.
	touched := &stubQueryer{description: "original body", updatedAt: "2026-08-12T09:05:00Z"}
	if err := guardStaleDescription(touched, "11111111-1111-1111-1111-111111111111", "2026-08-12T09:00:00Z", "original body"); err != nil {
		t.Fatalf("a same-body issue must not be refused: %v", err)
	}

	moved := &stubQueryer{description: "original body plus someone else's paragraph", updatedAt: "2026-08-12T09:05:00Z"}
	err := guardStaleDescription(moved, "11111111-1111-1111-1111-111111111111", "2026-08-12T09:00:00Z", "original body")
	if err == nil {
		t.Fatal("a description edited since the read must refuse the write")
	}
	var cerr *cliError
	if !errors.As(err, &cerr) || cerr.code != 5 {
		t.Fatalf("want exit code 5, got %v", err)
	}
	if !strings.Contains(err.Error(), "would delete that edit") {
		t.Fatalf("error must say what the write would destroy: %v", err)
	}
}
