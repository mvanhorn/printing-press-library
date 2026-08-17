// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// TestTeachLookup_HappyPath: a single teach-lookup invocation writes
// a row into entity_lookups, exits silently with code 0, and the row
// is discoverable via lookups.Lookup against the same DB.
func TestTeachLookup_HappyPath(t *testing.T) {
	home := withTempHome(t)
	dbPath := filepath.Join(home, "data.db")

	// Use a canonical NOT in the seed payload to confirm the write
	// path is real (a seeded canonical would conflict on PK and
	// silently no-op, masking a real bug if the write logic broke).
	stdout, stderr, err := runRootArgs(t,
		"teach-lookup",
		"--kind", "stock_ticker",
		"--canonical", "Microsoft",
		"--value", "MSFT",
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("teach-lookup failed: %v (stderr=%q)", err, stderr)
	}
	if stdout != "" {
		t.Errorf("teach-lookup should be silent on success; stdout=%q", stdout)
	}
	if stderr != "" {
		t.Errorf("teach-lookup should be silent on success; stderr=%q", stderr)
	}

	// Verify the row landed and Lookup resolves it.
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer s.Close()
	got, ok, err := lookups.Lookup(s.DB(), "stock_ticker", "Microsoft")
	if err != nil {
		t.Fatalf("lookups.Lookup: %v", err)
	}
	if !ok || got != "MSFT" {
		t.Errorf("Lookup(stock_ticker, Microsoft) = (%q, %v), want (\"MSFT\", true)", got, ok)
	}

	// And source should default to 'taught' since no --source was passed.
	var source string
	if err := s.DB().QueryRow(
		`SELECT source FROM entity_lookups WHERE kind = ? AND canonical = ? AND value = ?`,
		"stock_ticker", "Microsoft", "MSFT",
	).Scan(&source); err != nil {
		t.Fatalf("select source: %v", err)
	}
	if source != "taught" {
		t.Errorf("source = %q, want %q", source, "taught")
	}

	// Also pin the plan's worked example: teaching a *seeded*
	// canonical with a *new* value (different from the seed)
	// creates a separate row alongside the seed, since the PK is
	// (kind, canonical, VALUE).
	stdout, _, err = runRootArgs(t,
		"teach-lookup",
		"--kind", "country_iso2",
		"--canonical", "Bosnia and Herzegovina",
		"--value", "BIH-CUSTOM",
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("teach-lookup custom alias: %v", err)
	}
	var customCount int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM entity_lookups WHERE kind = ? AND canonical = ? AND value = ?`,
		"country_iso2", "Bosnia and Herzegovina", "BIH-CUSTOM",
	).Scan(&customCount); err != nil {
		t.Fatalf("count custom: %v", err)
	}
	if customCount != 1 {
		t.Errorf("teach-lookup with new value beside seeded canonical wrote %d rows, want 1", customCount)
	}
}

// TestTeachLookup_MissingFlags pins the usage-error path: any of
// --kind, --canonical, or --value missing produces a non-zero exit
// with usageErr code 2.
func TestTeachLookup_MissingFlags(t *testing.T) {
	withTempHome(t)
	cases := []struct {
		name  string
		args  []string
		want  string
	}{
		{
			name: "missing kind",
			args: []string{"teach-lookup", "--canonical", "Portugal", "--value", "PT"},
			want: "kind is required",
		},
		{
			name: "missing canonical",
			args: []string{"teach-lookup", "--kind", "country_iso2", "--value", "PT"},
			want: "canonical is required",
		},
		{
			name: "missing value",
			args: []string{"teach-lookup", "--kind", "country_iso2", "--canonical", "Portugal"},
			want: "value is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runRootArgs(t, tc.args...)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			if ExitCode(err) != 2 {
				t.Errorf("ExitCode = %d, want 2 (usage error)", ExitCode(err))
			}
		})
	}
}

// TestTeachLookup_RejectsComputedKind verifies that teaching a row
// under a computed kind (lowercase, kebab-case, etc.) is refused.
// Computed kinds resolve via string transform, not DB lookup, so
// adding a row would silently be ignored.
func TestTeachLookup_RejectsComputedKind(t *testing.T) {
	home := withTempHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"teach-lookup",
		"--kind", "lowercase",
		"--canonical", "Portugal",
		"--value", "portugal",
		"--db", dbPath,
	)
	if err == nil {
		t.Fatalf("expected error for computed kind, got nil")
	}
	if !strings.Contains(err.Error(), "computed kind") {
		t.Errorf("error = %v, want substring \"computed kind\"", err)
	}
	if ExitCode(err) != 2 {
		t.Errorf("ExitCode = %d, want 2", ExitCode(err))
	}
}

// TestTeachLookup_Idempotent verifies that re-running the same teach
// is a silent no-op via INSERT OR IGNORE. Exactly one row should
// exist after N invocations.
func TestTeachLookup_Idempotent(t *testing.T) {
	home := withTempHome(t)
	dbPath := filepath.Join(home, "data.db")

	for i := 0; i < 3; i++ {
		_, _, err := runRootArgs(t,
			"teach-lookup",
			"--kind", "country_iso2",
			"--canonical", "Curaçao",
			"--value", "CW",
			"--db", dbPath,
		)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}

	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	var count int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "Curaçao",
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("repeated teach-lookup wrote %d rows, want 1 (PK conflict should silence)", count)
	}
}

// TestTeachLookup_RespectsNoLearnFlag confirms the --no-learn flag
// makes teach-lookup a no-op, matching how `teach` behaves. A user
// who has globally disabled learning expects the entire surface to
// stay quiet.
func TestTeachLookup_RespectsNoLearnFlag(t *testing.T) {
	home := withTempHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"--no-learn",
		"teach-lookup",
		"--kind", "country_iso2",
		"--canonical", "Portugal",
		"--value", "PT",
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("teach-lookup --no-learn: %v", err)
	}

	// The DB wasn't even opened, so the file should not exist OR
	// it should be empty of our row. Open with a clean store and
	// confirm there are no rows for Portugal under country_iso2
	// (other than seeds, which the empty DB hasn't gotten yet
	// because we never opened it from the CLI side). The simpler
	// guard: the DB file was never created at all.
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// The DB now exists (we just created it), but it should have
	// the seeded Portugal row (not the taught one). The taught row
	// would have source='taught'; the seed has source='seeded'.
	var sources []string
	rows, err := s.DB().Query(
		`SELECT source FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "Portugal",
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			t.Fatalf("scan: %v", err)
		}
		sources = append(sources, source)
	}
	for _, src := range sources {
		if src == "taught" {
			t.Errorf("--no-learn write leaked through; saw source=taught row")
		}
	}
}

// TestTeachLookup_JSONOutput exercises the --json output path —
// when --json is set globally, teach-lookup emits the recorded
// envelope so scripted callers can confirm the write.
func TestTeachLookup_JSONOutput(t *testing.T) {
	home := withTempHome(t)
	dbPath := filepath.Join(home, "data.db")

	stdout, _, err := runRootArgs(t,
		"--json",
		"teach-lookup",
		"--kind", "stock_ticker",
		"--canonical", "Apple Inc.",
		"--value", "AAPL",
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("teach-lookup --json: %v", err)
	}
	// The printer may pretty-print, so test for whitespace-tolerant
	// substrings of the JSON shape.
	if !strings.Contains(stdout, "recorded") || !strings.Contains(stdout, "true") {
		t.Errorf("JSON output missing recorded:true: %s", stdout)
	}
	if !strings.Contains(stdout, "AAPL") {
		t.Errorf("JSON output missing AAPL: %s", stdout)
	}
}
