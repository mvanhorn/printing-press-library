// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func openMailTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedMailMeta writes 12 rows across two accounts:
//
//	ads (8 rows):
//	  news@letters.example  x4  promotions, 3 unread, sizes 1000/2000/3000/4000,
//	                             dates 1000..4000, all with List-Unsubscribe
//	  boss@work.example     x3  primary, 0 unread, sizes 10/20/30, dates 5000..7000
//	  lone@one.example      x1  "", unread, size 500, date 900
//	personal (4 rows):
//	  news@letters.example  x2  promotions, 1 unread
//	  friend@home.example   x2  primary, 0 unread
func seedMailMeta(t *testing.T, s *Store) {
	t.Helper()
	var rows []MailMeta
	for i := 1; i <= 4; i++ {
		rows = append(rows, MailMeta{
			Account: "ads", ID: fmt.Sprintf("n%d", i), ThreadID: "tn",
			FromEmail: "news@letters.example", FromName: "Letters Weekly",
			Subject: fmt.Sprintf("Newsletter %d", i), Snippet: "the weekly letters digest",
			InternalDate: int64(i * 1000), SizeEstimate: int64(i * 1000),
			LabelIDs: []string{"CATEGORY_PROMOTIONS", "UNREAD"}, Category: "promotions",
			ListUnsubscribe: "<https://letters.example/unsub>", ListUnsubscribePost: "List-Unsubscribe=One-Click",
			Unread:      i != 4, // 3 of 4 unread
			AuthResults: "mx.google.com; dkim=pass header.i=@letters.example",
		})
	}
	for i := 1; i <= 3; i++ {
		rows = append(rows, MailMeta{
			Account: "ads", ID: fmt.Sprintf("b%d", i), ThreadID: "tb",
			FromEmail: "boss@work.example", FromName: "The Boss",
			Subject: fmt.Sprintf("Standup %d", i), Snippet: "quick sync notes",
			InternalDate: int64(4000 + i*1000), SizeEstimate: int64(i * 10),
			LabelIDs: []string{"INBOX"}, Category: "primary",
		})
	}
	rows = append(rows, MailMeta{
		Account: "ads", ID: "l1",
		FromEmail: "lone@one.example", FromName: "",
		Subject: "archived thing", Snippet: "no category here",
		InternalDate: 900, SizeEstimate: 500,
		LabelIDs: []string{"UNREAD"}, Category: "", Unread: true,
	})
	for i := 1; i <= 2; i++ {
		rows = append(rows, MailMeta{
			Account: "personal", ID: fmt.Sprintf("pn%d", i),
			FromEmail: "news@letters.example", FromName: "Letters Weekly",
			Subject: "Newsletter", Snippet: "digest",
			InternalDate: int64(i * 100), SizeEstimate: 700,
			LabelIDs: []string{"CATEGORY_PROMOTIONS"}, Category: "promotions",
			ListUnsubscribe: "<mailto:unsub@letters.example>",
			Unread:          i == 1, // 1 of 2 unread
		})
	}
	for i := 1; i <= 2; i++ {
		rows = append(rows, MailMeta{
			Account: "personal", ID: fmt.Sprintf("pf%d", i),
			FromEmail: "friend@home.example", FromName: "A Friend",
			Subject: "hello", Snippet: "catching up",
			InternalDate: int64(1000 + i), SizeEstimate: 50,
			LabelIDs: []string{"INBOX"}, Category: "primary",
		})
	}
	n, err := s.UpsertMailMeta(rows)
	if err != nil {
		t.Fatalf("seed upsert: %v", err)
	}
	if n != 12 {
		t.Fatalf("seed wrote %d rows, want 12", n)
	}
}

func TestMailMetaUpsertUpdateDelete(t *testing.T) {
	s := openMailTestStore(t)
	seedMailMeta(t, s)

	if n, err := s.CountMailMeta("ads"); err != nil || n != 8 {
		t.Fatalf("ads count = %d, %v; want 8", n, err)
	}
	if n, err := s.CountMailMeta("personal"); err != nil || n != 4 {
		t.Fatalf("personal count = %d, %v; want 4", n, err)
	}

	// Seeded rows round-trip the new-column values exactly: auth_results as
	// written, list_unsub_domain defaulted to '' (never NULL).
	seeded, err := s.GetMailMeta("ads", "n1")
	if err != nil {
		t.Fatalf("get seeded n1: %v", err)
	}
	if seeded.AuthResults != "mx.google.com; dkim=pass header.i=@letters.example" {
		t.Fatalf("auth_results round trip wrong: %q", seeded.AuthResults)
	}
	if seeded.ListUnsubDomain != "" {
		t.Fatalf("list_unsub_domain should default to '': %q", seeded.ListUnsubDomain)
	}
	if b1, err := s.GetMailMeta("ads", "b1"); err != nil || b1.AuthResults != "" {
		t.Fatalf("unset auth_results should read back '': %+v, %v", b1, err)
	}

	// Re-upserting the same (account, id) must replace, not duplicate.
	if _, err := s.UpsertMailMeta([]MailMeta{{
		Account: "ads", ID: "n1", FromEmail: "news@letters.example", FromName: "Letters Weekly",
		Subject: "Newsletter 1 (edited)", Snippet: "edited snippet",
		InternalDate: 1000, SizeEstimate: 1111,
		LabelIDs: []string{"CATEGORY_PROMOTIONS"}, Category: "promotions",
		ListUnsubscribe: "<https://letters.example/unsub>", Unread: false,
		AuthResults:     "mx.google.com; dkim=fail",
		ListUnsubDomain: "letters.example",
	}}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if n, _ := s.CountMailMeta("ads"); n != 8 {
		t.Fatalf("count after re-upsert = %d, want 8 (duplicate row created)", n)
	}
	got, err := s.GetMailMeta("ads", "n1")
	if err != nil || got.SizeEstimate != 1111 || got.Unread || got.Subject != "Newsletter 1 (edited)" {
		t.Fatalf("re-upsert did not replace: %+v, %v", got, err)
	}
	if got.AuthResults != "mx.google.com; dkim=fail" || got.ListUnsubDomain != "letters.example" {
		t.Fatalf("re-upsert did not replace new columns: %+v", got)
	}
	// Restore n1 to its seeded shape so aggregate assertions below stay exact.
	seedMailMeta(t, s)

	// Label update: flip n2 read, move it out of promotions.
	found, err := s.UpdateMailLabels("ads", "n2", []string{"CATEGORY_UPDATES"}, false, "updates")
	if err != nil || !found {
		t.Fatalf("UpdateMailLabels: found=%v err=%v", found, err)
	}
	got, err = s.GetMailMeta("ads", "n2")
	if err != nil || got.Unread || got.Category != "updates" || len(got.LabelIDs) != 1 || got.LabelIDs[0] != "CATEGORY_UPDATES" {
		t.Fatalf("label update not applied: %+v, %v", got, err)
	}
	// Missing row reports found=false, no error.
	found, err = s.UpdateMailLabels("ads", "ghost", []string{"INBOX"}, true, "primary")
	if err != nil || found {
		t.Fatalf("UpdateMailLabels(ghost): found=%v err=%v; want false, nil", found, err)
	}
	// Put n2 back for the aggregation test.
	seedMailMeta(t, s)

	// FTS sees seeded content, scoped per account.
	hits, err := s.SearchMailMeta("ads", "standup", 10)
	if err != nil || len(hits) != 3 {
		t.Fatalf("FTS standup hits = %d, %v; want 3", len(hits), err)
	}
	if hits, _ := s.SearchMailMeta("personal", "standup", 10); len(hits) != 0 {
		t.Fatalf("FTS must not leak across accounts: %+v", hits)
	}

	// Delete two ads rows; only ads shrinks, FTS row disappears.
	deleted, err := s.DeleteMailMeta("ads", []string{"b1", "b2", "nosuch"})
	if err != nil || deleted != 2 {
		t.Fatalf("DeleteMailMeta = %d, %v; want 2", deleted, err)
	}
	if n, _ := s.CountMailMeta("ads"); n != 6 {
		t.Fatalf("ads count after delete = %d, want 6", n)
	}
	if n, _ := s.CountMailMeta("personal"); n != 4 {
		t.Fatalf("personal count after delete = %d, want 4", n)
	}
	if _, err := s.GetMailMeta("ads", "b1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted row still readable: %v", err)
	}
	if hits, _ := s.SearchMailMeta("ads", "standup", 10); len(hits) != 1 {
		t.Fatalf("FTS rows not deleted with mail_meta rows: %d hits", len(hits))
	}
}

func TestSenderStatsAggregation(t *testing.T) {
	s := openMailTestStore(t)
	seedMailMeta(t, s)

	stats, err := s.SenderStats("ads", "", 0, 25)
	if err != nil {
		t.Fatalf("SenderStats: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("ads sender rows = %d, want 3 (%+v)", len(stats), stats)
	}
	// Sorted by count desc: news (4), boss (3), lone (1).
	news, boss, lone := stats[0], stats[1], stats[2]
	if news.FromEmail != "news@letters.example" || news.Count != 4 {
		t.Fatalf("top sender wrong: %+v", news)
	}
	if news.TotalSize != 10000 {
		t.Fatalf("news total_size = %d, want 10000", news.TotalSize)
	}
	if news.UnreadCount != 3 || news.UnreadRate != 0.75 {
		t.Fatalf("news unread = %d rate %v, want 3 / 0.75", news.UnreadCount, news.UnreadRate)
	}
	if news.FirstSeen != 1000 || news.LastSeen != 4000 {
		t.Fatalf("news first/last = %d/%d, want 1000/4000", news.FirstSeen, news.LastSeen)
	}
	if !news.HasUnsubscribe || news.FromName != "Letters Weekly" {
		t.Fatalf("news unsub/name wrong: %+v", news)
	}
	if boss.FromEmail != "boss@work.example" || boss.Count != 3 || boss.UnreadCount != 0 || boss.UnreadRate != 0 {
		t.Fatalf("boss row wrong: %+v", boss)
	}
	if boss.TotalSize != 60 || boss.HasUnsubscribe {
		t.Fatalf("boss size/unsub wrong: %+v", boss)
	}
	if lone.FromEmail != "lone@one.example" || lone.Count != 1 || lone.UnreadRate != 1.0 {
		t.Fatalf("lone row wrong: %+v", lone)
	}

	// Account isolation: personal aggregates only its 4 rows.
	pstats, err := s.SenderStats("personal", "", 0, 25)
	if err != nil || len(pstats) != 2 {
		t.Fatalf("personal senders = %d, %v; want 2", len(pstats), err)
	}
	if pstats[0].Count != 2 || pstats[1].Count != 2 {
		t.Fatalf("personal counts wrong: %+v", pstats)
	}
	for _, st := range pstats {
		if st.FromEmail == "news@letters.example" && (st.UnreadCount != 1 || st.UnreadRate != 0.5) {
			t.Fatalf("personal news unread wrong: %+v", st)
		}
	}

	// Category filter.
	promo, err := s.SenderStats("ads", "promotions", 0, 25)
	if err != nil || len(promo) != 1 || promo[0].FromEmail != "news@letters.example" {
		t.Fatalf("category filter wrong: %+v, %v", promo, err)
	}

	// Since filter: only rows with internal_date >= 5000 (boss's 3).
	recent, err := s.SenderStats("ads", "", 5000, 25)
	if err != nil || len(recent) != 1 || recent[0].FromEmail != "boss@work.example" || recent[0].Count != 3 {
		t.Fatalf("since filter wrong: %+v, %v", recent, err)
	}

	// Top limit.
	top1, err := s.SenderStats("ads", "", 0, 1)
	if err != nil || len(top1) != 1 || top1[0].FromEmail != "news@letters.example" {
		t.Fatalf("top limit wrong: %+v, %v", top1, err)
	}
}

// A mail_meta table created before auth_results/list_unsub_domain shipped
// must gain both columns on the next Open (CREATE TABLE IF NOT EXISTS alone
// would silently leave the old shape and every read/write would fail).
func TestMailMetaColumnBackfill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE mail_meta (
		account TEXT NOT NULL,
		id TEXT NOT NULL,
		thread_id TEXT NOT NULL DEFAULT '',
		from_email TEXT NOT NULL DEFAULT '',
		from_name TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL DEFAULT '',
		snippet TEXT NOT NULL DEFAULT '',
		internal_date INTEGER NOT NULL DEFAULT 0,
		size_estimate INTEGER NOT NULL DEFAULT 0,
		label_ids TEXT NOT NULL DEFAULT '[]',
		category TEXT NOT NULL DEFAULT '',
		list_unsubscribe TEXT NOT NULL DEFAULT '',
		list_unsubscribe_post TEXT NOT NULL DEFAULT '',
		unread INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (account, id)
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(
		`INSERT INTO mail_meta (account, id, from_email) VALUES ('ads', 'old1', 'x@y.z')`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open over old-shape table: %v", err)
	}
	defer s.Close()

	// Old row reads back with the backfilled defaults.
	got, err := s.GetMailMeta("ads", "old1")
	if err != nil || got.FromEmail != "x@y.z" || got.AuthResults != "" || got.ListUnsubDomain != "" {
		t.Fatalf("old row after backfill: %+v, %v", got, err)
	}
	// New-shape writes land.
	if _, err := s.UpsertMailMeta([]MailMeta{{
		Account: "ads", ID: "new1", FromEmail: "a@b.c", AuthResults: "mx; dkim=pass",
	}}); err != nil {
		t.Fatalf("upsert after backfill: %v", err)
	}
	if got, err := s.GetMailMeta("ads", "new1"); err != nil || got.AuthResults != "mx; dkim=pass" {
		t.Fatalf("new row after backfill: %+v, %v", got, err)
	}
}

func TestMailSyncStateRoundTrip(t *testing.T) {
	s := openMailTestStore(t)

	// Missing row: zero state, no error.
	st, err := s.GetMailSyncState("ads")
	if err != nil || st.HistoryID != "" || st.LastFullSync != "" || st.LastIncremental != "" {
		t.Fatalf("empty state wrong: %+v, %v", st, err)
	}

	if err := s.SaveMailSyncState("ads", "12345", "full"); err != nil {
		t.Fatalf("save full: %v", err)
	}
	st, err = s.GetMailSyncState("ads")
	if err != nil || st.HistoryID != "12345" || st.LastFullSync == "" || st.LastIncremental != "" {
		t.Fatalf("after full: %+v, %v", st, err)
	}
	fullStamp := st.LastFullSync

	if err := s.SaveMailSyncState("ads", "12399", "incremental"); err != nil {
		t.Fatalf("save incremental: %v", err)
	}
	st, err = s.GetMailSyncState("ads")
	if err != nil || st.HistoryID != "12399" || st.LastIncremental == "" {
		t.Fatalf("after incremental: %+v, %v", st, err)
	}
	if st.LastFullSync != fullStamp {
		t.Fatalf("incremental save clobbered last_full_sync: %q -> %q", fullStamp, st.LastFullSync)
	}

	if err := s.SaveMailSyncState("ads", "1", "bogus"); err == nil {
		t.Fatal("bogus mode accepted")
	}

	// Per-account isolation.
	if st, _ := s.GetMailSyncState("personal"); st.HistoryID != "" {
		t.Fatalf("cross-account state leak: %+v", st)
	}
}
