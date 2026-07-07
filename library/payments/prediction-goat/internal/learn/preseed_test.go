// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// openPreseedStore opens a fresh store at a temp path so each preseed
// test runs against an isolated DB. Mirrors openRecallStore so the two
// suites use the same store-lifecycle shape.
func openPreseedStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "preseed.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// staticScanner is a test scanner that ignores db and returns a
// preconfigured row slice. Keeps each test's setup obvious at the
// callsite — no fixture file, no SQL dance.
func staticScanner(rows []learn.PreseedRow, err error) learn.ScannerFn {
	return func(_ context.Context, _ *sql.DB) ([]learn.PreseedRow, error) {
		return rows, err
	}
}

func TestPreseedRun_EmptyRegistry_NoOp(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)
	n, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 0 {
		t.Errorf("Run returned %d, want 0 on empty registry", n)
	}
}

func TestPreseedRun_TwoScanners_UpsertsAllRows(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	learn.RegisterScanner("kalshi_markets", staticScanner([]learn.PreseedRow{
		{QueryPattern: "odds Portugal wins world cup", ResourceID: "KX-PT", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"Portugal"}},
		{QueryPattern: "odds USA wins world cup", ResourceID: "KX-US", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"USA"}},
		{QueryPattern: "odds England wins world cup", ResourceID: "KX-EN", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"England"}},
	}, nil))
	learn.RegisterScanner("events", staticScanner([]learn.PreseedRow{
		{QueryPattern: "will Lakers win finals", ResourceID: "poly-lal", ResourceType: "events", Venue: "polymarket", Entities: []string{"Lakers"}},
		{QueryPattern: "will Celtics win finals", ResourceID: "poly-bos", ResourceType: "events", Venue: "polymarket", Entities: []string{"Celtics"}},
		{QueryPattern: "will Warriors win finals", ResourceID: "poly-gsw", ResourceType: "events", Venue: "polymarket", Entities: []string{"Warriors"}},
	}, nil))

	n, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 6 {
		t.Errorf("Run inserted %d rows, want 6", n)
	}

	rows, err := s.ListLearnings(store.ListLearningsFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListLearnings: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("ListLearnings returned %d rows, want 6", len(rows))
	}
	for _, r := range rows {
		if r.Source != learn.SourcePreseed {
			t.Errorf("row %s/%s source = %q, want %q", r.ResourceType, r.ResourceID, r.Source, learn.SourcePreseed)
		}
		if r.Confidence != 2 {
			t.Errorf("row %s/%s confidence = %d, want 2 (U4 floor)", r.ResourceType, r.ResourceID, r.Confidence)
		}
	}
}

func TestPreseedRun_DuplicateRowsWithinScanner_Deduped(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	// Same (query, resource) triple twice plus one distinct entry.
	learn.RegisterScanner("kalshi_markets", staticScanner([]learn.PreseedRow{
		{QueryPattern: "odds USA wins world cup", ResourceID: "KX-US", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"USA"}},
		{QueryPattern: "odds USA wins world cup", ResourceID: "KX-US", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"USA"}},
		{QueryPattern: "odds Portugal wins world cup", ResourceID: "KX-PT", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"Portugal"}},
	}, nil))

	n, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 2 {
		t.Errorf("Run inserted %d rows, want 2 (dup collapsed)", n)
	}
}

func TestPreseedRun_RerunIsNoOp(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	learn.RegisterScanner("kalshi_markets", staticScanner([]learn.PreseedRow{
		{QueryPattern: "odds USA wins world cup", ResourceID: "KX-US", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"USA"}},
	}, nil))

	n1, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	if n1 != 1 {
		t.Errorf("Run 1 inserted %d, want 1", n1)
	}

	n2, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if n2 != 0 {
		t.Errorf("Run 2 inserted %d, want 0 (re-run should be no-op)", n2)
	}

	// Confirm confidence did not bump on the re-run.
	rows, err := s.ListLearnings(store.ListLearningsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListLearnings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListLearnings returned %d rows, want 1", len(rows))
	}
	if rows[0].Confidence != 2 {
		t.Errorf("confidence after re-run = %d, want 2 (no bump on preseed/preseed)", rows[0].Confidence)
	}
}

func TestPreseedRun_TaughtRowPreserved(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	// User taught a high-confidence row first.
	if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
		Query:        "odds USA wins world cup",
		ResourceID:   "KXMENWORLDCUP-26-US",
		ResourceType: "kalshi_markets",
		Venue:        "kalshi",
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("seed taught: %v", err)
	}
	// Bump it twice so confidence lands above the preseed floor.
	for i := 0; i < 2; i++ {
		if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
			Query:        "odds USA wins world cup",
			ResourceID:   "KXMENWORLDCUP-26-US",
			ResourceType: "kalshi_markets",
			Venue:        "kalshi",
			Source:       store.LearningSourceTaught,
		}); err != nil {
			t.Fatalf("bump taught: %v", err)
		}
	}

	before, err := s.ListLearnings(store.ListLearningsFilter{Limit: 10})
	if err != nil || len(before) != 1 {
		t.Fatalf("seed state: %v / %d rows", err, len(before))
	}
	if before[0].Confidence < 3 {
		t.Fatalf("seed confidence = %d, want >= 3", before[0].Confidence)
	}
	originalConfidence := before[0].Confidence

	// Now preseed emits the same (query, resource).
	learn.RegisterScanner("kalshi_markets", staticScanner([]learn.PreseedRow{
		{QueryPattern: "odds USA wins world cup", ResourceID: "KXMENWORLDCUP-26-US", ResourceType: "kalshi_markets", Venue: "kalshi", Entities: []string{"USA"}},
	}, nil))
	n, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 0 {
		t.Errorf("preseed inserted %d, want 0 over a taught row", n)
	}

	after, err := s.ListLearnings(store.ListLearningsFilter{Limit: 10})
	if err != nil || len(after) != 1 {
		t.Fatalf("post state: %v / %d rows", err, len(after))
	}
	if after[0].Source != store.LearningSourceTaught {
		t.Errorf("source = %q, want taught (user signal must win)", after[0].Source)
	}
	if after[0].Confidence != originalConfidence {
		t.Errorf("confidence after preseed = %d, want %d (no bump from preseed)", after[0].Confidence, originalConfidence)
	}
}

func TestPreseedRun_ScannerErrorAggregatedNotFatal(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	scannerErr := errors.New("synthetic scanner failure")
	learn.RegisterScanner("kalshi_markets", staticScanner(nil, scannerErr))
	learn.RegisterScanner("events", staticScanner([]learn.PreseedRow{
		{QueryPattern: "will Lakers win finals", ResourceID: "poly-lal", ResourceType: "events", Venue: "polymarket", Entities: []string{"Lakers"}},
	}, nil))

	n, err := learn.Run(context.Background(), s.DB())
	if err == nil {
		t.Fatalf("Run should report aggregated scanner error, got nil")
	}
	if !errors.Is(err, scannerErr) {
		t.Errorf("Run error chain should include scanner error; got %v", err)
	}
	if n != 1 {
		t.Errorf("Run inserted %d, want 1 (scanner error must not block sibling scanner)", n)
	}
}

func TestPreseedRun_DisabledByEnv(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	called := false
	learn.RegisterScanner("kalshi_markets", func(_ context.Context, _ *sql.DB) ([]learn.PreseedRow, error) {
		called = true
		return []learn.PreseedRow{
			{QueryPattern: "odds USA wins world cup", ResourceID: "KX-US", ResourceType: "kalshi_markets"},
		}, nil
	})

	t.Setenv("PRESEED_DISABLED", "1")
	n, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 0 {
		t.Errorf("Run inserted %d, want 0 when PRESEED_DISABLED is set", n)
	}
	if called {
		t.Errorf("scanner should not be invoked when PRESEED_DISABLED is set")
	}
}

func TestPreseedRun_RowCapTruncates(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)

	rows := make([]learn.PreseedRow, 5)
	for i := range rows {
		rows[i] = learn.PreseedRow{
			QueryPattern: "pattern" + string(rune('A'+i)),
			ResourceID:   "R" + string(rune('A'+i)),
			ResourceType: "kalshi_markets",
		}
	}
	learn.RegisterScanner("kalshi_markets", staticScanner(rows, nil))

	n, err := learn.RunWith(context.Background(), s.DB(), learn.RunOpts{RowCap: 3})
	if err != nil {
		t.Fatalf("RunWith: %v", err)
	}
	if n != 3 {
		t.Errorf("RunWith with cap=3 inserted %d, want 3", n)
	}
}

func TestPreseedRun_EmptyResourceIDSkipped(t *testing.T) {
	learn.ResetScannersForTest()
	s := openPreseedStore(t)
	learn.RegisterScanner("kalshi_markets", staticScanner([]learn.PreseedRow{
		{QueryPattern: "odds USA wins world cup", ResourceID: "", ResourceType: "kalshi_markets"},
		{QueryPattern: "odds Portugal wins world cup", ResourceID: "KX-PT", ResourceType: "kalshi_markets", Entities: []string{"Portugal"}},
	}, nil))

	n, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 1 {
		t.Errorf("Run inserted %d, want 1 (empty resource_id should skip)", n)
	}
}
