package cli

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func assertAvailabilityUnionIndexes(t *testing.T, db *sql.DB, query string, args []any) {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail)
		plan.WriteByte('\n')
	}
	for _, index := range []string{"idx_availability_od_date", "idx_awards_od_date"} {
		if !strings.Contains(plan.String(), index) {
			t.Fatalf("plan for %q missing %s:\n%s", query, index, plan.String())
		}
	}
}

func TestNovelQueryBuildersKeepAvailabilityUnionIndexes(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	tests := []struct {
		name  string
		build func() (string, []any)
	}{
		{"new-since", func() (string, []any) {
			return buildNewSinceQuery(now.Add(-time.Hour), "JFK", "NRT", "", "business", 10)
		}},
		{"direct-scan", func() (string, []any) {
			return buildDirectScanQuery("j", "JFK", "NRT", nil, "2026-10-01", "2026-10-05", 0, 10)
		}},
		{"recheck", func() (string, []any) { return buildRecheckQuery("j", "JFK", "NRT", "", 0, now, 10) }},
		{"reach", func() (string, []any) { return buildReachEvidenceQuery("JFK", "NRT", "j", now) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { q, args := tc.build(); assertAvailabilityUnionIndexes(t, db.DB(), q, args) })
	}
}
