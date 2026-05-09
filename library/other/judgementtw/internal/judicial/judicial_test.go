// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package judicial

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"judgementtw-pp-cli/internal/extract"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	// Generator's judgments table — minimal stand-in so MaxJID joins work.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS judgments (id TEXT PRIMARY KEY, data TEXT)`); err != nil {
		t.Fatalf("judgments: %v", err)
	}
	return db
}

func TestEnsureSchemaIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Errorf("second EnsureSchema failed: %v", err)
	}
}

func TestIndexAndQueryCitations(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	jid1 := "TPSM,115,台抗,703,20260430,1"
	jid2 := "TPHM,110,毒抗,1212,20210831,1"
	jid3 := "TPDM,114,訴,500,20250101,1"

	if err := IndexCitations(ctx, db, jid1, []extract.Citation{
		{Statute: "刑法", Article: 50},
		{Statute: "毒品危害防制條例", Article: 17},
	}, []string{jid2}); err != nil {
		t.Fatalf("index1: %v", err)
	}
	if err := IndexCitations(ctx, db, jid3, []extract.Citation{
		{Statute: "毒品危害防制條例", Article: 17},
	}, []string{jid2}); err != nil {
		t.Fatalf("index3: %v", err)
	}

	counts, err := CountByStatute(ctx, db, "毒品危害防制條例", 17)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if len(counts) == 0 {
		t.Fatal("expected at least 1 row")
	}
	totals := 0
	for _, c := range counts {
		totals += c.Count
	}
	if totals != 2 {
		t.Errorf("expected 2 total citations of 毒品危害防制條例 §17, got %d (%+v)", totals, counts)
	}

	citers, err := CitedBy(ctx, db, jid2, 0)
	if err != nil {
		t.Fatalf("cited-by: %v", err)
	}
	if len(citers) != 2 {
		t.Errorf("expected 2 citers, got %d (%+v)", len(citers), citers)
	}

	got, err := CitationsOf(ctx, db, jid1)
	if err != nil {
		t.Fatalf("citations-of: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 citations for jid1, got %d", len(got))
	}
}

func TestReindexCitationsReplaces(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	jid := "TPSM,115,台抗,703,20260430,1"

	_ = IndexCitations(ctx, db, jid, []extract.Citation{
		{Statute: "刑法", Article: 50},
	}, nil)
	_ = IndexCitations(ctx, db, jid, []extract.Citation{
		{Statute: "民法", Article: 184},
	}, nil)

	got, _ := CitationsOf(ctx, db, jid)
	if len(got) != 1 || got[0].Statute != "民法" {
		t.Errorf("re-index should replace; got %+v", got)
	}
}

func TestSentenceAggregation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	jids := []string{
		"TPSM,115,台上,1,20260101,1",
		"TPSM,115,台上,2,20260102,1",
		"TPHM,115,毒抗,3,20260103,1",
	}
	statute := "毒品危害防制條例"
	for _, j := range jids {
		_ = IndexCitations(ctx, db, j, []extract.Citation{{Statute: statute, Article: 4}}, nil)
	}
	_ = IndexSentences(ctx, db, jids[0], []extract.Sentence{{Kind: extract.SentencePrison, PrisonMonths: 12}})
	_ = IndexSentences(ctx, db, jids[1], []extract.Sentence{{Kind: extract.SentencePrison, PrisonMonths: 24}})
	_ = IndexSentences(ctx, db, jids[2], []extract.Sentence{{Kind: extract.SentenceFine, FineNTD: 30000}})

	stats, err := AggregateSentences(ctx, db, statute, "", 0)
	if err != nil {
		t.Fatalf("agg: %v", err)
	}
	if stats.TotalCount != 3 {
		t.Errorf("total: got %d", stats.TotalCount)
	}
	if stats.PrisonCount != 2 {
		t.Errorf("prison count: got %d", stats.PrisonCount)
	}
	if stats.FineCount != 1 {
		t.Errorf("fine count: got %d", stats.FineCount)
	}
	if stats.PrisonMin != 12 || stats.PrisonMax != 24 {
		t.Errorf("min/max: got %d/%d", stats.PrisonMin, stats.PrisonMax)
	}

	// Filter by court
	tpsOnly, _ := AggregateSentences(ctx, db, statute, "TPS", 0)
	if tpsOnly.PrisonCount != 2 {
		t.Errorf("TPS-only prison: got %d", tpsOnly.PrisonCount)
	}
}

func TestWatchlistRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	q := map[string]any{"keyword": "毒品危害防制條例", "type": "criminal"}
	if err := SaveWatch(ctx, db, "drug-cases", WatchQuery, q); err != nil {
		t.Fatal(err)
	}
	got, err := GetWatch(ctx, db, "drug-cases")
	if err != nil || got == nil {
		t.Fatalf("get: %v / %+v", err, got)
	}
	if got.Kind != WatchQuery {
		t.Errorf("kind: got %q", got.Kind)
	}
	if err := UpdateWatchCursor(ctx, db, "drug-cases", "TPSM,115,台抗,703,20260430,1"); err != nil {
		t.Fatal(err)
	}
	got, _ = GetWatch(ctx, db, "drug-cases")
	if got.LastSeen == "" {
		t.Error("cursor not stored")
	}
	all, err := ListWatches(ctx, db)
	if err != nil || len(all) != 1 {
		t.Errorf("list: %v / %d", err, len(all))
	}
	if err := DeleteWatch(ctx, db, "drug-cases"); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetWatch(ctx, db, "drug-cases"); got != nil {
		t.Error("delete failed")
	}
}

func TestLogEvent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if err := LogEvent(ctx, db, "synced", "TPSM,115,台抗,703,20260430,1", "ok"); err != nil {
		t.Fatal(err)
	}
	row := db.QueryRow(`SELECT COUNT(*) FROM change_log`)
	var n int
	_ = row.Scan(&n)
	if n != 1 {
		t.Errorf("expected 1 row, got %d", n)
	}
}
