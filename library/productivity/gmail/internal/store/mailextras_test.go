// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for the unsubscribe-ledger, checkpoint, score, and novel-read
// aggregation layer (mailextras.go). Seeded stores, exact-value asserts.

package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openExtrasTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedMeta(t *testing.T, s *Store, rows []MailMeta) {
	t.Helper()
	n, err := s.UpsertMailMeta(rows)
	if err != nil || n != len(rows) {
		t.Fatalf("seed upsert = (%d, %v), want (%d, nil)", n, err, len(rows))
	}
}

const dayMs = int64(24 * 60 * 60 * 1000)

func TestMailUnsubLedgerLifecycleAndViolations(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	now := time.Now().UTC()

	// Sender A: 2xx post 5 days ago, then two arrivals after the 2-day grace.
	postedA := now.Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	idA, err := s.InsertMailUnsubAttempt(MailUnsubAttempt{
		Account: "acct", Sender: "a@list.example", URL: "https://u.list.example/x",
		PlanSha: "sha-a", PostedAt: postedA, Status: "unknown",
	})
	if err != nil || idA <= 0 {
		t.Fatalf("insert = (%d, %v)", idA, err)
	}
	if err := s.SetMailUnsubAttemptStatus(idA, "200"); err != nil {
		t.Fatal(err)
	}
	// Sender B: posted but got a 500 — must NOT count as posted for verify.
	if _, err := s.InsertMailUnsubAttempt(MailUnsubAttempt{
		Account: "acct", Sender: "b@list.example", URL: "https://u.list.example/b",
		PostedAt: postedA, Status: "500",
	}); err != nil {
		t.Fatal(err)
	}
	// Sender C: 2xx post, but only arrivals INSIDE the grace window.
	if _, err := s.InsertMailUnsubAttempt(MailUnsubAttempt{
		Account: "acct", Sender: "c@list.example", URL: "https://u.list.example/c",
		PostedAt: postedA, Status: "204",
	}); err != nil {
		t.Fatal(err)
	}

	nowMs := now.UnixMilli()
	seedMeta(t, s, []MailMeta{
		// A: one arrival inside grace (ignored), two after grace.
		{Account: "acct", ID: "a1", FromEmail: "a@list.example", Subject: "in-grace", InternalDate: nowMs - 4*dayMs},
		{Account: "acct", ID: "a2", FromEmail: "a@list.example", Subject: "violation-1", InternalDate: nowMs - 2*dayMs},
		{Account: "acct", ID: "a3", FromEmail: "a@list.example", Subject: "violation-2", InternalDate: nowMs - 1*dayMs},
		// B arrivals (ignored: no 2xx post).
		{Account: "acct", ID: "b1", FromEmail: "b@list.example", Subject: "b", InternalDate: nowMs - 1*dayMs},
		// C: only in-grace arrivals.
		{Account: "acct", ID: "c1", FromEmail: "c@list.example", Subject: "c", InternalDate: nowMs - 4*dayMs},
	})

	got, err := s.UnsubViolations("acct", now.Add(-30*24*time.Hour), 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("violations = %+v, want exactly sender A", got)
	}
	v := got[0]
	if v.Sender != "a@list.example" || v.ArrivalsSince != 2 || v.NewestSubject != "violation-2" {
		t.Fatalf("violation = %+v, want a@list.example / 2 arrivals / newest violation-2", v)
	}
	if v.NewestDateMs != nowMs-1*dayMs {
		t.Fatalf("NewestDateMs = %d, want %d", v.NewestDateMs, nowMs-1*dayMs)
	}
	// A postedSince window that excludes the post drops the violation.
	got, err = s.UnsubViolations("acct", now.Add(-24*time.Hour), 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("violations outside window = %+v, want none", got)
	}
	// Ledger listing round-trips rows in insert order.
	attempts, err := s.ListMailUnsubAttempts("acct")
	if err != nil || len(attempts) != 3 {
		t.Fatalf("attempts = (%d, %v), want 3", len(attempts), err)
	}
	if attempts[0].Status != "200" || attempts[1].Status != "500" || attempts[2].Status != "204" {
		t.Fatalf("statuses = %s/%s/%s", attempts[0].Status, attempts[1].Status, attempts[2].Status)
	}
}

func TestMailCheckpointsRoundTrip(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	_, ok, err := s.GetMailCheckpoint("acct", "delta")
	if err != nil || ok {
		t.Fatalf("missing checkpoint = (ok=%v, %v), want absent", ok, err)
	}
	if err := s.SaveMailCheckpoint(MailCheckpoint{Account: "acct", Kind: "delta", WatermarkMs: 111, MsgCount: 5}); err != nil {
		t.Fatal(err)
	}
	cp, ok, err := s.GetMailCheckpoint("acct", "delta")
	if err != nil || !ok || cp.WatermarkMs != 111 || cp.MsgCount != 5 || cp.TakenAt == "" {
		t.Fatalf("checkpoint = (%+v, ok=%v, %v)", cp, ok, err)
	}
	// Upsert advances in place.
	if err := s.SaveMailCheckpoint(MailCheckpoint{Account: "acct", Kind: "delta", WatermarkMs: 222, MsgCount: 9}); err != nil {
		t.Fatal(err)
	}
	cp, _, _ = s.GetMailCheckpoint("acct", "delta")
	if cp.WatermarkMs != 222 || cp.MsgCount != 9 {
		t.Fatalf("advanced checkpoint = %+v, want 222/9", cp)
	}

	seedMeta(t, s, []MailMeta{
		{Account: "acct", ID: "m1", FromEmail: "x@y", InternalDate: 100},
		{Account: "acct", ID: "m2", FromEmail: "x@y", InternalDate: 300},
	})
	maxMs, count, err := s.MailWatermark("acct")
	if err != nil || maxMs != 300 || count != 2 {
		t.Fatalf("watermark = (%d, %d, %v), want (300, 2)", maxMs, count, err)
	}
	maxMs, count, err = s.MailWatermark("empty")
	if err != nil || maxMs != 0 || count != 0 {
		t.Fatalf("empty watermark = (%d, %d, %v), want zeros", maxMs, count, err)
	}
}

func TestMailScoresFirstAndLatest(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	if _, ok, err := s.LatestMailScore("acct"); err != nil || ok {
		t.Fatalf("latest on empty = ok=%v err=%v", ok, err)
	}
	if _, err := s.InsertMailScore("acct", "2026-01-01T00:00:00Z", `{"unread_pct":50}`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertMailScore("acct", "2026-02-01T00:00:00Z", `{"unread_pct":25}`); err != nil {
		t.Fatal(err)
	}
	first, ok, err := s.FirstMailScore("acct")
	if err != nil || !ok || first.TakenAt != "2026-01-01T00:00:00Z" || first.Metrics != `{"unread_pct":50}` {
		t.Fatalf("first = %+v ok=%v err=%v", first, ok, err)
	}
	latest, ok, err := s.LatestMailScore("acct")
	if err != nil || !ok || latest.TakenAt != "2026-02-01T00:00:00Z" {
		t.Fatalf("latest = %+v ok=%v err=%v", latest, ok, err)
	}
}

func TestCategoryDigestAndTopSenders(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	base := int64(1_700_000_000_000)
	seedMeta(t, s, []MailMeta{
		{Account: "acct", ID: "p1", FromEmail: "promo@a", Category: "promotions", Unread: true, SizeEstimate: 100, InternalDate: base + 1},
		{Account: "acct", ID: "p2", FromEmail: "promo@a", Category: "promotions", Unread: false, SizeEstimate: 200, InternalDate: base + 2},
		{Account: "acct", ID: "p3", FromEmail: "promo@b", Category: "promotions", Unread: true, SizeEstimate: 300, InternalDate: base + 3},
		{Account: "acct", ID: "u1", FromEmail: "up@c", Category: "updates", Unread: false, SizeEstimate: 50, InternalDate: base + 4},
		{Account: "acct", ID: "n1", FromEmail: "no@cat", Category: "", Unread: true, SizeEstimate: 10, InternalDate: base + 5},
	})
	rows, err := s.CategoryDigest("acct", 0)
	if err != nil {
		t.Fatal(err)
	}
	byCat := map[string]CategoryDigestRow{}
	for _, r := range rows {
		byCat[r.Category] = r
	}
	p := byCat["promotions"]
	if p.Total != 3 || p.Unread != 2 || p.TotalSize != 600 || p.OldestUnreadMs != base+1 {
		t.Fatalf("promotions digest = %+v", p)
	}
	u := byCat["updates"]
	if u.Total != 1 || u.Unread != 0 || u.OldestUnreadMs != 0 || u.TotalSize != 50 {
		t.Fatalf("updates digest = %+v", u)
	}
	if byCat[""].Total != 1 || byCat[""].Unread != 1 {
		t.Fatalf("uncategorized digest = %+v", byCat[""])
	}
	// Since-bound excludes older rows.
	rows, err = s.CategoryDigest("acct", base+3)
	if err != nil {
		t.Fatal(err)
	}
	byCat = map[string]CategoryDigestRow{}
	for _, r := range rows {
		byCat[r.Category] = r
	}
	if byCat["promotions"].Total != 1 || byCat["promotions"].TotalSize != 300 {
		t.Fatalf("since-bounded promotions = %+v", byCat["promotions"])
	}

	top, err := s.CategoryTopSenders("acct", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(top["promotions"]) != 1 || top["promotions"][0].FromEmail != "promo@a" || top["promotions"][0].Count != 2 {
		t.Fatalf("top promotions = %+v, want promo@a count 2", top["promotions"])
	}
}

func TestDeltaSenderAndCategoryStats(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	wm := int64(1_700_000_000_000)
	seedMeta(t, s, []MailMeta{
		// spiky: 10 prior messages over 10 days, 4 since.
		{Account: "acct", ID: "s1", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 10*dayMs},
		{Account: "acct", ID: "s2", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 9*dayMs},
		{Account: "acct", ID: "s3", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 8*dayMs},
		{Account: "acct", ID: "s4", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 7*dayMs},
		{Account: "acct", ID: "s5", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 6*dayMs},
		{Account: "acct", ID: "s6", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 5*dayMs},
		{Account: "acct", ID: "s7", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 4*dayMs},
		{Account: "acct", ID: "s8", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 3*dayMs},
		{Account: "acct", ID: "s9", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm - 2*dayMs},
		{Account: "acct", ID: "s10", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm},
		{Account: "acct", ID: "s11", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm + 1},
		{Account: "acct", ID: "s12", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm + 2},
		{Account: "acct", ID: "s13", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm + 3},
		{Account: "acct", ID: "s14", FromEmail: "spiky@x", Category: "promotions", InternalDate: wm + 4},
		// brand new sender since the watermark.
		{Account: "acct", ID: "n1", FromEmail: "new@x", Category: "primary", InternalDate: wm + 5},
		// old sender with no new activity: excluded entirely.
		{Account: "acct", ID: "o1", FromEmail: "old@x", Category: "primary", InternalDate: wm - dayMs},
	})
	stats, err := s.DeltaSenderStats("acct", wm)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("stats = %+v, want spiky + new only", stats)
	}
	if stats[0].FromEmail != "spiky@x" || stats[0].SinceCount != 4 || stats[0].PriorCount != 10 || stats[0].FirstSeenMs != wm-10*dayMs {
		t.Fatalf("spiky stats = %+v", stats[0])
	}
	if stats[1].FromEmail != "new@x" || stats[1].SinceCount != 1 || stats[1].PriorCount != 0 {
		t.Fatalf("new-sender stats = %+v", stats[1])
	}
	cats, err := s.DeltaCategoryCounts("acct", wm)
	if err != nil {
		t.Fatal(err)
	}
	if cats["promotions"] != 4 || cats["primary"] != 1 || len(cats) != 2 {
		t.Fatalf("category counts = %+v", cats)
	}
}

func TestStorageAttribution(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	y2023 := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	y2024 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	seedMeta(t, s, []MailMeta{
		{Account: "acct", ID: "m1", FromEmail: "big@x", Subject: "huge", Category: "promotions", SizeEstimate: 5000, InternalDate: y2023},
		{Account: "acct", ID: "m2", FromEmail: "big@x", Subject: "large", Category: "promotions", SizeEstimate: 3000, InternalDate: y2024},
		{Account: "acct", ID: "m3", FromEmail: "small@x", Subject: "tiny", Category: "updates", SizeEstimate: 100, InternalDate: y2024},
	})
	bySender, err := s.StorageBySender("acct", 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(bySender) != 2 || bySender[0].FromEmail != "big@x" || bySender[0].TotalSize != 8000 || bySender[0].Count != 2 {
		t.Fatalf("bySender = %+v", bySender)
	}
	byCat, err := s.StorageByCategory("acct")
	if err != nil {
		t.Fatal(err)
	}
	if byCat[0].Category != "promotions" || byCat[0].TotalSize != 8000 || byCat[1].Category != "updates" || byCat[1].TotalSize != 100 {
		t.Fatalf("byCategory = %+v", byCat)
	}
	byYear, err := s.StorageByYear("acct")
	if err != nil {
		t.Fatal(err)
	}
	if len(byYear) != 2 || byYear[0].Year != 2024 || byYear[0].TotalSize != 3100 || byYear[1].Year != 2023 || byYear[1].TotalSize != 5000 {
		t.Fatalf("byYear = %+v", byYear)
	}
	largest, err := s.StorageLargest("acct", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(largest) != 2 || largest[0].ID != "m1" || largest[0].SizeEstimate != 5000 || largest[1].ID != "m2" {
		t.Fatalf("largest = %+v", largest)
	}
}

func TestSenderLabelStats(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	seedMeta(t, s, []MailMeta{
		// receipts@x: 5 labeled Label_7, 1 labeled Label_9, 2 unlabeled.
		{Account: "acct", ID: "r1", FromEmail: "receipts@x", LabelIDs: []string{"INBOX", "Label_7"}},
		{Account: "acct", ID: "r2", FromEmail: "receipts@x", LabelIDs: []string{"Label_7", "UNREAD"}},
		{Account: "acct", ID: "r3", FromEmail: "receipts@x", LabelIDs: []string{"Label_7", "CATEGORY_UPDATES"}},
		{Account: "acct", ID: "r4", FromEmail: "receipts@x", LabelIDs: []string{"Label_7"}},
		{Account: "acct", ID: "r5", FromEmail: "receipts@x", LabelIDs: []string{"Label_7"}},
		{Account: "acct", ID: "r6", FromEmail: "receipts@x", LabelIDs: []string{"Label_9"}},
		{Account: "acct", ID: "r7", FromEmail: "receipts@x", LabelIDs: []string{"INBOX"}},
		{Account: "acct", ID: "r8", FromEmail: "receipts@x", LabelIDs: []string{"INBOX", "IMPORTANT", "STARRED"}},
		// onlysystem@x: system labels only — no user-label rows at all.
		{Account: "acct", ID: "o1", FromEmail: "onlysystem@x", LabelIDs: []string{"INBOX", "TRASH", "SPAM", "SENT", "DRAFT", "CATEGORY_SOCIAL"}},
	})
	stats, err := s.SenderLabelStats("acct")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("stats = %+v, want exactly the two receipts@x user labels", stats)
	}
	byLabel := map[string]SenderLabelStat{}
	for _, st := range stats {
		if st.FromEmail != "receipts@x" {
			t.Fatalf("unexpected sender row: %+v", st)
		}
		byLabel[st.Label] = st
	}
	l7 := byLabel["Label_7"]
	if l7.LabelCount != 5 || l7.LabeledTotal != 6 || l7.SenderTotal != 8 {
		t.Fatalf("Label_7 stats = %+v, want 5/6/8", l7)
	}
	l9 := byLabel["Label_9"]
	if l9.LabelCount != 1 || l9.LabeledTotal != 6 || l9.SenderTotal != 8 {
		t.Fatalf("Label_9 stats = %+v, want 1/6/8", l9)
	}
}

func TestTrashLedgersAndOutsideCount(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	if err := s.CreateMailLedger(MailLedger{LedgerID: "led1", Account: "acct", PlanSha: "sha1", Action: "trash", CreatedAt: "2026-08-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertMailLedgerEntries([]MailLedgerEntry{
		{LedgerID: "led1", ID: "t1", Kind: "trash", DeltaAdd: []string{"TRASH"}, PrePlacement: []string{"INBOX"}},
		{LedgerID: "led1", ID: "t2", Kind: "trash", DeltaAdd: []string{"TRASH"}},
		{LedgerID: "led1", ID: "t3", Kind: "trash", DeltaAdd: []string{"TRASH"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMailLedgerEntryUndone("led1", "t2", "undone"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMailLedgerEntryUndone("led1", "t3", "conflict"); err != nil {
		t.Fatal(err)
	}
	// A label-only ledger must not appear.
	if err := s.CreateMailLedger(MailLedger{LedgerID: "led2", Account: "acct", Action: "label"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertMailLedgerEntries([]MailLedgerEntry{
		{LedgerID: "led2", ID: "l1", Kind: "label", DeltaAdd: []string{"Label_1"}},
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := s.TrashLedgers("acct")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("trash ledgers = %+v, want just led1", rows)
	}
	r := rows[0]
	if r.LedgerID != "led1" || r.Trashed != 3 || r.Undone != 1 || r.Conflict != 1 || r.CreatedAt != "2026-08-01T00:00:00Z" {
		t.Fatalf("ledger row = %+v", r)
	}

	seedMeta(t, s, []MailMeta{
		{Account: "acct", ID: "t1", FromEmail: "x@y", LabelIDs: []string{"TRASH"}},          // ledgered
		{Account: "acct", ID: "ext1", FromEmail: "x@y", LabelIDs: []string{"TRASH"}},        // outside
		{Account: "acct", ID: "ext2", FromEmail: "x@y", LabelIDs: []string{"TRASH", "..."}}, // outside
		{Account: "acct", ID: "keep", FromEmail: "x@y", LabelIDs: []string{"INBOX"}},
	})
	n, err := s.TrashOutsideLedgerCount("acct")
	if err != nil || n != 2 {
		t.Fatalf("outside count = (%d, %v), want 2", n, err)
	}
}

func TestScoreAggregates(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	base := int64(1_700_000_000_000)
	seedMeta(t, s, []MailMeta{
		{Account: "acct", ID: "m1", FromEmail: "a@x", Category: "promotions", Unread: true, SizeEstimate: 100, InternalDate: base, ListUnsubscribe: "<https://u.x/1>"},
		{Account: "acct", ID: "m2", FromEmail: "a@x", Category: "promotions", Unread: false, SizeEstimate: 200, InternalDate: base + 1, ListUnsubscribe: "<https://u.x/1>"},
		{Account: "acct", ID: "m3", FromEmail: "b@y", Category: "primary", Unread: true, SizeEstimate: 300, InternalDate: base + 2, ListUnsubscribe: "<https://u.y/2>"},
		{Account: "acct", ID: "m4", FromEmail: "c@z", Category: "updates", Unread: false, SizeEstimate: 400, InternalDate: base + 3},
	})
	a, err := s.ScoreAggregates("acct")
	if err != nil {
		t.Fatal(err)
	}
	want := ScoreAggregates{Total: 4, Unread: 2, Promotions: 2, SubscriptionSenders: 2, TotalSize: 1000, OldestUnreadMs: base}
	if a != want {
		t.Fatalf("aggregates = %+v, want %+v", a, want)
	}
}

func TestUnsubSenderAggregatesAndNewestMeta(t *testing.T) {
	t.Parallel()
	s := openExtrasTestStore(t)
	base := int64(1_700_000_000_000)
	seedMeta(t, s, []MailMeta{
		{Account: "acct", ID: "l1", FromEmail: "list@x", Unread: true, InternalDate: base + 1, ListUnsubscribe: "<https://u.x/old>", ListUnsubscribePost: "List-Unsubscribe=One-Click"},
		{Account: "acct", ID: "l2", FromEmail: "list@x", Unread: false, InternalDate: base + 2},
		{Account: "acct", ID: "l3", FromEmail: "list@x", Unread: true, InternalDate: base + 3, ListUnsubscribe: "<https://u.x/new>", ListUnsubscribePost: "List-Unsubscribe=One-Click"},
		// Below min-count.
		{Account: "acct", ID: "s1", FromEmail: "seldom@x", InternalDate: base + 4, ListUnsubscribe: "<https://u.x/s>"},
		// No unsubscribe header at all.
		{Account: "acct", ID: "p1", FromEmail: "person@x", InternalDate: base + 5},
		{Account: "acct", ID: "p2", FromEmail: "person@x", InternalDate: base + 6},
		{Account: "acct", ID: "p3", FromEmail: "person@x", InternalDate: base + 7},
	})
	aggs, err := s.UnsubSenderAggregates("acct", 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(aggs) != 1 {
		t.Fatalf("aggregates = %+v, want just list@x", aggs)
	}
	if aggs[0].FromEmail != "list@x" || aggs[0].Count != 3 || aggs[0].UnreadCount != 2 || aggs[0].NewestMs != base+3 {
		t.Fatalf("list@x agg = %+v", aggs[0])
	}
	// min-count 1 admits seldom@x too (still never person@x).
	aggs, err = s.UnsubSenderAggregates("acct", 0, 1)
	if err != nil || len(aggs) != 2 {
		t.Fatalf("min-count 1 aggregates = (%+v, %v), want 2 rows", aggs, err)
	}

	m, err := s.NewestUnsubMeta("acct", "list@x")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "l3" || m.ListUnsubscribe != "<https://u.x/new>" {
		t.Fatalf("newest unsub meta = %+v, want l3", m)
	}
	if _, err := s.NewestUnsubMeta("acct", "person@x"); err == nil {
		t.Fatal("person@x has no unsub-bearing message; want sql.ErrNoRows")
	}

	n, err := s.SetMailListUnsubDomain("acct", "list@x", "u.x")
	if err != nil || n != 2 {
		t.Fatalf("SetMailListUnsubDomain = (%d, %v), want 2 rows", n, err)
	}
	got, err := s.GetMailMeta("acct", "l3")
	if err != nil || got.ListUnsubDomain != "u.x" {
		t.Fatalf("l3 domain = (%q, %v), want u.x", got.ListUnsubDomain, err)
	}
	got, _ = s.GetMailMeta("acct", "l2")
	if got.ListUnsubDomain != "" {
		t.Fatalf("l2 (no unsub header) domain = %q, want ''", got.ListUnsubDomain)
	}
}
