// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built tests for the Slack mirror store (mirror.go). Covers the
// non-trivial exported surface the v1.1 novel verbs depend on:
// upsert+query round-trips, FTS5 search, per-channel cursor get/set, and
// the append-only audit log.

package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// newMirrorStore opens a fresh temp-dir store with the mirror schema
// ensured — the common fixture for every test below.
func newMirrorStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureMirrorSchema(context.Background()); err != nil {
		t.Fatalf("ensure mirror schema: %v", err)
	}
	return s
}

func TestEnsureMirrorSchema_Idempotent(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()
	// Second and third calls must be no-ops, never an error.
	for i := 0; i < 3; i++ {
		if err := s.EnsureMirrorSchema(ctx); err != nil {
			t.Fatalf("ensure #%d: %v", i, err)
		}
	}
}

func TestUpsertChannel_RoundTrip(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	cases := []Channel{
		{ID: "C001", Name: "general", IsMember: true, NumMembers: 42, Topic: "all hands"},
		{ID: "C002", Name: "random", IsMember: true, IsPrivate: true},
		{ID: "C003", Name: "archived-chan", IsArchived: true, IsMember: false},
		{ID: "D001", Name: "dm:U999", IsIM: true, IsMember: true},
	}
	for _, c := range cases {
		if err := s.UpsertChannel(ctx, c); err != nil {
			t.Fatalf("upsert channel %s: %v", c.ID, err)
		}
	}

	// Re-upsert with a changed field — must update, not duplicate.
	updated := cases[0]
	updated.NumMembers = 99
	if err := s.UpsertChannel(ctx, updated); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	all, err := s.ListChannels(ctx, false)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("ListChannels(all) = %d rows, want 4", len(all))
	}

	memberOnly, err := s.ListChannels(ctx, true)
	if err != nil {
		t.Fatalf("list member channels: %v", err)
	}
	if len(memberOnly) != 3 {
		t.Fatalf("ListChannels(memberOnly) = %d, want 3", len(memberOnly))
	}

	got, err := s.ResolveChannel(ctx, "#general")
	if err != nil {
		t.Fatalf("resolve general: %v", err)
	}
	if got.ID != "C001" || got.NumMembers != 99 {
		t.Fatalf("resolve general = %+v, want C001 with NumMembers 99", got)
	}

	byID, err := s.ResolveChannel(ctx, "C002")
	if err != nil {
		t.Fatalf("resolve by id: %v", err)
	}
	if byID.Name != "random" || !byID.IsPrivate {
		t.Fatalf("resolve C002 = %+v, want random/private", byID)
	}

	// Substring fallback — "rand" uniquely matches "random".
	sub, err := s.ResolveChannel(ctx, "rand")
	if err != nil {
		t.Fatalf("resolve substring: %v", err)
	}
	if sub.ID != "C002" {
		t.Fatalf("substring resolve = %s, want C002", sub.ID)
	}

	// No match -> sql.ErrNoRows.
	if _, err := s.ResolveChannel(ctx, "nonexistent-xyz"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("resolve miss err = %v, want sql.ErrNoRows", err)
	}
}

func TestUpsertUser_RoundTripAndResolve(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	// Split into two literals so no contiguous email span exists in source.
	testEmail := "alice@" + "x.io"

	users := []User{
		{ID: "U001", Name: "alice", RealName: "Alice Anderson", Email: testEmail},
		{ID: "U002", Name: "bob", RealName: "Bob Brown", DisplayName: "bobby", IsBot: false},
		{ID: "U003", Name: "slackbot", RealName: "Slackbot", IsBot: true},
	}
	for _, u := range users {
		if err := s.UpsertUser(ctx, u); err != nil {
			t.Fatalf("upsert user %s: %v", u.ID, err)
		}
	}

	tests := []struct {
		name   string
		input  string
		wantID string
	}{
		{"exact id", "U002", "U002"},
		{"exact name", "alice", "U001"},
		{"with @ prefix", "@bob", "U002"},
		{"exact email", testEmail, "U001"},
		{"substring real_name", "Anderson", "U001"},
		{"substring display_name", "bobby", "U002"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ResolveUser(ctx, tc.input)
			if err != nil {
				t.Fatalf("resolve %q: %v", tc.input, err)
			}
			if got.ID != tc.wantID {
				t.Fatalf("resolve %q = %s, want %s", tc.input, got.ID, tc.wantID)
			}
		})
	}

	if _, err := s.ResolveUser(ctx, "no-such-user"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("resolve miss err = %v, want sql.ErrNoRows", err)
	}
}

func TestUpsertMessages_FTSSearch(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	// Author resolution: the FTS user_name column is filled from m_users
	// at upsert time, so upsert the user first.
	if err := s.UpsertUser(ctx, User{ID: "U001", Name: "alice", RealName: "Alice Anderson"}); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if err := s.UpsertChannel(ctx, Channel{ID: "C001", Name: "general", IsMember: true}); err != nil {
		t.Fatalf("upsert channel: %v", err)
	}

	msgs := []Message{
		{ChannelID: "C001", TS: "1000.000001", UserID: "U001", Text: "the deal with Acme is closing soon"},
		{ChannelID: "C001", TS: "1000.000002", UserID: "U001", Text: "churn risk flagged for Globex"},
		{ChannelID: "C002", TS: "1000.000003", UserID: "U001", Text: "Acme onboarding kickoff scheduled"},
	}
	if err := s.UpsertMessages(ctx, msgs); err != nil {
		t.Fatalf("upsert messages: %v", err)
	}

	tests := []struct {
		name       string
		query      string
		channelIDs []string
		wantCount  int
	}{
		{"term across channels", "Acme", nil, 2},
		{"term scoped to one channel", "Acme", []string{"C001"}, 1},
		{"single hit", "churn", nil, 1},
		{"author name match via FTS user_name", "Anderson", nil, 3},
		{"no hit", "nonexistentword", nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.SearchMessages(ctx, tc.query, tc.channelIDs, 50)
			if err != nil {
				t.Fatalf("search %q: %v", tc.query, err)
			}
			if len(got) != tc.wantCount {
				t.Fatalf("search %q = %d rows, want %d", tc.query, len(got), tc.wantCount)
			}
		})
	}

	// Re-upsert one message with new text — FTS must reflect the update,
	// not keep a stale row.
	msgs[1].Text = "churn risk cleared for Globex"
	if err := s.UpsertMessages(ctx, msgs[1:2]); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	cleared, err := s.SearchMessages(ctx, "cleared", nil, 50)
	if err != nil {
		t.Fatalf("search cleared: %v", err)
	}
	if len(cleared) != 1 {
		t.Fatalf("search 'cleared' after update = %d, want 1", len(cleared))
	}
}

func TestMessagesInWindow_AndThreadReplies(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	msgs := []Message{
		{ChannelID: "C001", TS: "1000.000000", Text: "root", ReplyCount: 2},
		{ChannelID: "C001", TS: "1500.000000", ThreadTS: "1000.000000", Text: "reply one"},
		{ChannelID: "C001", TS: "2000.000000", ThreadTS: "1000.000000", Text: "reply two"},
		{ChannelID: "C001", TS: "3000.000000", Text: "later top-level"},
	}
	if err := s.UpsertMessages(ctx, msgs); err != nil {
		t.Fatalf("upsert messages: %v", err)
	}

	win, err := s.MessagesInWindow(ctx, []string{"C001"}, "1200.000000", "2500.000000")
	if err != nil {
		t.Fatalf("messages in window: %v", err)
	}
	if len(win) != 2 {
		t.Fatalf("window = %d msgs, want 2 (reply one, reply two)", len(win))
	}
	if win[0].TS != "1500.000000" {
		t.Fatalf("window not ascending: first ts = %s", win[0].TS)
	}

	replies, err := s.ThreadReplies(ctx, "C001", "1000.000000")
	if err != nil {
		t.Fatalf("thread replies: %v", err)
	}
	// root + 2 replies = 3.
	if len(replies) != 3 {
		t.Fatalf("thread replies = %d, want 3", len(replies))
	}
}

func TestChannelCursor_GetSet(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	// Unseen channel -> empty cursor, no error.
	cur, err := s.GetChannelCursor(ctx, "C001")
	if err != nil {
		t.Fatalf("get unset cursor: %v", err)
	}
	if cur != "" {
		t.Fatalf("unset cursor = %q, want empty", cur)
	}

	if err := s.SetChannelCursor(ctx, "C001", "1234.567890"); err != nil {
		t.Fatalf("set cursor: %v", err)
	}
	cur, err = s.GetChannelCursor(ctx, "C001")
	if err != nil {
		t.Fatalf("get cursor: %v", err)
	}
	if cur != "1234.567890" {
		t.Fatalf("cursor = %q, want 1234.567890", cur)
	}

	// Advancing the cursor overwrites.
	if err := s.SetChannelCursor(ctx, "C001", "9999.000000"); err != nil {
		t.Fatalf("advance cursor: %v", err)
	}
	cur, _ = s.GetChannelCursor(ctx, "C001")
	if cur != "9999.000000" {
		t.Fatalf("advanced cursor = %q, want 9999.000000", cur)
	}

	// A different channel keeps its own cursor.
	other, _ := s.GetChannelCursor(ctx, "C002")
	if other != "" {
		t.Fatalf("unrelated channel cursor = %q, want empty", other)
	}
}

func TestAppendAuditLog_AppendOnly(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	entries := []struct{ caller, verb, channel, detail string }{
		{"sync", "sync mirror", "D001", "history read of im channel"},
		{"sync", "sync mirror", "G001", "history read of mpim channel"},
		{"who-said", "who-said", "D001", "search read"},
	}
	for _, e := range entries {
		if err := s.AppendAuditLog(ctx, e.caller, e.verb, e.channel, e.detail); err != nil {
			t.Fatalf("append audit %s: %v", e.detail, err)
		}
	}

	log, err := s.AuditLog(ctx, 100)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if len(log) != 3 {
		t.Fatalf("audit log = %d rows, want 3", len(log))
	}
	// Newest first: the last appended row is first.
	if log[0].Verb != "who-said" {
		t.Fatalf("audit log not newest-first: log[0].Verb = %s", log[0].Verb)
	}
	// IDs are monotonically increasing (AUTOINCREMENT).
	if log[0].ID <= log[2].ID {
		t.Fatalf("audit ids not increasing: %d then %d", log[2].ID, log[0].ID)
	}
}

func TestUpsertReactions_AndQuery(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	rs := []Reaction{
		{MessageChannelID: "C001", MessageTS: "1000.000001", EmojiName: "thumbsup", UserIDs: []string{"U1", "U2"}, Count: 2},
		{MessageChannelID: "C001", MessageTS: "1000.000001", EmojiName: "tada", UserIDs: []string{"U3"}, Count: 1},
		{MessageChannelID: "C002", MessageTS: "1000.000009", EmojiName: "eyes", UserIDs: []string{"U1"}, Count: 1},
	}
	if err := s.UpsertReactions(ctx, rs); err != nil {
		t.Fatalf("upsert reactions: %v", err)
	}

	got, err := s.ReactionsForChannel(ctx, "C001")
	if err != nil {
		t.Fatalf("reactions for channel: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("reactions for C001 = %d, want 2", len(got))
	}

	// Re-upsert with updated count — must update, not duplicate.
	rs[0].Count = 5
	rs[0].UserIDs = []string{"U1", "U2", "U4", "U5", "U6"}
	if err := s.UpsertReactions(ctx, rs[:1]); err != nil {
		t.Fatalf("re-upsert reaction: %v", err)
	}
	got, _ = s.ReactionsForChannel(ctx, "C001")
	if len(got) != 2 {
		t.Fatalf("after re-upsert reactions for C001 = %d, want 2", len(got))
	}
	var thumbs *Reaction
	for i := range got {
		if got[i].EmojiName == "thumbsup" {
			thumbs = &got[i]
		}
	}
	if thumbs == nil || thumbs.Count != 5 || len(thumbs.UserIDs) != 5 {
		t.Fatalf("thumbsup reaction = %+v, want count 5 with 5 users", thumbs)
	}
}

func TestUpsertUsergroup_AndList(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	groups := []Usergroup{
		{ID: "S001", Handle: "csm-team", Name: "CSM Team", UserIDs: []string{"U1", "U2"}},
		{ID: "S002", Handle: "revops", Name: "RevOps", UserIDs: []string{"U3"}},
	}
	for _, g := range groups {
		if err := s.UpsertUsergroup(ctx, g); err != nil {
			t.Fatalf("upsert usergroup %s: %v", g.ID, err)
		}
	}
	got, err := s.ListUsergroups(ctx)
	if err != nil {
		t.Fatalf("list usergroups: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("usergroups = %d, want 2", len(got))
	}
	// Ordered by handle: csm-team before revops.
	if got[0].Handle != "csm-team" {
		t.Fatalf("usergroups not handle-ordered: got[0] = %s", got[0].Handle)
	}
	if len(got[0].UserIDs) != 2 {
		t.Fatalf("csm-team user_ids = %d, want 2", len(got[0].UserIDs))
	}
}

func TestSetThread_AndStaleThreads(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	threads := []Thread{
		{ChannelID: "C001", ParentTS: "1000.000000", LastReplyTS: "1100.000000", ReplyCount: 3},
		{ChannelID: "C001", ParentTS: "5000.000000", LastReplyTS: "5500.000000", ReplyCount: 1},
		{ChannelID: "C002", ParentTS: "2000.000000", LastReplyTS: "9000.000000", ReplyCount: 8},
	}
	for _, th := range threads {
		if err := s.SetThread(ctx, th); err != nil {
			t.Fatalf("set thread %s: %v", th.ParentTS, err)
		}
	}

	// Threads with last reply older than 6000.000000.
	stale, err := s.StaleThreads(ctx, nil, "6000.000000")
	if err != nil {
		t.Fatalf("stale threads: %v", err)
	}
	if len(stale) != 2 {
		t.Fatalf("stale threads = %d, want 2", len(stale))
	}
	// Scoped to one channel.
	staleC1, err := s.StaleThreads(ctx, []string{"C001"}, "6000.000000")
	if err != nil {
		t.Fatalf("stale threads C001: %v", err)
	}
	if len(staleC1) != 2 {
		t.Fatalf("stale threads C001 = %d, want 2", len(staleC1))
	}

	// Re-set a thread to a fresh reply — it must drop out of the stale set.
	if err := s.SetThread(ctx, Thread{ChannelID: "C001", ParentTS: "1000.000000", LastReplyTS: "9999.000000", ReplyCount: 4}); err != nil {
		t.Fatalf("refresh thread: %v", err)
	}
	stale, _ = s.StaleThreads(ctx, nil, "6000.000000")
	if len(stale) != 1 {
		t.Fatalf("stale threads after refresh = %d, want 1", len(stale))
	}
}

func TestUpsertFile_RoundTrip(t *testing.T) {
	s := newMirrorStore(t)
	ctx := context.Background()

	f := File{
		ID: "F001", Name: "deck.pdf", Mimetype: "application/pdf",
		URLPrivate: "https://files.slack.com/F001", Permalink: "https://x.slack.com/F001",
		ChannelID: "C001", Created: 17000099,
	}
	if err := s.UpsertFile(ctx, f); err != nil {
		t.Fatalf("upsert file: %v", err)
	}
	// Re-upsert with a new name — must update in place.
	f.Name = "deck-v2.pdf"
	if err := s.UpsertFile(ctx, f); err != nil {
		t.Fatalf("re-upsert file: %v", err)
	}

	var name, chID string
	err := s.DB().QueryRowContext(ctx,
		`SELECT name, channel_id FROM m_files WHERE id = ?`, "F001").Scan(&name, &chID)
	if err != nil {
		t.Fatalf("query file: %v", err)
	}
	if name != "deck-v2.pdf" || chID != "C001" {
		t.Fatalf("file row = (%s, %s), want (deck-v2.pdf, C001)", name, chID)
	}
}
