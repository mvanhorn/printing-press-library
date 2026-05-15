// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Behavioral assertions for the v1.1 novel verbs against a seeded mirror.
// These exercise the verb logic functions (gatherCustomerMentions,
// whoSaidLocal, gatherDrift, findChannels, ...) against a real SQLite
// mirror seeded via the store API — the same path the verb RunE takes —
// so a PASS here proves the verbs return real data, not fabrications.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// recentTS returns a Slack ts string for `agoMinutes` minutes before now,
// so seeded messages land inside the verbs' default windows regardless of
// the calendar date the test runs on.
func recentTS(agoMinutes int) string {
	t := time.Now().Add(-time.Duration(agoMinutes) * time.Minute)
	return fmt.Sprintf("%d.000100", t.Unix())
}

// seededDB bundles the seeded mirror with the dynamic thread-root ts so
// tests that need the fresh thread don't hardcode a calendar-dependent ts.
type seededDB struct {
	db         *store.Store
	ctx        context.Context
	threadRoot string
}

// seedBehavioralDB builds a temp mirror with users, channels, messages
// (3 mentioning "Sonria"), a stale thread, a fresh thread, a DM, and a
// flagged reaction — the shared fixture for the behavioral assertions.
func seedBehavioralDB(t *testing.T) seededDB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "seeded.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.EnsureMirrorSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatalf("seed: %v", e)
		}
	}
	must(db.UpsertUser(ctx, store.User{ID: "U_ERICK", Name: "erick", RealName: "Erick Holmann"}))
	must(db.UpsertUser(ctx, store.User{ID: "U_SOFIA", Name: "sofia", RealName: "Sofia Garcia"}))
	must(db.UpsertChannel(ctx, store.Channel{ID: "C_CHURN", Name: "churnsales", IsMember: true}))
	must(db.UpsertChannel(ctx, store.Channel{ID: "C_CSM", Name: "csm", IsMember: true}))
	must(db.UpsertChannel(ctx, store.Channel{ID: "C_DM_SOFIA", Name: "dm:U_SOFIA", IsIM: true, IsMember: true}))
	// A dormant channel: a member channel whose only message is from 2020.
	must(db.UpsertChannel(ctx, store.Channel{ID: "C_DEAD", Name: "old-project", IsMember: true}))
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_DEAD", TS: "00000000.000000", UserID: "U_ERICK", Text: "last message here, years ago"},
	}))

	// 3 messages mentioning Sonria — recent enough for default windows.
	mSonria1, mSonria2, mSonria3 := recentTS(180), recentTS(160), recentTS(140)
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_CHURN", TS: mSonria1, UserID: "U_ERICK", Text: "Sonria renewal call went well", Permalink: "https://x/1"},
		{ChannelID: "C_CHURN", TS: mSonria2, UserID: "U_SOFIA", Text: "Following up with Sonria on the dashboard", Permalink: "https://x/2"},
		{ChannelID: "C_CSM", TS: mSonria3, UserID: "U_ERICK", Text: "Sonria onboarding extended; compensation note attached", Permalink: "https://x/3"},
	}))
	// A fresh thread (root + reply) — recent last reply.
	threadRoot, threadReply := recentTS(120), recentTS(100)
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_CSM", TS: threadRoot, UserID: "U_ERICK", Text: "We agreed to ship the digest verb", ThreadTS: threadRoot, ReplyCount: 1, Permalink: "https://x/4"},
		{ChannelID: "C_CSM", TS: threadReply, UserID: "U_SOFIA", Text: "I'll follow up on the action item tomorrow", ThreadTS: threadRoot},
	}))
	must(db.SetThread(ctx, store.Thread{ChannelID: "C_CSM", ParentTS: threadRoot, LastReplyTS: threadReply, ReplyCount: 1}))
	// A DM message — recent.
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_DM_SOFIA", TS: recentTS(80), UserID: "U_SOFIA", Text: "quick question about Petroautos"},
	}))
	// One STALE thread — last reply in 2020.
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_CHURN", TS: "00000000.000000", UserID: "U_ERICK", Text: "Old dropped thread about a deal", ThreadTS: "00000000.000000", ReplyCount: 2, Permalink: "https://x/old"},
	}))
	must(db.SetThread(ctx, store.Thread{ChannelID: "C_CHURN", ParentTS: "00000000.000000", LastReplyTS: "00000100.000000", ReplyCount: 2}))
	// A flagged reaction on a recent message.
	must(db.UpsertReactions(ctx, []store.Reaction{
		{MessageChannelID: "C_CHURN", MessageTS: mSonria2, EmojiName: "eyes", UserIDs: []string{"U_ERICK"}, Count: 1},
	}))
	return seededDB{db: db, ctx: ctx, threadRoot: threadRoot}
}

func TestBehavioral_CustomerIntel_FindsSonria(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	mentions, err := gatherCustomerMentions(ctx, db, "Sonria", "", 100, false)
	if err != nil {
		t.Fatalf("gatherCustomerMentions: %v", err)
	}
	if len(mentions) < 1 {
		t.Fatalf("expected >=1 Sonria mention, got %d", len(mentions))
	}
	found := false
	for _, m := range mentions {
		if containsFold(m.Text, "Sonria") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no mention text contained 'Sonria': %+v", mentions)
	}
	t.Logf("customer-intel found %d Sonria mentions", len(mentions))
}

func TestBehavioral_CustomerIntel_Redacts(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	mentions, err := gatherCustomerMentions(ctx, db, "Sonria", "", 100, true)
	if err != nil {
		t.Fatalf("gatherCustomerMentions: %v", err)
	}
	for _, m := range mentions {
		if containsFold(m.Text, "compensation") {
			t.Fatalf("--redact-sensitivity left 'compensation' in: %q", m.Text)
		}
	}
}

func TestBehavioral_WhoSaid_ReturnsArray(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	hits, err := whoSaidLocal(ctx, db, "Sonria", "", 50)
	if err != nil {
		t.Fatalf("whoSaidLocal: %v", err)
	}
	if len(hits) < 1 {
		t.Fatalf("expected >=1 who-said hit for Sonria, got %d", len(hits))
	}
}

func TestBehavioral_Drift_FindsStaleThread(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	// Cutoff = now-minus-7d as a Slack ts. The 2020 thread is stale; the
	// 2026 thread is fresh.
	cutoff, err := resolveWindowTS("7d")
	if err != nil {
		t.Fatalf("resolveWindowTS: %v", err)
	}
	threads, err := gatherDrift(ctx, db, nil, cutoff, 100)
	if err != nil {
		t.Fatalf("gatherDrift: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("expected exactly 1 stale thread, got %d: %+v", len(threads), threads)
	}
	if threads[0].ParentTS != "00000000.000000" {
		t.Fatalf("expected the 2020 thread, got %+v", threads[0])
	}
}

func TestBehavioral_Drift_EmptyDBReturnsNone(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cutoff, _ := resolveWindowTS("7d")
	threads, err := gatherDrift(ctx, db, nil, cutoff, 100)
	if err != nil {
		t.Fatalf("gatherDrift: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("expected 0 stale threads on empty DB, got %d", len(threads))
	}
}

func TestBehavioral_ChannelFind_ResolvesChurnsales(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	matches, err := findChannels(ctx, db, "chu", 25)
	if err != nil {
		t.Fatalf("findChannels: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 channel match for 'chu', got %d: %+v", len(matches), matches)
	}
	if matches[0].Name != "churnsales" {
		t.Fatalf("expected #churnsales, got %q", matches[0].Name)
	}
}

func TestBehavioral_UserFind_ResolvesSofia(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	matches, err := findUsers(ctx, db, "sofia", 25)
	if err != nil {
		t.Fatalf("findUsers: %v", err)
	}
	if len(matches) != 1 || matches[0].ID != "U_SOFIA" {
		t.Fatalf("expected U_SOFIA, got %+v", matches)
	}
}

func TestBehavioral_Digest_CountsMessages(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	channels, err := db.ListChannels(ctx, true)
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	digest, err := buildDigest(ctx, db, channels, "", 3, false)
	if err != nil {
		t.Fatalf("buildDigest: %v", err)
	}
	var total int
	for _, dc := range digest {
		total += dc.MessageCount
	}
	if total == 0 {
		t.Fatalf("digest counted 0 messages across %d channels", len(digest))
	}
}

func TestBehavioral_ThreadSummary_BucketsActionItems(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	ch, err := db.ResolveChannel(ctx, "csm")
	if err != nil {
		t.Fatalf("ResolveChannel: %v", err)
	}
	sum, err := buildThreadSummary(ctx, db, ch, s.threadRoot)
	if err != nil {
		t.Fatalf("buildThreadSummary: %v", err)
	}
	if sum.MessageCount != 2 {
		t.Fatalf("expected 2 thread messages, got %d", sum.MessageCount)
	}
	if len(sum.Decisions) == 0 {
		t.Fatalf("expected the 'we agreed' line bucketed as a decision")
	}
	if len(sum.ActionItems) == 0 {
		t.Fatalf("expected the \"I'll follow up\" line bucketed as an action item")
	}
}

func TestBehavioral_Attention_BucketsSignals(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	res, err := buildAttention(ctx, db, "7d", mustWindow(t, "7d"), false, 100)
	if err != nil {
		t.Fatalf("buildAttention: %v", err)
	}
	if res.Total == 0 {
		t.Fatalf("expected triage items (DM + flagged reaction at minimum)")
	}
	if res.Buckets["dm"] == 0 {
		t.Fatalf("expected the seeded DM in the dm bucket")
	}
	if res.Buckets["flagged"] == 0 {
		t.Fatalf("expected the :eyes: reaction in the flagged bucket")
	}
}

func TestBehavioral_DMSummary_CountsDM(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	dms, err := selectDMChannels(ctx, db, "")
	if err != nil {
		t.Fatalf("selectDMChannels: %v", err)
	}
	summaries, err := buildDMSummaries(ctx, db, dms, "", 3, false)
	if err != nil {
		t.Fatalf("buildDMSummaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].MessageCount != 1 {
		t.Fatalf("expected 1 DM with 1 message, got %+v", summaries)
	}
}

func TestBehavioral_Dormant_FlagsSilentChannel(t *testing.T) {
	s := seedBehavioralDB(t)
	db, ctx := s.db, s.ctx
	// A 30-day window: the active channels (last message ~3h ago) are not
	// dormant, but #old-project (last message in 2020) must be flagged.
	cutoff := mustWindow(t, "30d")
	rows, err := gatherDormant(ctx, db, cutoff, false, 100)
	if err != nil {
		t.Fatalf("gatherDormant: %v", err)
	}
	var deadFlagged bool
	for _, r := range rows {
		if r.ChannelID == "C_DEAD" {
			deadFlagged = true
		}
		if r.ChannelID == "C_CHURN" || r.ChannelID == "C_CSM" {
			t.Fatalf("active channel %s wrongly flagged dormant", r.ChannelID)
		}
	}
	if !deadFlagged {
		t.Fatalf("expected #old-project (C_DEAD) flagged dormant, rows: %+v", rows)
	}
}

func mustWindow(t *testing.T, w string) string {
	t.Helper()
	ts, err := resolveWindowTS(w)
	if err != nil {
		t.Fatalf("resolveWindowTS(%q): %v", w, err)
	}
	return ts
}

// TestSeedExportDB writes a seeded mirror to the path in P1_SEED_EXPORT
// when that env var is set, so the acceptance harness can exercise the
// real verb binary against real data. It is a no-op in a normal `go
// test ./...` run (env var unset), so it adds nothing to CI noise.
func TestSeedExportDB(t *testing.T) {
	dest := os.Getenv("P1_SEED_EXPORT")
	if dest == "" {
		t.Skip("P1_SEED_EXPORT not set — export helper inactive")
	}
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dest)
	if err != nil {
		t.Fatalf("open export db: %v", err)
	}
	if err := db.EnsureMirrorSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatalf("seed: %v", e)
		}
	}
	must(db.UpsertUser(ctx, store.User{ID: "U_ERICK", Name: "erick", RealName: "Erick Holmann"}))
	must(db.UpsertUser(ctx, store.User{ID: "U_SOFIA", Name: "sofia", RealName: "Sofia Garcia"}))
	must(db.UpsertChannel(ctx, store.Channel{ID: "C_CHURN", Name: "churnsales", IsMember: true}))
	must(db.UpsertChannel(ctx, store.Channel{ID: "C_CSM", Name: "csm", IsMember: true}))
	r := func(min int) string {
		return fmt.Sprintf("%d.000100", time.Now().Add(-time.Duration(min)*time.Minute).Unix())
	}
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_CHURN", TS: r(180), UserID: "U_ERICK", Text: "Sonria renewal call went well", Permalink: "https://x/1"},
		{ChannelID: "C_CHURN", TS: r(160), UserID: "U_SOFIA", Text: "Following up with Sonria on the dashboard", Permalink: "https://x/2"},
		{ChannelID: "C_CSM", TS: r(140), UserID: "U_ERICK", Text: "Sonria onboarding extended", Permalink: "https://x/3"},
	}))
	must(db.SetThread(ctx, store.Thread{ChannelID: "C_CHURN", ParentTS: "00000000.000000", LastReplyTS: "00000100.000000", ReplyCount: 2}))
	must(db.UpsertMessages(ctx, []store.Message{
		{ChannelID: "C_CHURN", TS: "00000000.000000", UserID: "U_ERICK", Text: "Old dropped thread", ThreadTS: "00000000.000000", ReplyCount: 2, Permalink: "https://x/old"},
	}))
	db.Close()
	t.Logf("seeded export DB at %s", dest)
}
