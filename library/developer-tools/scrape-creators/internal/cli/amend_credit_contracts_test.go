// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the credit-spend contracts of the novel commands,
// exercised through the apiGetter seam with a scripted fake client and a
// temp SQLite store — no live calls, no credits.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

// scriptedGetter satisfies apiGetter with a per-call script and records every
// call it served, so tests can assert both behavior and spend shape.
type scriptedGetter struct {
	t      *testing.T
	script func(path string, params map[string]string) (json.RawMessage, error)
	calls  []scriptedCall
}

type scriptedCall struct {
	path   string
	params map[string]string
}

func (g *scriptedGetter) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	g.calls = append(g.calls, scriptedCall{path: path, params: params})
	return g.script(path, params)
}

func (g *scriptedGetter) callsTo(path string) int {
	n := 0
	for _, c := range g.calls {
		if c.path == path {
			n++
		}
	}
	return n
}

func commentObjs(prefix string, n int) string {
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, fmt.Sprintf(`{"id":"%s-%d","text":"c%d"}`, prefix, i, i))
	}
	return "[" + strings.Join(items, ",") + "]"
}

// --- account estimate: exit-7 boundary and balance parsing ---

func TestAccountEstimate_OverBudgetBoundary(t *testing.T) {
	balance := func(body string) *scriptedGetter {
		return &scriptedGetter{t: t, script: func(path string, _ map[string]string) (json.RawMessage, error) {
			if path != "/v1/account/credit-balance" {
				t.Fatalf("unexpected fetch %s", path)
			}
			return json.RawMessage(body), nil
		}}
	}

	// Projection strictly above the balance is over budget...
	out, err := runAccountEstimate(context.Background(), balance(`{"creditCount":100}`), estimatePlan{calls: 101, creditsPerCall: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !out.OverBudget {
		t.Errorf("101 credits against balance 100 must be over budget (envelope: %+v)", out)
	}
	if out.Remaining != -1 {
		t.Errorf("remaining_after_run = %d, want -1", out.Remaining)
	}

	// ...a projection that lands exactly on the balance is allowed.
	out, err = runAccountEstimate(context.Background(), balance(`{"creditCount":100}`), estimatePlan{calls: 100, creditsPerCall: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.OverBudget {
		t.Errorf("100 credits against balance 100 must NOT be over budget (envelope: %+v)", out)
	}
}

func TestAccountEstimate_BalanceShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int64
	}{
		{"live creditCount number", `{"creditCount":14000}`, 14000},
		{"live creditCount string", `{"creditCount":"14000"}`, 14000},
		{"legacy balance", `{"balance":250}`, 250},
		{"legacy credits", `{"credits":42}`, 42},
	}
	for _, tc := range cases {
		got, err := parseCreditBalance(json.RawMessage(tc.body))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: balance = %d, want %d", tc.name, got, tc.want)
		}
	}

	// A zero/unparsable balance is an error, never a spurious exit-7 signal.
	if _, err := parseCreditBalance(json.RawMessage(`{"unexpected":"shape"}`)); err == nil {
		t.Error("unparsable balance envelope must error instead of reporting balance 0")
	}
	g := &scriptedGetter{t: t, script: func(string, map[string]string) (json.RawMessage, error) {
		return json.RawMessage(`{"creditCount":0}`), nil
	}}
	if _, err := runAccountEstimate(context.Background(), g, estimatePlan{calls: 1}); err == nil {
		t.Error("zero balance must surface as an error, not an over-budget envelope")
	}
}

func TestAccountEstimate_PerCommentUsesFlatUpperBound(t *testing.T) {
	_, projected, breakdown := estimatePlan{posts: 4, withReplies: "per-comment"}.projection()
	if projected != 4*creditsIncludeReplies {
		t.Errorf("per-comment projection = %d, want the flat upper bound %d", projected, 4*creditsIncludeReplies)
	}
	if !strings.Contains(breakdown, "upper bound") {
		t.Errorf("breakdown must name the upper-bound assumption, got %q", breakdown)
	}
}

// --- comments thread: route selection ---

func threadGetter(t *testing.T, topLevel int, extra string) *scriptedGetter {
	return &scriptedGetter{t: t, script: func(path string, params map[string]string) (json.RawMessage, error) {
		switch path {
		case "/v2/instagram/post/comments":
			return json.RawMessage(fmt.Sprintf(`{"comments":%s,"credits_charged":1%s}`, commentObjs("c", topLevel), extra)), nil
		case "/v1/instagram/post/comment/replies":
			return json.RawMessage(`{"replies":[],"credits_charged":1}`), nil
		}
		t.Fatalf("unexpected fetch %s", path)
		return nil, nil
	}}
}

func TestThreadRouting_AutoThresholds(t *testing.T) {
	cases := []struct {
		topLevel  int
		wantRoute string
	}{
		{includeRepliesFlatCredits + 1, "flat"},
		{includeRepliesFlatCredits, "flat"},
		{includeRepliesFlatCredits - 1, "per-comment"},
	}
	for _, tc := range cases {
		g := threadGetter(t, tc.topLevel, "")
		out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "auto"})
		if err != nil {
			t.Fatalf("topLevel=%d: %v", tc.topLevel, err)
		}
		if out.Route != tc.wantRoute {
			t.Errorf("topLevel=%d: route = %s, want %s (reason: %s)", tc.topLevel, out.Route, tc.wantRoute, out.RouteReason)
		}
		if tc.wantRoute == "per-comment" {
			if got := g.callsTo("/v1/instagram/post/comment/replies"); got != tc.topLevel {
				t.Errorf("topLevel=%d: reply fetches = %d, want one per comment (%d)", tc.topLevel, got, tc.topLevel)
			}
		}
	}
}

func TestThreadForcedFlat_SkipsProbe(t *testing.T) {
	g := threadGetter(t, 3, "")
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "flat"})
	if err != nil {
		t.Fatal(err)
	}
	if got := g.callsTo("/v2/instagram/post/comments"); got != 1 {
		t.Errorf("forced flat must cost exactly one comments fetch, made %d", got)
	}
	if g.calls[0].params["include_replies"] != "true" {
		t.Errorf("forced flat must set include_replies=true, params: %v", g.calls[0].params)
	}
	if out.Route != "flat" || out.RouteReason != "forced by --route" {
		t.Errorf("route/reason = %s/%s, want flat/forced by --route", out.Route, out.RouteReason)
	}
}

// --- comments thread: budgeted traversal (F3) ---

// pagedThreadGetter serves pageCount comment pages; every page but the last
// reports has_more with a cursor. Each page charges pageCost credits.
func pagedThreadGetter(t *testing.T, pageCount, perPage int, pageCost int64) *scriptedGetter {
	return &scriptedGetter{t: t, script: func(path string, params map[string]string) (json.RawMessage, error) {
		switch path {
		case "/v2/instagram/post/comments":
			page := 1
			if cur := params["cursor"]; cur != "" {
				fmt.Sscanf(cur, "cur-%d", &page)
			}
			hasMore := page < pageCount
			body := fmt.Sprintf(`{"comments":%s,"credits_charged":%d,"has_more":%t`, commentObjs(fmt.Sprintf("p%d", page), perPage), pageCost, hasMore)
			if hasMore {
				body += fmt.Sprintf(`,"cursor":"cur-%d"`, page+1)
			}
			return json.RawMessage(body + "}"), nil
		case "/v1/instagram/post/comment/replies":
			return json.RawMessage(`{"replies":[],"credits_charged":1}`), nil
		}
		t.Fatalf("unexpected fetch %s", path)
		return nil, nil
	}}
}

func TestThreadDefaultKeepsSinglePageAndReportsTruncation(t *testing.T) {
	g := pagedThreadGetter(t, 3, 16, 1)
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	// probe + include_replies refetch, no page traversal without --max-credits
	if got := g.callsTo("/v2/instagram/post/comments"); got != 2 {
		t.Errorf("default run made %d comments fetches, want 2 (probe + flat)", got)
	}
	if !out.Truncated {
		t.Error("has_more pages left behind must set truncated")
	}
	if !strings.Contains(out.Note, "--max-credits") {
		t.Errorf("truncation note must point at --max-credits, got %q", out.Note)
	}
}

func TestThreadTraversal_CompletesWithinBudget(t *testing.T) {
	g := pagedThreadGetter(t, 3, 16, 1)
	out, rows, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "flat", maxCredits: 100})
	if err != nil {
		t.Fatal(err)
	}
	if out.Truncated {
		t.Errorf("all pages fit the budget; truncated must be false (note: %s)", out.Note)
	}
	if out.TopLevel != 48 {
		t.Errorf("top_level_comments = %d, want 48 (3 pages x 16)", out.TopLevel)
	}
	if out.PagesFetched != 3 {
		t.Errorf("pages_fetched = %d, want 3", out.PagesFetched)
	}
	if out.CreditsCharged != 3 {
		t.Errorf("credits_charged = %d, want 3", out.CreditsCharged)
	}
	if len(rows) != 48 {
		t.Errorf("store rows = %d, want 48", len(rows))
	}
}

func TestThreadTraversal_BudgetStopKeepsTruncatedTrue(t *testing.T) {
	// Every page costs 15 (flat pricing); budget 20 admits the first page
	// (charged 15), then the worst-case estimate (15) no longer fits: the
	// traversal must stop BEFORE paying for page 2 and stay truncated.
	g := pagedThreadGetter(t, 5, 16, 15)
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "flat", maxCredits: 20})
	if err != nil {
		t.Fatal(err)
	}
	if got := g.callsTo("/v2/instagram/post/comments"); got != 1 {
		t.Errorf("made %d comments fetches, want 1 (budget refuses page 2)", got)
	}
	if !out.Truncated {
		t.Error("budget stop with pages remaining must keep truncated=true")
	}
	if !strings.Contains(out.Note, "--max-credits 20") {
		t.Errorf("note must explain the budget stop, got %q", out.Note)
	}
	if out.CreditsCharged != 15 {
		t.Errorf("credits_charged = %d, want 15", out.CreditsCharged)
	}
}

func TestThreadTraversal_BreachHaltsImmediately(t *testing.T) {
	// The first page prices above the whole budget: the breach must be
	// reported and no further page bought, even though has_more is true.
	g := pagedThreadGetter(t, 5, 16, 30)
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "flat", maxCredits: 20})
	if err != nil {
		t.Fatal(err)
	}
	if got := g.callsTo("/v2/instagram/post/comments"); got != 1 {
		t.Errorf("made %d comments fetches after a breach, want 1", got)
	}
	if !strings.Contains(out.Note, "budget exceeded") {
		t.Errorf("note must report the breach, got %q", out.Note)
	}
	if out.CreditsCharged != 30 {
		t.Errorf("credits_charged = %d, want the honest overshoot 30", out.CreditsCharged)
	}
}

func TestThreadTraversal_CyclicCursorStopsWithoutRebuying(t *testing.T) {
	// The endpoint always reports has_more=true and always serves the SAME
	// cursor: page 2's envelope points back at cursor "cur-loop". The
	// traversal must detect the repeat and stop after buying each page once,
	// keeping truncated=true with a note naming the cycle — never re-buying
	// the same page until the budget or the page cap intervenes.
	g := &scriptedGetter{t: t, script: func(path string, params map[string]string) (json.RawMessage, error) {
		if path != "/v2/instagram/post/comments" {
			t.Fatalf("unexpected fetch %s", path)
		}
		return json.RawMessage(fmt.Sprintf(
			`{"comments":%s,"credits_charged":1,"has_more":true,"cursor":"cur-loop"}`,
			commentObjs("c", 16))), nil
	}}
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "flat", maxCredits: 100})
	if err != nil {
		t.Fatal(err)
	}
	// Page 1 (no cursor) + page 2 (cursor cur-loop); the second occurrence of
	// cur-loop is refused before any fetch.
	if got := g.callsTo("/v2/instagram/post/comments"); got != 2 {
		t.Errorf("made %d comments fetches, want 2 (cycle detected before re-buying)", got)
	}
	if out.CreditsCharged != 2 {
		t.Errorf("credits_charged = %d, want 2 (no charge for the repeated cursor)", out.CreditsCharged)
	}
	if !out.Truncated {
		t.Error("cycle stop must keep truncated=true")
	}
	if !strings.Contains(out.Note, "already served") {
		t.Errorf("note must name the cursor cycle, got %q", out.Note)
	}
	if strings.Contains(out.Note, "--max-credits") {
		t.Errorf("a cursor cycle is not a budget problem; note must not suggest raising the budget, got %q", out.Note)
	}
}

func TestThreadTraversal_PageFailureIsDiagnosedHonestly(t *testing.T) {
	// Page 2's fetch fails outright: the note must name the failed page as
	// the cause and must NOT suggest raising --max-credits — the stop has
	// nothing to do with the budget.
	g := &scriptedGetter{t: t, script: func(path string, params map[string]string) (json.RawMessage, error) {
		if path != "/v2/instagram/post/comments" {
			t.Fatalf("unexpected fetch %s", path)
		}
		if params["cursor"] != "" {
			return nil, fmt.Errorf("HTTP 502 from comments endpoint")
		}
		return json.RawMessage(fmt.Sprintf(
			`{"comments":%s,"credits_charged":1,"has_more":true,"cursor":"cur-2"}`,
			commentObjs("c", 16))), nil
	}}
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "flat", maxCredits: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Truncated {
		t.Error("failed page with pages remaining must keep truncated=true")
	}
	if !strings.Contains(out.Note, "page 2 fetch failed") {
		t.Errorf("note must name the failed page as the cause, got %q", out.Note)
	}
	if strings.Contains(out.Note, "--max-credits") {
		t.Errorf("a page failure is not a budget problem; note must not suggest raising the budget, got %q", out.Note)
	}
	if len(out.FetchFailures) != 1 || out.FetchFailures[0].Source != "page 2" {
		t.Errorf("fetch_failures must record the failed page, got %+v", out.FetchFailures)
	}
}

func TestThreadAllRepliesFailedPlusPageFailure_CarriesBothDiagnoses(t *testing.T) {
	// Combined failure: every reply fetch fails AND the next top-level page
	// fetch fails. The all-replies-failed hard error must not swallow the
	// page-failure diagnosis: the returned error carries both causes, and the
	// envelope keeps the page failure in note and fetch_failures.
	g := &scriptedGetter{t: t, script: func(path string, params map[string]string) (json.RawMessage, error) {
		switch path {
		case "/v2/instagram/post/comments":
			if params["cursor"] != "" {
				return nil, fmt.Errorf("HTTP 502 from comments endpoint")
			}
			return json.RawMessage(fmt.Sprintf(
				`{"comments":%s,"credits_charged":1,"has_more":true,"cursor":"cur-2"}`,
				commentObjs("c", 3))), nil
		case "/v1/instagram/post/comment/replies":
			return nil, fmt.Errorf("HTTP 500 from replies endpoint")
		}
		t.Fatalf("unexpected fetch %s", path)
		return nil, nil
	}}
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "per-comment", maxCredits: 100})
	if err == nil {
		t.Fatal("all reply fetches failed: a hard error is required")
	}
	if !strings.Contains(err.Error(), "all 3 source fetch(es) failed") {
		t.Errorf("error must carry the all-replies-failed diagnosis, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "page 2 fetch failed") {
		t.Errorf("error must also carry the page-failure diagnosis, got %q", err.Error())
	}
	if !strings.Contains(out.Note, "page 2 fetch failed") {
		t.Errorf("envelope note must keep the page-failure cause, got %q", out.Note)
	}
	// fetch_failures records the 3 reply failures plus the failed page.
	if len(out.FetchFailures) != 4 {
		t.Errorf("fetch_failures = %d entries, want 4 (3 replies + page 2): %+v", len(out.FetchFailures), out.FetchFailures)
	}

	// The RunE error path serializes this envelope to stdout before exiting
	// non-zero: the structured diagnoses must reach stdout consumers, not
	// just the error text. Assert the exact bytes a --json caller would see.
	var buf bytes.Buffer
	if perr := printPartialThreadEnvelope(&buf, out, &rootFlags{asJSON: true}); perr != nil {
		t.Fatalf("printing the partial envelope: %v", perr)
	}
	var printed struct {
		Note          string         `json:"note"`
		FetchFailures []fetchFailure `json:"fetch_failures"`
		Truncated     bool           `json:"truncated"`
	}
	if uerr := json.Unmarshal(buf.Bytes(), &printed); uerr != nil {
		t.Fatalf("stdout envelope is not valid JSON: %v\n%s", uerr, buf.String())
	}
	if !strings.Contains(printed.Note, "page 2 fetch failed") {
		t.Errorf("stdout envelope note must carry the page-failure cause, got %q", printed.Note)
	}
	if len(printed.FetchFailures) != 4 {
		t.Errorf("stdout envelope fetch_failures = %d entries, want 4: %s", len(printed.FetchFailures), buf.String())
	}

	// An empty envelope (e.g. first probe failed outright) stays silent on
	// stdout: error-only behavior is preserved when there is nothing to report.
	var empty bytes.Buffer
	if perr := printPartialThreadEnvelope(&empty, threadEnvelope{PostURL: "https://x/p/1"}, &rootFlags{asJSON: true}); perr != nil {
		t.Fatalf("printing the empty envelope: %v", perr)
	}
	if empty.Len() != 0 {
		t.Errorf("empty envelope must print nothing, got %s", empty.String())
	}
}

func TestThreadTraversal_PerCommentReplyFetchesAreGated(t *testing.T) {
	// per-comment route, 10 comments per page at 1 cr each + 1 cr per reply
	// call: budget 6 admits the probe (1 cr) and then reply fetches until the
	// worst-case estimate no longer fits. Spend must never exceed max here
	// (all costs equal the estimate), and the note must name the stop.
	g := pagedThreadGetter(t, 2, 10, 1)
	out, _, err := fetchCommentThread(context.Background(), g, threadFetchOpts{postURL: "https://x/p/1", route: "per-comment", maxCredits: 6})
	if err != nil {
		t.Fatal(err)
	}
	if out.CreditsCharged > 6 {
		t.Errorf("credits_charged = %d, must stay within --max-credits 6 when every fetch costs the estimate", out.CreditsCharged)
	}
	if got := g.callsTo("/v1/instagram/post/comment/replies"); got != 5 {
		t.Errorf("reply fetches = %d, want 5 (probe 1 cr + 5 replies = 6)", got)
	}
	if !strings.Contains(out.Note, "reply fetch") {
		t.Errorf("note must name the gated fetch kind, got %q", out.Note)
	}
}

// --- comments sweep: budget gating end-to-end (F1's reachable gates) ---

func sweepTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "sweep-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.EnsureCommentCorpus(context.Background(), db.DB()); err != nil {
		t.Fatal(err)
	}
	return db
}

// sweepGetter serves one posts page with n posts, then per-post comments at
// commentCost credits each.
func sweepGetter(t *testing.T, posts int, commentCost int64) *scriptedGetter {
	return &scriptedGetter{t: t, script: func(path string, params map[string]string) (json.RawMessage, error) {
		switch path {
		case "/v2/instagram/user/posts":
			items := make([]string, 0, posts)
			for i := 0; i < posts; i++ {
				items = append(items, fmt.Sprintf(`{"url":"https://x/p/%d"}`, i))
			}
			return json.RawMessage(`{"posts":[` + strings.Join(items, ",") + `],"credits_charged":1}`), nil
		case "/v2/instagram/post/comments":
			return json.RawMessage(fmt.Sprintf(`{"comments":%s,"credits_charged":%d}`, commentObjs("s", 2), commentCost)), nil
		}
		t.Fatalf("unexpected fetch %s", path)
		return nil, nil
	}}
}

func TestSweep_BudgetGatesCommentFetches(t *testing.T) {
	// Posts fetch charges 1, each comments fetch charges 4 and widens the
	// estimate: budget 10 admits post 1 (charged 5) and post 2 would fit only
	// if 5+4 <= 10... it does not (estimate is 4, 5+4=9 <= 10 admits post 2 →
	// charged 9; post 3 needs 9+4=13 > 10 → refused). The sweep must stop at
	// the gate with the stop note, not silently.
	g := sweepGetter(t, 5, 4)
	db := sweepTestDB(t)
	env, err := runCommentsSweep(context.Background(), g, db.DB(), sweepOpts{handle: "h", budget: newSweepBudget(10)})
	if err != nil {
		t.Fatal(err)
	}
	if env.PostsScanned != 2 {
		t.Errorf("posts_scanned = %d, want 2 (third comments fetch refused by the gate)", env.PostsScanned)
	}
	if !env.StoppedEarly {
		t.Error("budget refusal must set stopped_early")
	}
	if !strings.Contains(env.Note, "comments fetch") {
		t.Errorf("note must name the refused fetch kind, got %q", env.Note)
	}
	if env.CreditsCharged != 9 {
		t.Errorf("credits_charged = %d, want 9", env.CreditsCharged)
	}
}

func TestSweep_BreachHaltsImmediately(t *testing.T) {
	// A comments fetch pricing far above its estimate must halt the sweep the
	// moment the charge lands, reporting the breach — the overshoot never
	// compounds across further posts.
	g := sweepGetter(t, 5, 50)
	db := sweepTestDB(t)
	env, err := runCommentsSweep(context.Background(), g, db.DB(), sweepOpts{handle: "h", budget: newSweepBudget(20)})
	if err != nil {
		t.Fatal(err)
	}
	if env.PostsScanned != 1 {
		t.Errorf("posts_scanned = %d, want 1 (halt on breach)", env.PostsScanned)
	}
	if !env.StoppedEarly || !strings.Contains(env.Note, "budget exceeded") {
		t.Errorf("breach must stop the sweep with the breach note, got stopped=%t note=%q", env.StoppedEarly, env.Note)
	}
	if got := g.callsTo("/v2/instagram/post/comments"); got != 1 {
		t.Errorf("comments fetches after breach = %d, want 1", got)
	}
}

func TestSweep_MaxPostsStops(t *testing.T) {
	g := sweepGetter(t, 5, 1)
	db := sweepTestDB(t)
	env, err := runCommentsSweep(context.Background(), g, db.DB(), sweepOpts{handle: "h", maxPosts: 2, budget: newSweepBudget(0)})
	if err != nil {
		t.Fatal(err)
	}
	if env.PostsScanned != 2 || !env.StoppedEarly || !strings.Contains(env.Note, "--max-posts 2") {
		t.Errorf("max-posts stop broken: scanned=%d stopped=%t note=%q", env.PostsScanned, env.StoppedEarly, env.Note)
	}
	if env.CommentsStored != 4 {
		t.Errorf("comments_stored = %d, want 4 (2 posts x 2 comments)", env.CommentsStored)
	}
}
