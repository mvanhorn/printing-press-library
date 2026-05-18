// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
)

// TestPlanForm4Ingest_TruncatesWhenAboveCap stubs a CIK with 250 cached
// Form 4 filings in the 12-month window and asserts that planForm4Ingest
// reports form4_truncated=true and form4_total_in_window=250 under the
// default --max-form4 200 cap. Guards against the prior silent-truncation
// bug Greptile flagged on insider_summary.go.
func TestPlanForm4Ingest_TruncatesWhenAboveCap(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	if err := s.EnsureEdgarSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const cik = "0000000042"
	const total = 250
	sinceISO := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	seedForm4Filings(t, s, cik, total)

	skip, limit, err := planForm4Ingest(ctx, s, cik, sinceISO, DefaultMaxForm4)
	if err != nil {
		t.Fatalf("planForm4Ingest: %v", err)
	}
	if !skip.Truncated {
		t.Errorf("Truncated = false; want true (250 > %d cap)", DefaultMaxForm4)
	}
	if skip.TotalInWindow != total {
		t.Errorf("TotalInWindow = %d; want %d", skip.TotalInWindow, total)
	}
	if skip.MaxForm4Applied != DefaultMaxForm4 {
		t.Errorf("MaxForm4Applied = %d; want %d", skip.MaxForm4Applied, DefaultMaxForm4)
	}
	if limit != DefaultMaxForm4 {
		t.Errorf("limit = %d; want %d", limit, DefaultMaxForm4)
	}
}

// TestPlanForm4Ingest_NotTruncatedWhenBelowCap covers the clean-bill case:
// 50 filings, default 200 cap → Truncated=false, TotalInWindow=50.
func TestPlanForm4Ingest_NotTruncatedWhenBelowCap(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	if err := s.EnsureEdgarSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const cik = "0000000043"
	const total = 50
	sinceISO := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	seedForm4Filings(t, s, cik, total)

	skip, limit, err := planForm4Ingest(ctx, s, cik, sinceISO, DefaultMaxForm4)
	if err != nil {
		t.Fatalf("planForm4Ingest: %v", err)
	}
	if skip.Truncated {
		t.Errorf("Truncated = true; want false (50 <= %d cap)", DefaultMaxForm4)
	}
	if skip.TotalInWindow != total {
		t.Errorf("TotalInWindow = %d; want %d", skip.TotalInWindow, total)
	}
	if limit != DefaultMaxForm4 {
		t.Errorf("limit = %d; want %d", limit, DefaultMaxForm4)
	}
}

// TestPlanForm4Ingest_CapDisabled covers maxForm4=0 → cap disabled, no
// truncation flag even when totalInWindow is large.
func TestPlanForm4Ingest_CapDisabled(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	if err := s.EnsureEdgarSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const cik = "0000000044"
	const total = 250
	sinceISO := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	seedForm4Filings(t, s, cik, total)

	skip, limit, err := planForm4Ingest(ctx, s, cik, sinceISO, 0)
	if err != nil {
		t.Fatalf("planForm4Ingest: %v", err)
	}
	if skip.Truncated {
		t.Errorf("Truncated = true; want false (cap disabled)")
	}
	if limit != 0 {
		t.Errorf("limit = %d; want 0 (unlimited)", limit)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "edgar-test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s
}

// seedForm4Filings inserts n Form 4 filings for cik with filed_at spread
// across the past 11 months (well inside a 12-month window).
func seedForm4Filings(t *testing.T, s *store.Store, cik string, n int) {
	t.Helper()
	ctx := context.Background()
	base := time.Now().AddDate(0, -11, 0)
	for i := 0; i < n; i++ {
		filedAt := base.AddDate(0, 0, i%330).Format("2006-01-02")
		if err := s.UpsertEdgarFiling(ctx, store.EdgarFiling{
			Accession: fmt.Sprintf("%s-%05d", cik, i),
			CIK:       cik,
			FormType:  "4",
			FiledAt:   filedAt,
		}); err != nil {
			t.Fatalf("seed filing %d: %v", i, err)
		}
	}
}
