// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for amend-2026-07-31: the --compact listing contract
// (search --agent emitted full _pp_body_text bodies per hit) and the batch
// read surface.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// corpusRow builds a JSON list item shaped like an archived Substack post:
// short identity fields plus the bulk fields --compact must never emit.
func corpusRow(slug string) map[string]any {
	return map[string]any{
		"slug":                slug,
		"title":               "Title for " + slug,
		"post_date":           "2026-07-24T12:00:00.000Z",
		"audience":            "only_paid",
		"canonical_url":       "https://example.substack.com/p/" + slug,
		"wordcount":           float64(1800),
		"_pp_body_text":       strings.Repeat("full article body ", 300),
		"body_html":           "<p>" + strings.Repeat("html ", 200) + "</p>",
		"truncated_body_text": "A short preview of the post.",
		"audio_items":         []any{map[string]any{"type": "voiceover", "url": "https://cdn.example/audio.mp3"}},
		"cover_image":         "https://cdn.example/cover.png",
		"appTheme":            map[string]any{"colors": map[string]any{"accent": "x"}},
	}
}

// TestCompactListStripsBulkCorpusFields is the regression test for the
// --compact contract: corpus rows carry the full indexed body in
// _pp_body_text (present in ~100% of rows, so the frequency-based keep rule
// re-admitted it), and `search --agent` returned entire articles per hit.
func TestCompactListStripsBulkCorpusFields(t *testing.T) {
	items := []map[string]any{corpusRow("a"), corpusRow("b"), corpusRow("c")}
	out := compactListFields(items)

	var rows []map[string]any
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("unmarshal compact output: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	for _, row := range rows {
		for _, banned := range []string{"_pp_body_text", "body_html", "truncated_body_text", "audio_items", "cover_image", "appTheme"} {
			if _, ok := row[banned]; ok {
				t.Fatalf("compact row still carries %q — the compact contract promises key fields only", banned)
			}
		}
		for _, want := range []string{"slug", "title", "post_date", "audience", "canonical_url"} {
			if _, ok := row[want]; !ok {
				t.Fatalf("compact row lost key field %q: %v", want, row)
			}
		}
		snip, ok := row["snippet"].(string)
		if !ok || snip == "" {
			t.Fatalf("compact row has no derived snippet: %v", row)
		}
		if len([]rune(snip)) > compactScalarMaxLen+1 { // +1 for the ellipsis rune
			t.Fatalf("snippet longer than the %d-rune bound: %d runes", compactScalarMaxLen, len([]rune(snip)))
		}
	}
}

// TestCompactListFrequencyRuleAdmitsOnlyShortScalars pins the data-driven
// keep rule's gate: a frequent key with object/array/long-string values must
// not ride the 80%-frequency extension back into compact output, while a
// frequent short scalar still does.
func TestCompactListFrequencyRuleAdmitsOnlyShortScalars(t *testing.T) {
	items := []map[string]any{}
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		items = append(items, map[string]any{
			"slug":       s,
			"reaction":   "❤️ " + s,                                        // short scalar, frequent -> kept
			"long_prose": strings.Repeat("word ", 100),                     // long string -> dropped
			"nested":     map[string]any{"deep": strings.Repeat("x", 500)}, // object -> dropped
		})
	}
	var rows []map[string]any
	if err := json.Unmarshal(compactListFields(items), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, row := range rows {
		if _, ok := row["reaction"]; !ok {
			t.Fatalf("frequent short scalar dropped: %v", row)
		}
		if _, ok := row["long_prose"]; ok {
			t.Fatalf("long string admitted by the frequency rule: %v", row)
		}
		if _, ok := row["nested"]; ok {
			t.Fatalf("object admitted by the frequency rule: %v", row)
		}
	}
}

// TestCompactListDropsAllNullKeys: a key that is null in every sampled row
// (Substack rows carry dozens: videoUpload, podcastPreviewUpload, ...) has no
// information and must not ride the frequency rule into compact output.
func TestCompactListDropsAllNullKeys(t *testing.T) {
	items := []map[string]any{}
	for _, s := range []string{"a", "b", "c"} {
		items = append(items, map[string]any{
			"slug":        s,
			"videoUpload": nil,
			"section":     nil,
		})
	}
	var rows []map[string]any
	if err := json.Unmarshal(compactListFields(items), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, row := range rows {
		if _, ok := row["videoUpload"]; ok {
			t.Fatalf("all-null key admitted: %v", row)
		}
		if _, ok := row["section"]; ok {
			t.Fatalf("all-null key admitted: %v", row)
		}
		if _, ok := row["slug"]; !ok {
			t.Fatalf("identity key lost: %v", row)
		}
	}
}

// TestCompactSnippetFallsBackAcrossProseFields covers the snippet source
// priority (_pp_body_text, then truncated_body_text, then description) and
// whitespace normalization.
func TestCompactSnippetFallsBackAcrossProseFields(t *testing.T) {
	if got := compactSnippet(map[string]any{"_pp_body_text": "line\none\n\n two "}); got != "line one two" {
		t.Fatalf("snippet = %q, want whitespace-collapsed body text", got)
	}
	if got := compactSnippet(map[string]any{"_pp_body_text": "![](https://cdn.example/cover.png) Real prose starts here."}); got != "Real prose starts here." {
		t.Fatalf("snippet = %q, want the leading markdown image stripped", got)
	}
	if got := compactSnippet(map[string]any{"truncated_body_text": "preview"}); got != "preview" {
		t.Fatalf("snippet = %q, want the truncated preview fallback", got)
	}
	if got := compactSnippet(map[string]any{"description": "desc"}); got != "desc" {
		t.Fatalf("snippet = %q, want the description fallback", got)
	}
	if got := compactSnippet(map[string]any{"wordcount": float64(3)}); got != "" {
		t.Fatalf("snippet = %q, want empty when no prose field exists", got)
	}
	long := strings.Repeat("a", compactScalarMaxLen+50)
	if got := compactSnippet(map[string]any{"description": long}); len([]rune(got)) != compactScalarMaxLen+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long snippet not bounded with ellipsis: %d runes", len([]rune(got)))
	}
}

// TestAgentEnvelopeHonorsLiveProvenance guards the meta plumbing read relies
// on: a live command passing {"source": "live"} must see exactly that in the
// agent envelope, not the printOutputWithFlags default of "local".
func TestAgentEnvelopeHonorsLiveProvenance(t *testing.T) {
	flags := &rootFlags{agent: true, asJSON: true}
	var out bytes.Buffer
	if err := printOutputWithFlagsMeta(&out, json.RawMessage(`{"slug":"x"}`), flags, map[string]any{"source": "live"}); err != nil {
		t.Fatalf("printOutputWithFlagsMeta: %v", err)
	}
	var envelope struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, out.String())
	}
	if envelope.Meta["source"] != "live" {
		t.Fatalf("meta.source = %v, want \"live\"", envelope.Meta["source"])
	}
}

// TestRunReadBatchMintsFreshContextPerPost is the regression test for the
// shared-deadline defect: a batch whose first post consumes more than one
// whole per-post budget must NOT starve the later posts. Each fetch must see
// its own fresh, unexpired context.
func TestRunReadBatchMintsFreshContextPerPost(t *testing.T) {
	const budget = 30 * time.Millisecond
	newCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), budget)
	}
	fetch := func(ctx context.Context, arg string) readResult {
		if arg == "slow" {
			// Burn more than the whole per-post budget, like a slow first post.
			select {
			case <-ctx.Done():
			case <-time.After(budget * 3):
			}
			return readResult{err: ctx.Err()}
		}
		if err := ctx.Err(); err != nil {
			return readResult{err: fmt.Errorf("inherited an expired deadline: %w", err)}
		}
		return readResult{envelope: map[string]any{"slug": arg}}
	}
	results := runReadBatch([]string{"slow", "second", "third"}, newCtx, fetch)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for i, r := range results[1:] {
		if r.err != nil {
			t.Fatalf("post %d failed after a slow first post: %v — per-post budgets must be independent", i+2, r.err)
		}
	}
}

// TestArchiveStopHintNamesWhoStopped: the user's --limit gets a rerun hint, the
// dogfood cap names itself (a --limit 0 caller must never be told to rerun
// with --limit 0), and an exhausted walk has no hint at all.
func TestArchiveStopHintNamesWhoStopped(t *testing.T) {
	if got := archiveStopHint(true, false, 50, 50); got != "" {
		t.Fatalf("exhausted walk produced a hint: %q", got)
	}
	got := archiveStopHint(false, false, 50, 50)
	if !strings.Contains(got, "--limit 50") || !strings.Contains(got, "rerun") {
		t.Fatalf("user-limit stop hint = %q, want a rerun hint naming --limit 50", got)
	}
	got = archiveStopHint(false, true, 0, 50)
	if !strings.Contains(got, "dogfood") || strings.Contains(got, "rerun with a higher --limit") {
		t.Fatalf("dogfood-cap stop hint = %q, want it to name the dogfood cap, not suggest a rerun", got)
	}
}

// TestReadRejectsZeroArgsShowsHelp and the dry-run case below smoke-test the
// batch surface without the network: `read` now accepts variadic posts.
func TestReadBatchDryRun(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"read", "pub/slug-one", "pub/slug-two", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("read multi-arg --dry-run error = %v (variadic args not wired?)", err)
	}
	if !strings.Contains(out.String(), "would fetch") {
		t.Fatalf("dry-run output missing: %s", out.String())
	}
}
