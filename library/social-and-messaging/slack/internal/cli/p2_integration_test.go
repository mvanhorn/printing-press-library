// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Integration tests for the 8 P2 transcendence verbs. They seed a real
// SQLite mirror via the store API, then execute each verb's cobra
// command against that DB and assert on the emitted JSON. attachP2Cmds
// is exercised here too — it is the function root.go calls to wire the
// verbs, so building a root command and calling it proves the wiring.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// p2TestRoot builds a minimal cobra root with the 8 P2 verbs attached.
// It mirrors what root.go's attachP2Cmds(rootCmd, flags) call does.
func p2TestRoot(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "slack-pp-cli"}
	attachP2Cmds(root, flags)
	return root
}

// runP2 executes the P2 root with the given args, returning stdout.
func runP2(t *testing.T, flags *rootFlags, args ...string) (string, error) {
	t.Helper()
	root := p2TestRoot(flags)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// seedMirror creates a temp DB and populates it with channels, users,
// messages, reactions, a usergroup and audit-log rows.
func seedMirror(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	if err := db.EnsureMirrorSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// Channels: one public, one IM.
	if err := db.UpsertChannel(ctx, store.Channel{
		ID: "C100", Name: "the-wolf-of-atom", IsMember: true,
	}); err != nil {
		t.Fatalf("upsert channel: %v", err)
	}
	if err := db.UpsertChannel(ctx, store.Channel{
		ID: "D200", Name: "dm:U2", IsMember: true, IsIM: true,
	}); err != nil {
		t.Fatalf("upsert im channel: %v", err)
	}

	// Users.
	for _, u := range []store.User{
		{ID: "U1", Name: "erick", RealName: "Erick Holmann"},
		{ID: "U2", Name: "marjorie", RealName: "Marjorie CSM", DisplayName: "Marjorie"},
	} {
		if err := db.UpsertUser(ctx, u); err != nil {
			t.Fatalf("upsert user: %v", err)
		}
	}

	// Messages in the public channel. One mentions "Sonria".
	msgs := []store.Message{
		{ChannelID: "C100", TS: "00000100.000000", UserID: "U1", Text: "kickoff with Sonria today", Permalink: "https://slack/p100"},
		{ChannelID: "C100", TS: "00000200.000000", UserID: "U2", Text: "second message", Permalink: "https://slack/p200"},
		{ChannelID: "D200", TS: "00000300.000000", UserID: "U2", Text: "dm to erick", Permalink: "https://slack/p300"},
	}
	if err := db.UpsertMessages(ctx, msgs); err != nil {
		t.Fatalf("upsert messages: %v", err)
	}

	// Reactions on the public-channel messages.
	reactions := []store.Reaction{
		{MessageChannelID: "C100", MessageTS: "00000100.000000", EmojiName: "tada", UserIDs: []string{"U2"}, Count: 1},
		{MessageChannelID: "C100", MessageTS: "00000100.000000", EmojiName: "fire", UserIDs: []string{"U1", "U2"}, Count: 2},
		{MessageChannelID: "C100", MessageTS: "00000200.000000", EmojiName: "+1", UserIDs: []string{"U1"}, Count: 1},
	}
	if err := db.UpsertReactions(ctx, reactions); err != nil {
		t.Fatalf("upsert reactions: %v", err)
	}

	// Usergroup.
	if err := db.UpsertUsergroup(ctx, store.Usergroup{
		ID: "S012ABC", Handle: "csm-team", Name: "CSM Team", UserIDs: []string{"U2"},
	}); err != nil {
		t.Fatalf("upsert usergroup: %v", err)
	}

	// Two audit-log rows.
	if err := db.AppendAuditLog(ctx, "cron", "sync mirror", "D200", "im read"); err != nil {
		t.Fatalf("append audit: %v", err)
	}
	if err := db.AppendAuditLog(ctx, "agent-x", "dm-engagement", "D200", "dm volume read"); err != nil {
		t.Fatalf("append audit: %v", err)
	}

	return dbPath
}

func TestP2_ReactionsSummarize_Seeded(t *testing.T) {
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	out, err := runP2(t, flags, "reactions", "summarize",
		"--channel", "#the-wolf-of-atom", "--window", "", "--db", dbPath)
	if err != nil {
		t.Fatalf("reactions summarize errored: %v\n%s", err, out)
	}
	var r struct {
		TotalReactions int `json:"total_reactions"`
		TopMessages    []struct {
			TS            string `json:"ts"`
			ReactionCount int    `json:"reaction_count"`
		} `json:"top_messages"`
		ClassCounts map[string]int `json:"class_counts"`
	}
	mustJSON(t, out, &r)
	if r.TotalReactions != 4 {
		t.Errorf("total_reactions = %d, want 4", r.TotalReactions)
	}
	if len(r.TopMessages) != 2 {
		t.Fatalf("top_messages = %d, want 2", len(r.TopMessages))
	}
	// ts 100 has tada(1)+fire(2)=3, ts 200 has +1(1)=1 — 100 ranks first.
	if r.TopMessages[0].TS != "00000100.000000" || r.TopMessages[0].ReactionCount != 3 {
		t.Errorf("top message = %+v, want ts100 count3", r.TopMessages[0])
	}
	if r.ClassCounts["hot"] != 2 || r.ClassCounts["celebrate"] != 1 || r.ClassCounts["approve"] != 1 {
		t.Errorf("class counts = %v, want hot2 celebrate1 approve1", r.ClassCounts)
	}
}

func TestP2_AgentAudit_Seeded(t *testing.T) {
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	out, err := runP2(t, flags, "agent-audit", "--window", "", "--db", dbPath)
	if err != nil {
		t.Fatalf("agent-audit errored: %v\n%s", err, out)
	}
	var r struct {
		TotalReads int `json:"total_reads"`
		Callers    []struct {
			Caller string `json:"caller"`
			Reads  int    `json:"reads"`
		} `json:"callers"`
	}
	mustJSON(t, out, &r)
	if r.TotalReads != 2 {
		t.Errorf("total_reads = %d, want 2", r.TotalReads)
	}
	if len(r.Callers) != 2 {
		t.Errorf("callers = %d, want 2", len(r.Callers))
	}
}

func TestP2_Unreads_Seeded(t *testing.T) {
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	// No channel cursor is set by seedMirror, so every message is unread.
	out, err := runP2(t, flags, "unreads", "--db", dbPath)
	if err != nil {
		t.Fatalf("unreads errored: %v\n%s", err, out)
	}
	var r unreadsReport
	mustJSON(t, out, &r)
	if r.TotalUnread != 3 {
		t.Errorf("total_unread = %d, want 3", r.TotalUnread)
	}
	if r.BucketCounts[bucketDM] != 1 {
		t.Errorf("dm bucket = %d, want 1", r.BucketCounts[bucketDM])
	}
	if r.BucketCounts[bucketInternal] != 2 {
		t.Errorf("internal bucket = %d, want 2", r.BucketCounts[bucketInternal])
	}

	// With every cursor advanced past the messages there are no unreads.
	db, _ := store.OpenWithContext(context.Background(), dbPath)
	_ = db.SetChannelCursor(context.Background(), "C100", "00009999.000000")
	_ = db.SetChannelCursor(context.Background(), "D200", "00009999.000000")
	db.Close()
	out, err = runP2(t, flags, "unreads", "--db", dbPath)
	if err != nil {
		t.Fatalf("unreads (cursor advanced) errored: %v\n%s", err, out)
	}
	var r2 unreadsReport
	mustJSON(t, out, &r2)
	if r2.TotalUnread != 0 {
		t.Errorf("after cursor advance total_unread = %d, want 0", r2.TotalUnread)
	}
	for _, b := range unreadBucketOrder {
		if r2.BucketCounts[b] != 0 {
			t.Errorf("bucket %q = %d, want 0 (honest empty)", b, r2.BucketCounts[b])
		}
	}
}

func TestP2_UsergroupsList_Seeded(t *testing.T) {
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	out, err := runP2(t, flags, "usergroups", "list", "--db", dbPath)
	if err != nil {
		t.Fatalf("usergroups list errored: %v\n%s", err, out)
	}
	var rows []usergroupRow
	mustJSON(t, out, &rows)
	if len(rows) != 1 {
		t.Fatalf("usergroups = %d, want 1", len(rows))
	}
	if rows[0].Handle != "csm-team" {
		t.Errorf("handle = %q, want csm-team", rows[0].Handle)
	}
	if rows[0].Mention != "@csm-team" {
		t.Errorf("mention = %q, want @csm-team (subteam render)", rows[0].Mention)
	}
	if len(rows[0].Members) != 1 || rows[0].Members[0] != "Marjorie" {
		t.Errorf("members = %v, want [Marjorie]", rows[0].Members)
	}
}

func TestP2_CustomerIntelDeep_SkipMissing_Seeded(t *testing.T) {
	// Pin sibling resolution to an empty temp dir so real pp-* mirrors
	// on the developer machine cannot leak into the assertion.
	t.Setenv("SLACK_PP_SIBLING_DIR", t.TempDir())
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	// No sibling DBs present in the temp env — verb must degrade.
	out, err := runP2(t, flags, "customer-intel-deep", "Sonria",
		"--skip-missing", "--window", "", "--db", dbPath)
	if err != nil {
		t.Fatalf("customer-intel-deep errored: %v\n%s", err, out)
	}
	var r customerIntelReport
	mustJSON(t, out, &r)
	if r.Customer != "Sonria" {
		t.Errorf("customer = %q, want Sonria", r.Customer)
	}
	// The seeded message "kickoff with Sonria today" must be on the timeline.
	if r.EventCount < 1 {
		t.Fatalf("expected at least 1 slack timeline event, got %d", r.EventCount)
	}
	foundSlack := false
	for _, e := range r.Timeline {
		if e.Source == "slack" && e.Permalink == "https://slack/p100" {
			foundSlack = true
		}
	}
	if !foundSlack {
		t.Errorf("expected the seeded Sonria slack mention on the timeline: %+v", r.Timeline)
	}
	// missing_sources MUST be present and list the absent siblings.
	if r.MissingSources == nil {
		t.Fatalf("missing_sources field absent — must be present even when empty")
	}
	wantMissing := map[string]bool{"attio": true, "asana": true, "fathom": true}
	for _, m := range r.MissingSources {
		delete(wantMissing, m)
	}
	if len(wantMissing) != 0 {
		t.Errorf("missing_sources = %v, expected attio/asana/fathom all listed", r.MissingSources)
	}
}

func TestP2_DMEngagement_Seeded(t *testing.T) {
	// Pin sibling resolution to an empty temp dir so real pp-* mirrors
	// on the developer machine cannot leak into the assertion.
	t.Setenv("SLACK_PP_SIBLING_DIR", t.TempDir())
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	out, err := runP2(t, flags, "dm-engagement", "--report", "all",
		"--window", "", "--db", dbPath)
	if err != nil {
		t.Fatalf("dm-engagement errored: %v\n%s", err, out)
	}
	var r dmEngagementReport
	mustJSON(t, out, &r)
	if len(r.Rows) != 1 {
		t.Fatalf("rows = %d, want 1 (U2 has a dm channel)", len(r.Rows))
	}
	if r.Rows[0].DMCount != 1 {
		t.Errorf("dm_count = %d, want 1", r.Rows[0].DMCount)
	}
	if r.MissingSources == nil {
		t.Errorf("missing_sources must be present (asana/fathom absent)")
	}
}

func TestP2_ActionFollowthrough_Seeded(t *testing.T) {
	// Pin sibling resolution to an empty temp dir so real pp-* mirrors
	// on the developer machine cannot leak into the assertion.
	t.Setenv("SLACK_PP_SIBLING_DIR", t.TempDir())
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	out, err := runP2(t, flags, "action-followthrough", "--report", "marjorie",
		"--window", "", "--db", dbPath)
	if err != nil {
		t.Fatalf("action-followthrough errored: %v\n%s", err, out)
	}
	var r actionFollowthroughReport
	mustJSON(t, out, &r)
	// No fathom mirror present — empty rows, fathom listed missing.
	if len(r.Rows) != 0 {
		t.Errorf("rows = %d, want 0 (no fathom mirror)", len(r.Rows))
	}
	foundFathom := false
	for _, m := range r.MissingSources {
		if m == "fathom" {
			foundFathom = true
		}
	}
	if !foundFathom {
		t.Errorf("missing_sources = %v, want fathom listed", r.MissingSources)
	}
}

func TestP2_GoalChannelPulse_NoRocksFile(t *testing.T) {
	// Pin sibling resolution to an empty temp dir so real pp-* mirrors
	// on the developer machine cannot leak into the assertion.
	t.Setenv("SLACK_PP_SIBLING_DIR", t.TempDir())
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	missingRocks := filepath.Join(t.TempDir(), "no-such-rocks.yml")
	out, err := runP2(t, flags, "goal-channel-pulse",
		"--rocks-file", missingRocks, "--db", dbPath)
	if err != nil {
		t.Fatalf("goal-channel-pulse errored: %v\n%s", err, out)
	}
	var r goalChannelPulseReport
	mustJSON(t, out, &r)
	if r.Note == "" {
		t.Errorf("expected an explanatory note when rocks.yml is absent")
	}
	if len(r.Pulses) != 0 {
		t.Errorf("pulses = %d, want 0 when rocks.yml is absent", len(r.Pulses))
	}
}

func TestP2_GoalChannelPulse_WithRocksFile(t *testing.T) {
	// Pin sibling resolution to an empty temp dir so real pp-* mirrors
	// on the developer machine cannot leak into the assertion.
	t.Setenv("SLACK_PP_SIBLING_DIR", t.TempDir())
	dbPath := seedMirror(t)
	flags := &rootFlags{asJSON: true}
	rocksPath := filepath.Join(t.TempDir(), "rocks.yml")
	if err := os.WriteFile(rocksPath,
		[]byte("rocks:\n  - rock: Wolf deal velocity\n    slack_channel: \"#the-wolf-of-atom\"\n"), 0o644); err != nil {
		t.Fatalf("write rocks.yml: %v", err)
	}
	out, err := runP2(t, flags, "goal-channel-pulse",
		"--rocks-file", rocksPath, "--window", "", "--db", dbPath)
	if err != nil {
		t.Fatalf("goal-channel-pulse errored: %v\n%s", err, out)
	}
	var r goalChannelPulseReport
	mustJSON(t, out, &r)
	if len(r.Pulses) != 1 {
		t.Fatalf("pulses = %d, want 1", len(r.Pulses))
	}
	p := r.Pulses[0]
	if !p.ChannelResolved || p.ChannelID != "C100" {
		t.Errorf("pulse channel not resolved: %+v", p)
	}
	if p.MessageCount != 2 {
		t.Errorf("message_count = %d, want 2", p.MessageCount)
	}
	if p.Stalled {
		t.Errorf("stalled = true, want false (2 messages)")
	}
}

// mustJSON decodes out into v, failing the test on a parse error.
func mustJSON(t *testing.T, out string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("output is not valid JSON: %v\n--- raw ---\n%s", err, out)
	}
}
