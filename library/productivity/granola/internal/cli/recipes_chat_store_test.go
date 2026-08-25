// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// PATCH(dual-path-store-read): regressions for the four cache-only commands
// (`recipes list`, `recipes describe`, `chat list`, `chat get`) after they were
// rerouted through the store read seam.
//
// The user-visible requirement is literal: on a migrated install the desktop
// cache never decrypts again, but the store already holds the recipes and chat
// threads the last cache sync wrote. These commands used to hard-fail on the
// decrypt; they must now serve what is local and say how old it is.

// runCLISplit is runCLI with stdout and stderr kept apart. The staleness
// notice is written to stderr precisely so it cannot corrupt the JSON on
// stdout, and a runner that merged the two streams could not tell the
// difference (nor could it json.Unmarshal the result).
func runCLISplit(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	rc := RootCmd()
	rc.SetOut(&out)
	rc.SetErr(&errOut)
	rc.SilenceUsage = true
	rc.SilenceErrors = true
	rc.SetArgs(append(args, "--no-refresh"))
	err := rc.Execute()
	return out.String(), errOut.String(), err
}

// isolateSyncState points ReadSyncState at a scratch file so a real
// ~/.local/share/granola-pp-cli/sync_state.json on the developer's machine
// cannot leak into an assertion. A non-zero at records a sync at that instant.
func isolateSyncState(t *testing.T, at time.Time) {
	t.Helper()
	t.Setenv("GRANOLA_SYNC_STATE_PATH", filepath.Join(t.TempDir(), "sync_state.json"))
	if at.IsZero() {
		return
	}
	if err := granola.WriteSyncState(granola.SyncState{
		LastSyncAt:        at,
		LastCacheSyncAt:   at,
		LastDecryptStatus: granola.DecryptStatusFailed,
	}); err != nil {
		t.Fatalf("WriteSyncState: %v", err)
	}
}

// stalenessBlock mirrors the JSON the commands attach. Declared in the test
// rather than reusing the production type so these tests assert the wire
// shape an agent actually parses, not the struct that happens to produce it.
type stalenessBlock struct {
	Source          string `json:"source"`
	LastCacheSyncAt string `json:"last_cache_sync_at"`
	Refreshable     bool   `json:"refreshable"`
	Notice          string `json:"notice"`
}

type recipeListEnvelope struct {
	Recipes   []map[string]any `json:"recipes"`
	Staleness *stalenessBlock  `json:"staleness"`
}

type chatListEnvelope struct {
	Threads   []map[string]any `json:"threads"`
	Staleness *stalenessBlock  `json:"staleness"`
}

func decodeRecipeList(t *testing.T, stdout string) recipeListEnvelope {
	t.Helper()
	var env recipeListEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal recipes list envelope: %v (stdout=%q)", err, stdout)
	}
	return env
}

func decodeChatList(t *testing.T, stdout string) chatListEnvelope {
	t.Helper()
	var env chatListEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal chat list envelope: %v (stdout=%q)", err, stdout)
	}
	return env
}

func recipeRowBySlug(rows []map[string]any, slug string) map[string]any {
	for _, r := range rows {
		if r["slug"] == slug {
			return r
		}
	}
	return nil
}

// TestRecipesList_ServedFromStore_NoCache is the headline R1 case: 57 recipes
// sit in the store on a real machine and `recipes list` used to refuse to show
// any of them because the cache would not decrypt.
func TestRecipesList_ServedFromStore_NoCache(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	seedRecipe(t, db, "rec_a", "action-items", "", "Pull the action items", "meeting", "public")
	seedRecipe(t, db, "rec_b", "weekly-sync", "Weekly Sync", "Recap the week", "", "user")

	stdout, _, err := runCLISplit(t, "recipes", "list", "--json")
	if err != nil {
		t.Fatalf("recipes list: %v (stdout=%q)", err, stdout)
	}
	env := decodeRecipeList(t, stdout)
	if len(env.Recipes) != 2 {
		t.Fatalf("expected 2 store recipes, got %d (%+v)", len(env.Recipes), env.Recipes)
	}
	a := recipeRowBySlug(env.Recipes, "action-items")
	if a == nil {
		t.Fatalf("action-items missing: %+v", env.Recipes)
	}
	if a["source"] != "public" {
		t.Errorf("action-items source = %v, want public", a["source"])
	}
	if a["description"] != "Pull the action items" {
		t.Errorf("action-items description = %v", a["description"])
	}
	// LoadCache defaults Name to Slug; a store row with an empty name column
	// must reach the command with the same default applied.
	if a["name"] != "action-items" {
		t.Errorf("action-items name = %v, want the slug default", a["name"])
	}

	// The --source filter must keep working against store-sourced rows.
	stdout, _, err = runCLISplit(t, "recipes", "list", "--source", "user", "--json")
	if err != nil {
		t.Fatalf("recipes list --source user: %v (stdout=%q)", err, stdout)
	}
	env = decodeRecipeList(t, stdout)
	if len(env.Recipes) != 1 || env.Recipes[0]["slug"] != "weekly-sync" {
		t.Errorf("--source user should select exactly weekly-sync, got %+v", env.Recipes)
	}
}

// TestChatList_ServedFromStore_NoCache: 225 chat threads in the store, zero
// shown before the reroute.
func TestChatList_ServedFromStore_NoCache(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	seedChatThread(t, db, "thr_1", "not_m1", "ws_1", "Thread One",
		"2026-05-01T10:00:00.000Z", "2026-05-01T10:05:00.000Z")
	seedChatMessage(t, db, "msg_a", "thr_1", "user", 0, "opening question", "2026-05-01T10:00:00.000Z")
	seedChatMessage(t, db, "msg_b", "thr_1", "assistant", 1, "the answer", "2026-05-01T10:01:00.000Z")

	stdout, _, err := runCLISplit(t, "chat", "list", "--json")
	if err != nil {
		t.Fatalf("chat list: %v (stdout=%q)", err, stdout)
	}
	env := decodeChatList(t, stdout)
	if len(env.Threads) != 1 {
		t.Fatalf("expected 1 store thread, got %d (%+v)", len(env.Threads), env.Threads)
	}
	th := env.Threads[0]
	if th["id"] != "thr_1" || th["title"] != "Thread One" || th["meeting_id"] != "not_m1" {
		t.Errorf("thread not reconstructed: %+v", th)
	}
	if th["workspace_id"] != "ws_1" || th["created_at"] != "2026-05-01T10:00:00.000Z" {
		t.Errorf("thread metadata lost: %+v", th)
	}
	if th["preview"] != "opening question" {
		t.Errorf("preview = %v, want the turn-0 message", th["preview"])
	}
	if count, _ := th["message_count"].(float64); count != 2 {
		t.Errorf("message_count = %v, want 2", th["message_count"])
	}
}

// TestChatGet_ServedFromStore_NoCache pins the per-thread dump.
func TestChatGet_ServedFromStore_NoCache(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	seedChatThread(t, db, "thr_1", "not_m1", "ws_1", "Thread One", "2026-05-01T10:00:00.000Z", "")
	seedChatMessage(t, db, "msg_c", "thr_1", "assistant", 2, "third", "2026-05-01T10:02:00.000Z")
	seedChatMessage(t, db, "msg_a", "thr_1", "user", 0, "first", "2026-05-01T10:00:00.000Z")
	seedChatMessage(t, db, "msg_b", "thr_1", "assistant", 1, "second", "2026-05-01T10:01:00.000Z")

	stdout, _, err := runCLISplit(t, "chat", "get", "thr_1", "--json")
	if err != nil {
		t.Fatalf("chat get: %v (stdout=%q)", err, stdout)
	}
	var got struct {
		ID        string                `json:"id"`
		Title     string                `json:"title"`
		MeetingID string                `json:"meeting_id"`
		Messages  []granola.ChatMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal chat get: %v (stdout=%q)", err, stdout)
	}
	if got.ID != "thr_1" || got.Title != "Thread One" || got.MeetingID != "not_m1" {
		t.Errorf("thread header wrong: %+v", got)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d (%+v)", len(got.Messages), got.Messages)
	}
	for i, want := range []string{"first", "second", "third"} {
		if got.Messages[i].Data.RawText != want {
			t.Errorf("message %d = %q, want %q (turn order broken)", i, got.Messages[i].Data.RawText, want)
		}
	}

	if _, _, err := runCLISplit(t, "chat", "get", "thr_missing", "--json"); err == nil {
		t.Error("expected an unknown thread id to be not-found")
	} else {
		var ce *cliError
		if !As(err, &ce) || ce.code != 3 {
			t.Errorf("expected a not-found cliError (code 3), got %#v", err)
		}
	}
}

// TestRecipesAndChatList_EmptyStore_ExitZero: a database that exists with zero
// rows is an empty answer, not a failure. This is the state a fresh install
// sits in between `sync-api` and `sync`.
func TestRecipesAndChatList_EmptyStore_ExitZero(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	withStoreDB(t, db, func(context.Context, *sql.DB) {}) // create the schema, seed nothing

	stdout, _, err := runCLISplit(t, "recipes", "list", "--json")
	if err != nil {
		t.Fatalf("recipes list on an empty store: %v (stdout=%q)", err, stdout)
	}
	if rows := decodeRecipeList(t, stdout).Recipes; len(rows) != 0 {
		t.Errorf("expected no recipes, got %+v", rows)
	}

	stdout, _, err = runCLISplit(t, "chat", "list", "--json")
	if err != nil {
		t.Fatalf("chat list on an empty store: %v (stdout=%q)", err, stdout)
	}
	if rows := decodeChatList(t, stdout).Threads; len(rows) != 0 {
		t.Errorf("expected no threads, got %+v", rows)
	}
}

// TestRecipesList_NoLocalDataAtAll_StillErrors: with no database and no
// readable cache there is genuinely nothing to show, and the error must stay
// an error rather than degrading into an empty success.
func TestRecipesList_NoLocalDataAtAll_StillErrors(t *testing.T) {
	newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")

	for _, args := range [][]string{
		{"recipes", "list", "--json"},
		{"recipes", "describe", "action-items", "--json"},
		{"chat", "list", "--json"},
		{"chat", "get", "thr_1", "--json"},
	} {
		stdout, _, err := runCLISplit(t, args...)
		if err == nil {
			t.Errorf("%v: expected an error with no store and no cache (stdout=%q)", args, stdout)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, "sync") {
			t.Errorf("%v: error should point at sync, got %q", args, msg)
		}
		if strings.Contains(msg, "safestorage") || strings.Contains(msg, "refresh refused") {
			t.Errorf("%v: error leaked the raw safestorage failure: %q", args, msg)
		}
		var ce *cliError
		if !As(err, &ce) || ce.code != 3 {
			t.Errorf("%v: expected a not-found cliError (code 3), got %#v", args, err)
		}
	}
}

// TestRecipesDescribe_UnknownSlug_NotFound: the reroute must not turn a miss
// into an empty success.
func TestRecipesDescribe_UnknownSlug_NotFound(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	seedRecipe(t, db, "rec_a", "action-items", "", "Pull the action items", "meeting", "public")

	stdout, _, err := runCLISplit(t, "recipes", "describe", "no-such-recipe", "--json")
	if err == nil {
		t.Fatalf("expected not-found for an unknown slug (stdout=%q)", stdout)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want a not-found message", err.Error())
	}
	var ce *cliError
	if !As(err, &ce) || ce.code != 3 {
		t.Errorf("expected a not-found cliError (code 3), got %#v", err)
	}
}

// TestRecipesDescribe_KnownSlug_StoreAndCacheMerge: the store answers the
// identity columns, the cache still supplies config.instructions (which has no
// store column at all), and describe must show both.
func TestRecipesDescribe_KnownSlug_StoreAndCacheMerge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	writeStateCache(t, home, map[string]any{
		"userRecipes": []map[string]any{{
			"id":   "rec_a",
			"slug": "weekly-sync",
			"config": map[string]any{
				"description":  "Cache description",
				"instructions": "Summarize the week in three bullets.",
			},
		}},
	})
	db := filepath.Join(home, ".local", "share", "granola-pp-cli", "data.db")
	seedRecipe(t, db, "rec_a", "weekly-sync", "Weekly Sync", "Store description", "meeting", "user")
	seedRecipeUsage(t, db, "rec_a", 7, "2026-05-01T10:00:00.000Z")

	stdout, stderr, err := runCLISplit(t, "recipes", "describe", "weekly-sync", "--json")
	if err != nil {
		t.Fatalf("recipes describe: %v (stdout=%q)", err, stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal describe: %v (stdout=%q)", err, stdout)
	}
	if got["id"] != "rec_a" || got["slug"] != "weekly-sync" || got["name"] != "Weekly Sync" {
		t.Errorf("identity fields wrong: %+v", got)
	}
	if got["instructions"] != "Summarize the week in three bullets." {
		t.Errorf("cache-only instructions lost: %v", got["instructions"])
	}
	if got["description"] != "Store description" {
		t.Errorf("store description should win, got %v", got["description"])
	}
	usage, _ := got["usage"].(map[string]any)
	if usage == nil || usage["count"] != "7" {
		t.Errorf("usage not carried through: %+v", got["usage"])
	}
	// A readable cache is not a frozen install.
	if _, ok := got["staleness"]; ok {
		t.Errorf("readable-cache install must not carry a staleness block: %+v", got)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("readable-cache install must not print a staleness notice, got %q", stderr)
	}
}

// TestRecipesList_TopUsage_OrdersNumerically: RecipeUsage.TotalCount is the
// cache's string, so a lexical sort would rank "3" above "12".
func TestRecipesList_TopUsage_OrdersNumerically(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	seedRecipe(t, db, "rec_small", "small", "Small", "", "", "user")
	seedRecipe(t, db, "rec_big", "big", "Big", "", "", "user")
	seedRecipe(t, db, "rec_none", "none", "None", "", "", "user")
	seedRecipeUsage(t, db, "rec_small", 3, "2026-05-01T10:00:00.000Z")
	seedRecipeUsage(t, db, "rec_big", 12, "2026-05-02T10:00:00.000Z")

	stdout, _, err := runCLISplit(t, "recipes", "list", "--top-usage", "--json")
	if err != nil {
		t.Fatalf("recipes list --top-usage: %v (stdout=%q)", err, stdout)
	}
	env := decodeRecipeList(t, stdout)
	if len(env.Recipes) != 3 {
		t.Fatalf("expected 3 rows, got %d (%+v)", len(env.Recipes), env.Recipes)
	}
	var order []string
	for _, r := range env.Recipes {
		order = append(order, r["slug"].(string))
	}
	if order[0] != "big" || order[1] != "small" || order[2] != "none" {
		t.Errorf("--top-usage order = %v, want [big small none] ('12' must outrank '3')", order)
	}
	if n, _ := env.Recipes[0]["usage_count"].(float64); n != 12 {
		t.Errorf("usage_count for big = %v, want 12", env.Recipes[0]["usage_count"])
	}
	if n, _ := env.Recipes[1]["usage_count"].(float64); n != 3 {
		t.Errorf("usage_count for small = %v, want 3", env.Recipes[1]["usage_count"])
	}
}

// TestChatList_AnchoredToMeeting_Filters keeps the positional meeting filter
// working off store-sourced threads.
func TestChatList_AnchoredToMeeting_Filters(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Time{})
	t.Setenv("GRANOLA_API_KEY", "")
	seedChatThread(t, db, "thr_1", "not_m1", "ws_1", "About M1", "2026-05-01T10:00:00.000Z", "")
	seedChatThread(t, db, "thr_2", "not_m2", "ws_1", "About M2", "2026-05-02T10:00:00.000Z", "")
	seedChatMessage(t, db, "msg_a", "thr_1", "user", 0, "m1 question", "")
	seedChatMessage(t, db, "msg_b", "thr_2", "user", 0, "m2 question", "")

	stdout, _, err := runCLISplit(t, "chat", "list", "not_m1", "--json")
	if err != nil {
		t.Fatalf("chat list not_m1: %v (stdout=%q)", err, stdout)
	}
	env := decodeChatList(t, stdout)
	if len(env.Threads) != 1 {
		t.Fatalf("expected 1 thread for not_m1, got %d (%+v)", len(env.Threads), env.Threads)
	}
	if env.Threads[0]["id"] != "thr_1" {
		t.Errorf("wrong thread: %+v", env.Threads[0])
	}
}

// TestStalenessDisclosed_WhenCacheUnreadable is R6: a command serving store
// data must say how current that data is, in the JSON an agent parses and on
// the stream a human reads.
func TestStalenessDisclosed_WhenCacheUnreadable(t *testing.T) {
	db := newGranolaFixture(t)
	lastSync := time.Date(2026, 6, 22, 17, 4, 5, 0, time.UTC)
	isolateSyncState(t, lastSync)
	t.Setenv("GRANOLA_API_KEY", "")
	seedRecipe(t, db, "rec_a", "action-items", "Action Items", "Pull the action items", "meeting", "public")
	seedChatThread(t, db, "thr_1", "not_m1", "ws_1", "Thread One", "2026-05-01T10:00:00.000Z", "")

	wantStamp := lastSync.Format(time.RFC3339)

	// recipes list
	stdout, stderr, err := runCLISplit(t, "recipes", "list", "--json")
	if err != nil {
		t.Fatalf("recipes list: %v", err)
	}
	st := decodeRecipeList(t, stdout).Staleness
	if st == nil {
		t.Fatalf("recipes list carried no staleness block: %s", stdout)
	}
	if st.Source != "store" {
		t.Errorf("recipes list staleness.source = %q, want store", st.Source)
	}
	if st.LastCacheSyncAt != wantStamp {
		t.Errorf("recipes list staleness.last_cache_sync_at = %q, want %q", st.LastCacheSyncAt, wantStamp)
	}
	if !strings.Contains(stderr, wantStamp) {
		t.Errorf("recipes list human notice missing the sync timestamp: %q", stderr)
	}

	// recipes describe
	stdout, stderr, err = runCLISplit(t, "recipes", "describe", "action-items", "--json")
	if err != nil {
		t.Fatalf("recipes describe: %v", err)
	}
	var described struct {
		Staleness *stalenessBlock `json:"staleness"`
	}
	if err := json.Unmarshal([]byte(stdout), &described); err != nil {
		t.Fatalf("unmarshal describe: %v (stdout=%q)", err, stdout)
	}
	if described.Staleness == nil {
		t.Fatalf("recipes describe carried no staleness block: %s", stdout)
	}
	if described.Staleness.LastCacheSyncAt != wantStamp {
		t.Errorf("recipes describe staleness.last_cache_sync_at = %q, want %q", described.Staleness.LastCacheSyncAt, wantStamp)
	}
	if !strings.Contains(stderr, wantStamp) {
		t.Errorf("recipes describe human notice missing the sync timestamp: %q", stderr)
	}

	// chat list
	stdout, stderr, err = runCLISplit(t, "chat", "list", "--json")
	if err != nil {
		t.Fatalf("chat list: %v", err)
	}
	chatSt := decodeChatList(t, stdout).Staleness
	if chatSt == nil {
		t.Fatalf("chat list carried no staleness block: %s", stdout)
	}
	if chatSt.LastCacheSyncAt != wantStamp {
		t.Errorf("chat list staleness.last_cache_sync_at = %q, want %q", chatSt.LastCacheSyncAt, wantStamp)
	}
	if !strings.Contains(stderr, wantStamp) {
		t.Errorf("chat list human notice missing the sync timestamp: %q", stderr)
	}
}

// TestChatList_DisclosesFrozenThreadSet: there is no chat endpoint on
// Granola's internal API, so re-running sync cannot advance chat_threads on a
// migrated install. An agent must be able to tell a frozen set from a current
// one without parsing prose, hence the refreshable flag.
func TestChatList_DisclosesFrozenThreadSet(t *testing.T) {
	db := newGranolaFixture(t)
	isolateSyncState(t, time.Date(2026, 6, 22, 17, 4, 5, 0, time.UTC))
	t.Setenv("GRANOLA_API_KEY", "")
	seedChatThread(t, db, "thr_1", "not_m1", "ws_1", "Thread One", "2026-05-01T10:00:00.000Z", "")

	stdout, stderr, err := runCLISplit(t, "chat", "list", "--json")
	if err != nil {
		t.Fatalf("chat list: %v", err)
	}
	st := decodeChatList(t, stdout).Staleness
	if st == nil {
		t.Fatalf("chat list carried no staleness block: %s", stdout)
	}
	if st.Refreshable {
		t.Error("chat list staleness.refreshable = true; chat threads cannot be refreshed")
	}
	if !strings.Contains(strings.ToLower(st.Notice), "refresh") {
		t.Errorf("chat list notice does not state the set cannot be refreshed: %q", st.Notice)
	}
	if !strings.Contains(strings.ToLower(stderr), "refresh") {
		t.Errorf("chat list human output does not state the set cannot be refreshed: %q", stderr)
	}

	// Recipes are refreshable by a future cache sync; only chat is frozen.
	seedRecipe(t, db, "rec_a", "action-items", "Action Items", "", "", "public")
	stdout, _, err = runCLISplit(t, "recipes", "list", "--json")
	if err != nil {
		t.Fatalf("recipes list: %v", err)
	}
	rst := decodeRecipeList(t, stdout).Staleness
	if rst == nil {
		t.Fatalf("recipes list carried no staleness block: %s", stdout)
	}
	if !rst.Refreshable {
		t.Error("recipes list staleness.refreshable = false; a cache sync can still advance recipes")
	}
}

// TestStalenessOmitted_WhenCacheReadable is requirement 7: the cache path is
// not frozen, so its output must stay byte-identical to what it always was -
// a bare JSON array, no envelope, no notice.
func TestStalenessOmitted_WhenCacheReadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	isolateSyncState(t, time.Date(2026, 6, 22, 17, 4, 5, 0, time.UTC))
	t.Setenv("GRANOLA_API_KEY", "")
	writeStateCache(t, home, map[string]any{
		"userRecipes": []map[string]any{{
			"id":     "rec_a",
			"slug":   "weekly-sync",
			"config": map[string]any{"description": "Cache description"},
		}},
		"entities": map[string]any{
			"chat_thread": map[string]any{
				"thr_1": map[string]any{
					"id":           "thr_1",
					"type":         "chat_thread",
					"workspace_id": "ws_1",
					"created_at":   "2026-05-01T10:00:00.000Z",
					"data":         map[string]any{"title": "Cache Thread", "document_id": "not_m1"},
				},
			},
			"chat_message": map[string]any{
				"msg_a": map[string]any{
					"id":   "msg_a",
					"data": map[string]any{"thread_id": "thr_1", "role": "user", "turn_index": 0, "raw_text": "cache first"},
				},
			},
		},
	})

	for _, args := range [][]string{
		{"recipes", "list", "--json"},
		{"chat", "list", "--json"},
	} {
		stdout, stderr, err := runCLISplit(t, args...)
		if err != nil {
			t.Fatalf("%v: %v (stdout=%q)", args, err, stdout)
		}
		var rows []map[string]any
		if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
			t.Fatalf("%v: readable-cache output must stay a bare array: %v (stdout=%q)", args, err, stdout)
		}
		if len(rows) != 1 {
			t.Errorf("%v: expected 1 cache row, got %d (%+v)", args, len(rows), rows)
		}
		if strings.Contains(stdout, "staleness") {
			t.Errorf("%v: readable-cache output must not carry a staleness block: %s", args, stdout)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Errorf("%v: readable-cache install must not print a staleness notice, got %q", args, stderr)
		}
	}
}
