// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the novel local-store read commands (digest, delta,
// storage report, sort suggest, trash report, score), each on a seeded
// store through the real command tree, asserting exact output values.
// The per-command scaffold smoke tests (*_test.go TestNovel*HelpWires)
// stay alongside; this file adds behavior plus the house --help/Examples
// and --select checks for every new command.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

const testDayMs = int64(24 * 60 * 60 * 1000)

// seedStore upserts rows into the fixture's store.
func seedStore(t *testing.T, fx *engineFixture, rows []store.MailMeta) *store.Store {
	t.Helper()
	db := fx.openStore(t)
	if n, err := db.UpsertMailMeta(rows); err != nil || n != len(rows) {
		t.Fatalf("seed upsert = (%d, %v), want %d", n, err, len(rows))
	}
	return db
}

func TestDigest_CategoryMathAndRollup(t *testing.T) {
	fx := newEngineFixture(t)
	nowMs := time.Now().UnixMilli()
	oldestUnread := nowMs - 3*testDayMs - 3600_000 // ~3.04 days ago -> 3 full days
	seedStore(t, fx, []store.MailMeta{
		{Account: "test", ID: "d1", FromEmail: "promo@a.example", Category: "promotions", Unread: true, SizeEstimate: 100, InternalDate: oldestUnread},
		{Account: "test", ID: "d2", FromEmail: "promo@a.example", Category: "promotions", Unread: false, SizeEstimate: 200, InternalDate: nowMs - 2*testDayMs},
		{Account: "test", ID: "d3", FromEmail: "promo@b.example", Category: "promotions", Unread: true, SizeEstimate: 300, InternalDate: nowMs - testDayMs},
		{Account: "test", ID: "d4", FromEmail: "up@c.example", Category: "updates", Unread: false, SizeEstimate: 50, InternalDate: nowMs - testDayMs},
		{Account: "test", ID: "d5", FromEmail: "none@d.example", Category: "", Unread: false, SizeEstimate: 25, InternalDate: nowMs - testDayMs},
	})

	out, stderr, code := fx.runCLI(t, "digest", "--account", "test", "--since", "")
	if code != 0 {
		t.Fatalf("digest exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	cats := parsed["categories"].([]any)
	if len(cats) != 6 {
		t.Fatalf("categories = %d, want all 6 (zero-count included)", len(cats))
	}
	byCat := map[string]map[string]any{}
	for _, c := range cats {
		m := c.(map[string]any)
		byCat[m["category"].(string)] = m
	}
	p := byCat["promotions"]
	if p["total"].(float64) != 3 || p["unread"].(float64) != 2 || p["total_size"].(float64) != 600 {
		t.Fatalf("promotions row = %v", p)
	}
	if p["oldest_unread_age_days"].(float64) != 3 {
		t.Fatalf("promotions oldest_unread_age_days = %v, want 3", p["oldest_unread_age_days"])
	}
	top := p["top_senders"].([]any)
	if len(top) != 2 || top[0].(map[string]any)["from_email"] != "promo@a.example" || top[0].(map[string]any)["count"].(float64) != 2 {
		t.Fatalf("promotions top_senders = %v", top)
	}
	// Zero-count categories are present with honest zeros and null age.
	social := byCat["social"]
	if social["total"].(float64) != 0 || social["oldest_unread_age_days"] != nil {
		t.Fatalf("social (zero-count) row = %v", social)
	}
	if forums := byCat["forums"]; forums["total"].(float64) != 0 {
		t.Fatalf("forums (zero-count) row = %v", forums)
	}
	if u := byCat["updates"]; u["total"].(float64) != 1 || u["unread"].(float64) != 0 || u["oldest_unread_age_days"] != nil {
		t.Fatalf("updates row = %v", u)
	}
	if e := byCat[""]; e["total"].(float64) != 1 || e["total_size"].(float64) != 25 {
		t.Fatalf("uncategorized row = %v", e)
	}
	rollup := parsed["rollup"].(map[string]any)
	if rollup["total"].(float64) != 5 || rollup["unread"].(float64) != 2 || rollup["total_size"].(float64) != 675 {
		t.Fatalf("rollup = %v", rollup)
	}
	if rollup["oldest_unread_age_days"].(float64) != 3 {
		t.Fatalf("rollup oldest age = %v, want 3", rollup["oldest_unread_age_days"])
	}

	// --since bound excludes older rows.
	out, _, code = fx.runCLI(t, "digest", "--account", "test", "--since", "2d")
	if code != 0 {
		t.Fatalf("digest --since exit = %d\n%s", code, out)
	}
	parsed = mustParseJSON(t, out)
	if got := parsed["rollup"].(map[string]any)["total"].(float64); got != 3 {
		t.Fatalf("since-bounded rollup total = %v, want 3", got)
	}

	// --select under --agent.
	out, _, code = fx.runCLI(t, "digest", "--account", "test", "--since", "", "--agent", "--select", "rollup.total")
	if code != 0 {
		t.Fatalf("digest --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	results := env["results"].(map[string]any)
	if results["rollup"].(map[string]any)["total"].(float64) != 5 || results["categories"] != nil {
		t.Fatalf("--select rollup.total results = %v", results)
	}
}

func TestDelta_BaselineAdvancePeekAndSpikes(t *testing.T) {
	fx := newEngineFixture(t)
	wm := time.Now().Add(-24 * time.Hour).UnixMilli()
	// Prior history: spiky sender, 10 messages over the 10 days before wm
	// (first seen exactly wm-10d -> prior daily average exactly 1.0), and
	// old1 at exactly wm so the baseline watermark IS wm.
	var rows []store.MailMeta
	for i := 0; i < 10; i++ {
		rows = append(rows, store.MailMeta{
			Account: "test", ID: fmt.Sprintf("pre%d", i), FromEmail: "spiky@x.example",
			Category: "promotions", InternalDate: wm - int64(10-i)*testDayMs,
		})
	}
	rows = append(rows, store.MailMeta{Account: "test", ID: "old1", FromEmail: "old@x.example", Category: "primary", InternalDate: wm})
	db := seedStore(t, fx, rows)

	// First run: baseline set + advanced.
	out, stderr, code := fx.runCLI(t, "delta", "--account", "test")
	if code != 0 {
		t.Fatalf("delta baseline exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	if parsed["baseline_set"] != true || parsed["advanced"] != true || parsed["new_total"].(float64) != 0 {
		t.Fatalf("baseline run = %s", out)
	}
	if !strings.Contains(parsed["note"].(string), "baseline") {
		t.Fatalf("baseline note = %v", parsed["note"])
	}

	// New arrivals after the baseline: 4 spiky (> 3x its 1/day average),
	// 1 from a never-seen sender.
	newRows := []store.MailMeta{
		{Account: "test", ID: "n1", FromEmail: "spiky@x.example", Category: "promotions", InternalDate: wm + 1000},
		{Account: "test", ID: "n2", FromEmail: "spiky@x.example", Category: "promotions", InternalDate: wm + 2000},
		{Account: "test", ID: "n3", FromEmail: "spiky@x.example", Category: "promotions", InternalDate: wm + 3000},
		{Account: "test", ID: "n4", FromEmail: "spiky@x.example", Category: "promotions", InternalDate: wm + 4000},
		{Account: "test", ID: "n5", FromEmail: "fresh@new.example", Category: "primary", InternalDate: wm + 5000},
	}
	if n, err := db.UpsertMailMeta(newRows); err != nil || n != len(newRows) {
		t.Fatalf("seed new rows: (%d, %v)", n, err)
	}

	assertDelta := func(parsed map[string]any) {
		t.Helper()
		if parsed["new_total"].(float64) != 5 {
			t.Fatalf("new_total = %v, want 5", parsed["new_total"])
		}
		per := parsed["per_category"].(map[string]any)
		if per["promotions"].(float64) != 4 || per["primary"].(float64) != 1 {
			t.Fatalf("per_category = %v", per)
		}
		newSenders := parsed["new_senders"].([]any)
		if len(newSenders) != 1 || newSenders[0] != "fresh@new.example" {
			t.Fatalf("new_senders = %v", newSenders)
		}
		spikes := parsed["spikes"].([]any)
		if len(spikes) != 1 {
			t.Fatalf("spikes = %v, want just spiky@x.example", spikes)
		}
		sp := spikes[0].(map[string]any)
		if sp["from_email"] != "spiky@x.example" || sp["since_count"].(float64) != 4 || sp["prior_daily_avg"].(float64) != 1 {
			t.Fatalf("spike row = %v", sp)
		}
	}

	// --peek reports without advancing.
	out, _, code = fx.runCLI(t, "delta", "--account", "test", "--peek")
	if code != 0 {
		t.Fatalf("delta --peek exit = %d\n%s", code, out)
	}
	parsed = mustParseJSON(t, out)
	assertDelta(parsed)
	if parsed["advanced"] != false {
		t.Fatalf("--peek advanced = %v, want false", parsed["advanced"])
	}

	// Real run reports the same and advances.
	out, _, code = fx.runCLI(t, "delta", "--account", "test")
	if code != 0 {
		t.Fatalf("delta advance exit = %d\n%s", code, out)
	}
	parsed = mustParseJSON(t, out)
	assertDelta(parsed)
	if parsed["advanced"] != true {
		t.Fatalf("advance run advanced = %v, want true", parsed["advanced"])
	}

	// After advancing, nothing is new.
	out, _, code = fx.runCLI(t, "delta", "--account", "test")
	if code != 0 {
		t.Fatalf("delta post-advance exit = %d\n%s", code, out)
	}
	parsed = mustParseJSON(t, out)
	if parsed["new_total"].(float64) != 0 || len(parsed["new_senders"].([]any)) != 0 {
		t.Fatalf("post-advance delta = %s", out)
	}

	// --select under --agent.
	out, _, code = fx.runCLI(t, "delta", "--account", "test", "--peek", "--agent", "--select", "new_total,advanced")
	if code != 0 {
		t.Fatalf("delta --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	results := env["results"].(map[string]any)
	if len(results) != 2 || results["new_total"] == nil {
		t.Fatalf("--select results = %v", results)
	}
}

func TestStorageReport_AttributionSums(t *testing.T) {
	fx := newEngineFixture(t)
	y2023 := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	y2024 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	seedStore(t, fx, []store.MailMeta{
		{Account: "test", ID: "s1", FromEmail: "big@x.example", Subject: "huge attachment", Category: "promotions", SizeEstimate: 5000, InternalDate: y2023},
		{Account: "test", ID: "s2", FromEmail: "big@x.example", Subject: "large", Category: "promotions", SizeEstimate: 3000, InternalDate: y2024},
		{Account: "test", ID: "s3", FromEmail: "small@y.example", Subject: "tiny", Category: "updates", SizeEstimate: 100, InternalDate: y2024},
	})

	out, stderr, code := fx.runCLI(t, "storage", "report", "--account", "test", "--top", "2")
	if code != 0 {
		t.Fatalf("storage report exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	bySender := parsed["by_sender"].([]any)
	if len(bySender) != 2 {
		t.Fatalf("by_sender = %v", bySender)
	}
	top := bySender[0].(map[string]any)
	if top["from_email"] != "big@x.example" || top["total_size"].(float64) != 8000 || top["count"].(float64) != 2 {
		t.Fatalf("top sender = %v", top)
	}
	if top["ready_query"] != "from:big@x.example larger:1m" {
		t.Fatalf("ready_query = %v", top["ready_query"])
	}
	byCat := parsed["by_category"].([]any)
	if byCat[0].(map[string]any)["category"] != "promotions" || byCat[0].(map[string]any)["total_size"].(float64) != 8000 {
		t.Fatalf("by_category = %v", byCat)
	}
	byYear := parsed["by_year"].([]any)
	if len(byYear) != 2 || byYear[0].(map[string]any)["year"].(float64) != 2024 || byYear[0].(map[string]any)["total_size"].(float64) != 3100 {
		t.Fatalf("by_year = %v", byYear)
	}
	if byYear[1].(map[string]any)["year"].(float64) != 2023 || byYear[1].(map[string]any)["total_size"].(float64) != 5000 {
		t.Fatalf("by_year[1] = %v", byYear[1])
	}
	largest := parsed["largest"].([]any)
	if len(largest) != 2 || largest[0].(map[string]any)["id"] != "s1" || largest[0].(map[string]any)["size_estimate"].(float64) != 5000 {
		t.Fatalf("largest = %v", largest)
	}

	// --select under --agent.
	out, _, code = fx.runCLI(t, "storage", "report", "--account", "test", "--agent", "--select", "by_year.year,by_year.count")
	if code != 0 {
		t.Fatalf("storage --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	yearRows := env["results"].(map[string]any)["by_year"].([]any)
	if row := yearRows[0].(map[string]any); len(row) != 2 || row["year"] == nil || row["count"] == nil {
		t.Fatalf("--select by_year row = %v", row)
	}
}

func TestSortSuggest_ConfidenceMathAndExclusions(t *testing.T) {
	fx := newEngineFixture(t)
	var rows []store.MailMeta
	// receipts@x: 8 total; 6 labeled (5x Label_7, 1x Label_9) -> confidence
	// 5/6 = 0.8333 >= 0.8, unlabeled 2.
	for i := 1; i <= 5; i++ {
		rows = append(rows, store.MailMeta{Account: "test", ID: fmt.Sprintf("r%d", i), FromEmail: "receipts@x.example", LabelIDs: []string{"INBOX", "Label_7"}})
	}
	rows = append(rows,
		store.MailMeta{Account: "test", ID: "r6", FromEmail: "receipts@x.example", LabelIDs: []string{"Label_9"}},
		store.MailMeta{Account: "test", ID: "r7", FromEmail: "receipts@x.example", LabelIDs: []string{"INBOX"}},
		store.MailMeta{Account: "test", ID: "r8", FromEmail: "receipts@x.example", LabelIDs: []string{"INBOX", "IMPORTANT", "STARRED", "CATEGORY_UPDATES"}},
	)
	// mixed@y: 6 labeled but split 3/3 -> confidence 0.5 -> excluded.
	for i := 1; i <= 3; i++ {
		rows = append(rows, store.MailMeta{Account: "test", ID: fmt.Sprintf("m%d", i), FromEmail: "mixed@y.example", LabelIDs: []string{"Label_1"}})
		rows = append(rows, store.MailMeta{Account: "test", ID: fmt.Sprintf("m%d", i+3), FromEmail: "mixed@y.example", LabelIDs: []string{"Label_2"}})
	}
	// sparse@z: only 2 labeled -> below --min-labeled -> excluded.
	rows = append(rows,
		store.MailMeta{Account: "test", ID: "sp1", FromEmail: "sparse@z.example", LabelIDs: []string{"Label_3"}},
		store.MailMeta{Account: "test", ID: "sp2", FromEmail: "sparse@z.example", LabelIDs: []string{"Label_3"}},
	)
	seedStore(t, fx, rows)

	out, stderr, code := fx.runCLI(t, "sort", "suggest", "--account", "test")
	if code != 0 {
		t.Fatalf("sort suggest exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	var suggestions []map[string]any
	if err := jsonUnmarshalString(out, &suggestions); err != nil {
		t.Fatalf("sort suggest output: %v\n%s", err, out)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %v, want exactly receipts@x.example", suggestions)
	}
	s := suggestions[0]
	if s["sender"] != "receipts@x.example" || s["label"] != "Label_7" {
		t.Fatalf("suggestion = %v", s)
	}
	if s["confidence"].(float64) != 0.8333 {
		t.Fatalf("confidence = %v, want 0.8333", s["confidence"])
	}
	if s["labeled_count"].(float64) != 6 || s["unlabeled_count"].(float64) != 2 {
		t.Fatalf("labeled/unlabeled = %v/%v, want 6/2", s["labeled_count"], s["unlabeled_count"])
	}
	wantPlan := `cleanup plan --account test --q "from:receipts@x.example -label:Label_7" --action label --add Label_7`
	if s["plan_invocation"] != wantPlan {
		t.Fatalf("plan_invocation = %q, want %q", s["plan_invocation"], wantPlan)
	}

	// Lower threshold admits the 0.5-confidence sender too.
	out, _, code = fx.runCLI(t, "sort", "suggest", "--account", "test", "--min-confidence", "0.5", "--min-labeled", "2")
	if code != 0 {
		t.Fatalf("sort suggest lowered exit = %d\n%s", code, out)
	}
	if err := jsonUnmarshalString(out, &suggestions); err != nil {
		t.Fatal(err)
	}
	if len(suggestions) != 3 {
		t.Fatalf("lowered thresholds suggestions = %d, want 3", len(suggestions))
	}

	// --select under --agent.
	out, _, code = fx.runCLI(t, "sort", "suggest", "--account", "test", "--agent", "--select", "sender,confidence")
	if code != 0 {
		t.Fatalf("sort suggest --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	first := env["results"].([]any)[0].(map[string]any)
	if len(first) != 2 || first["confidence"] == nil {
		t.Fatalf("--select row = %v", first)
	}
}

func TestTrashReport_DaysRemainingAndOutsideLedger(t *testing.T) {
	fx := newEngineFixture(t)
	db := fx.openStore(t)
	now := time.Now().UTC()

	mkLedger := func(id string, age time.Duration, ids ...string) {
		t.Helper()
		if err := db.CreateMailLedger(store.MailLedger{
			LedgerID: id, Account: "test", PlanSha: "sha-" + id, Action: "trash",
			CreatedAt: now.Add(-age).Format(time.RFC3339),
		}); err != nil {
			t.Fatal(err)
		}
		var entries []store.MailLedgerEntry
		for _, mid := range ids {
			entries = append(entries, store.MailLedgerEntry{LedgerID: id, ID: mid, Kind: "trash", DeltaAdd: []string{"TRASH"}, PrePlacement: []string{"INBOX"}})
		}
		if _, err := db.InsertMailLedgerEntries(entries); err != nil {
			t.Fatal(err)
		}
	}
	mkLedger("fresh001", 2*24*time.Hour+time.Hour, "t1", "t2", "t3") // age 2 -> 28 remaining
	mkLedger("stale001", 25*24*time.Hour+time.Hour, "t4")            // age 25 -> 5 remaining
	if err := db.SetMailLedgerEntryUndone("fresh001", "t2", "undone"); err != nil {
		t.Fatal(err)
	}

	// Two TRASH-labeled rows outside any ledger + one ledgered + one clean.
	if _, err := db.UpsertMailMeta([]store.MailMeta{
		{Account: "test", ID: "t1", FromEmail: "a@x", LabelIDs: []string{"TRASH"}},
		{Account: "test", ID: "ext1", FromEmail: "a@x", LabelIDs: []string{"TRASH"}},
		{Account: "test", ID: "ext2", FromEmail: "b@y", LabelIDs: []string{"TRASH", "CATEGORY_PROMOTIONS"}},
		{Account: "test", ID: "keep1", FromEmail: "b@y", LabelIDs: []string{"INBOX"}},
	}); err != nil {
		t.Fatal(err)
	}

	out, stderr, code := fx.runCLI(t, "trash", "report", "--account", "test")
	if code != 0 {
		t.Fatalf("trash report exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	if parsed["retention_note"] != trashRetentionNote {
		t.Fatalf("retention_note = %v", parsed["retention_note"])
	}
	ledgers := parsed["ledgers"].([]any)
	if len(ledgers) != 2 {
		t.Fatalf("ledgers = %v, want 2", ledgers)
	}
	stale := ledgers[0].(map[string]any) // oldest first
	fresh := ledgers[1].(map[string]any)
	if stale["ledger_id"] != "stale001" || stale["age_days"].(float64) != 25 || stale["days_remaining"].(float64) != 5 {
		t.Fatalf("stale ledger row = %v", stale)
	}
	if fresh["ledger_id"] != "fresh001" || fresh["trashed"].(float64) != 3 || fresh["undone"].(float64) != 1 || fresh["days_remaining"].(float64) != 28 {
		t.Fatalf("fresh ledger row = %v", fresh)
	}
	if fresh["undo_hint"] != "gmail-pp-cli undo --ledger fresh001" {
		t.Fatalf("undo_hint = %v", fresh["undo_hint"])
	}
	if parsed["outside_ledger_trashed"].(float64) != 2 {
		t.Fatalf("outside_ledger_trashed = %v, want 2", parsed["outside_ledger_trashed"])
	}
	if note, _ := parsed["outside_ledger_note"].(string); !strings.Contains(note, "2 TRASH-labeled") {
		t.Fatalf("outside_ledger_note = %q", note)
	}

	// --closing-soon keeps only the 5-days-remaining ledger.
	out, _, code = fx.runCLI(t, "trash", "report", "--account", "test", "--closing-soon")
	if code != 0 {
		t.Fatalf("trash report --closing-soon exit = %d\n%s", code, out)
	}
	parsed = mustParseJSON(t, out)
	ledgers = parsed["ledgers"].([]any)
	if len(ledgers) != 1 || ledgers[0].(map[string]any)["ledger_id"] != "stale001" {
		t.Fatalf("--closing-soon ledgers = %v", ledgers)
	}

	// --select under --agent.
	out, _, code = fx.runCLI(t, "trash", "report", "--account", "test", "--agent", "--select", "ledgers.ledger_id,ledgers.days_remaining")
	if code != 0 {
		t.Fatalf("trash report --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	rows := env["results"].(map[string]any)["ledgers"].([]any)
	if row := rows[0].(map[string]any); len(row) != 2 || row["days_remaining"] == nil {
		t.Fatalf("--select ledger row = %v", row)
	}
}

func TestScore_SnapshotAndDeltas(t *testing.T) {
	fx := newEngineFixture(t)
	nowMs := time.Now().UnixMilli()
	oldestUnread := nowMs - 2*testDayMs - 3600_000 // ~2.04 days -> 2 full days
	db := seedStore(t, fx, []store.MailMeta{
		{Account: "test", ID: "sc1", FromEmail: "a@x.example", Category: "promotions", Unread: true, SizeEstimate: 100, InternalDate: oldestUnread, ListUnsubscribe: "<https://u.x.example/1>"},
		{Account: "test", ID: "sc2", FromEmail: "a@x.example", Category: "promotions", Unread: false, SizeEstimate: 200, InternalDate: nowMs - testDayMs, ListUnsubscribe: "<https://u.x.example/1>"},
		{Account: "test", ID: "sc3", FromEmail: "b@y.example", Category: "primary", Unread: true, SizeEstimate: 300, InternalDate: nowMs - testDayMs, ListUnsubscribe: "<https://u.y.example/2>"},
		{Account: "test", ID: "sc4", FromEmail: "c@z.example", Category: "updates", Unread: false, SizeEstimate: 400, InternalDate: nowMs - testDayMs},
	})

	// First run: baseline.
	out, stderr, code := fx.runCLI(t, "score", "--account", "test")
	if code != 0 {
		t.Fatalf("score exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	cur := parsed["current"].(map[string]any)
	if cur["unread_pct"].(float64) != 50 || cur["promotions_pct"].(float64) != 50 {
		t.Fatalf("current pcts = %v", cur)
	}
	if cur["subscription_sender_count"].(float64) != 2 || cur["storage_total"].(float64) != 1000 {
		t.Fatalf("current counts = %v", cur)
	}
	if cur["oldest_unread_days"].(float64) != 2 {
		t.Fatalf("oldest_unread_days = %v, want 2", cur["oldest_unread_days"])
	}
	if parsed["baseline"] != true || parsed["delta_vs_previous"] != nil {
		t.Fatalf("first run must be the baseline: %s", out)
	}

	// Mark sc1 read; second run reports the movement vs previous AND first.
	if _, err := db.UpsertMailMeta([]store.MailMeta{
		{Account: "test", ID: "sc1", FromEmail: "a@x.example", Category: "promotions", Unread: false, SizeEstimate: 100, InternalDate: oldestUnread, ListUnsubscribe: "<https://u.x.example/1>"},
	}); err != nil {
		t.Fatal(err)
	}
	out, _, code = fx.runCLI(t, "score", "--account", "test")
	if code != 0 {
		t.Fatalf("second score exit = %d\n%s", code, out)
	}
	parsed = mustParseJSON(t, out)
	cur = parsed["current"].(map[string]any)
	if cur["unread_pct"].(float64) != 25 {
		t.Fatalf("second unread_pct = %v, want 25", cur["unread_pct"])
	}
	if parsed["baseline"] != false && parsed["baseline"] != nil {
		t.Fatalf("second run baseline = %v", parsed["baseline"])
	}
	dPrev := parsed["delta_vs_previous"].(map[string]any)
	if dPrev["unread_pct"].(float64) != -25 || dPrev["total"].(float64) != 0 {
		t.Fatalf("delta_vs_previous = %v", dPrev)
	}
	dFirst := parsed["delta_vs_first"].(map[string]any)
	if dFirst["unread_pct"].(float64) != -25 {
		t.Fatalf("delta_vs_first = %v", dFirst)
	}
	// The store now holds exactly two snapshots.
	firstSnap, ok, err := db.FirstMailScore("test")
	if err != nil || !ok {
		t.Fatalf("first snapshot: ok=%v err=%v", ok, err)
	}
	var firstM map[string]any
	if err := json.Unmarshal([]byte(firstSnap.Metrics), &firstM); err != nil {
		t.Fatal(err)
	}
	if firstM["unread_pct"].(float64) != 50 {
		t.Fatalf("stored baseline metrics = %v", firstM)
	}

	// --select under --agent.
	out, _, code = fx.runCLI(t, "score", "--account", "test", "--agent", "--select", "current.unread_pct")
	if code != 0 {
		t.Fatalf("score --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	results := env["results"].(map[string]any)
	if results["current"].(map[string]any)["unread_pct"] == nil || results["taken_at"] != nil {
		t.Fatalf("--select results = %v", results)
	}
}

// TestNewCommandsHelpHasExamples pins acceptance 6: every new command's
// --help exits 0 and carries house-format Examples.
func TestNewCommandsHelpHasExamples(t *testing.T) {
	paths := [][]string{
		{"unsub", "audit"},
		{"unsub", "plan"},
		{"unsub", "run"},
		{"unsub", "verify"},
		{"digest"},
		{"delta"},
		{"storage", "report"},
		{"sort", "suggest"},
		{"trash", "report"},
		{"score"},
	}
	for _, p := range paths {
		fx := struct{}{} // no fixture needed; --help never touches auth or store
		_ = fx
		cmd := RootCmd()
		cmd.SetArgs(append(append([]string(nil), p...), "--help"))
		var out strings.Builder
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v --help error: %v", p, err)
		}
		help := out.String()
		for _, want := range []string{"Usage:", "Examples:", "gmail-pp-cli"} {
			if !strings.Contains(help, want) {
				t.Fatalf("%v --help missing %q:\n%s", p, want, help)
			}
		}
	}
}
