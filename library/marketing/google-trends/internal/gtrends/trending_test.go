// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package gtrends

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestExtractTrendingSessionTokensJSONShape(t *testing.T) {
	html := `<script>window.WIZ_global_data = {"f.sid":"8817470312252861603","cfb2h":"boq_trends-boq-servers-frontend_20260712.06_p0"};</script>`
	sid, bl, err := extractTrendingSessionTokens(html)
	if err != nil {
		t.Fatalf("extractTrendingSessionTokens: %v", err)
	}
	if sid != "8817470312252861603" {
		t.Errorf("sid = %q, want %q", sid, "8817470312252861603")
	}
	if bl != "boq_trends-boq-servers-frontend_20260712.06_p0" {
		t.Errorf("bl = %q, want the build label", bl)
	}
}

func TestExtractTrendingSessionTokensQueryStringShape(t *testing.T) {
	html := `<a href="/_/TrendsUi/data/batchexecute?f.sid=-123456&bl=boq_trends-boq-servers-frontend_20260101.00_p1">link</a>`
	sid, bl, err := extractTrendingSessionTokens(html)
	if err != nil {
		t.Fatalf("extractTrendingSessionTokens: %v", err)
	}
	if sid != "-123456" {
		t.Errorf("sid = %q, want %q", sid, "-123456")
	}
	if bl != "boq_trends-boq-servers-frontend_20260101.00_p1" {
		t.Errorf("bl = %q, want the build label", bl)
	}
}

func TestExtractTrendingSessionTokensMissing(t *testing.T) {
	_, _, err := extractTrendingSessionTokens(`<html><body>Google has changed this page entirely</body></html>`)
	if !errors.Is(err, ErrTrendingScrapeFailed) {
		t.Fatalf("expected ErrTrendingScrapeFailed, got %v", err)
	}
}

// batchexecuteFixture builds a realistic-shaped batchexecute response: a
// length-prefix line followed by the "wrb.fr" frame whose third element is a
// JSON-encoded STRING containing the nested story-list payload.
func batchexecuteFixture(t *testing.T, storyTerms []string) []byte {
	t.Helper()
	stories := make([]any, 0, len(storyTerms))
	for _, term := range storyTerms {
		// Each "story" is itself an array; the display term is nested one
		// level in, matching the bounded-depth search in
		// findFirstPlausibleString.
		stories = append(stories, []any{[]any{term, nil}, []any{"/trending/explore?q=" + term}})
	}
	inner := []any{stories}
	innerJSON := mustJSON(t, inner)

	frame := []any{"wrb.fr", "DqDTgb", string(innerJSON), nil, nil, nil, "generic"}
	outer := []any{frame}
	outerJSON := mustJSON(t, outer)

	body := ")]}'\n" + "123\n" + string(outerJSON) + "\n"
	return []byte(body)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return b
}

func TestParseTrendingBatchExecuteStrictFrame(t *testing.T) {
	body := batchexecuteFixture(t, []string{"Term One", "Term Two", "Term Three"})
	topics, err := parseTrendingBatchExecute(body)
	if err != nil {
		t.Fatalf("parseTrendingBatchExecute: %v", err)
	}
	if len(topics) != 3 {
		t.Fatalf("expected 3 topics, got %d: %+v", len(topics), topics)
	}
	for i, want := range []string{"Term One", "Term Two", "Term Three"} {
		if topics[i].Term != want {
			t.Errorf("topics[%d].Term = %q, want %q", i, topics[i].Term, want)
		}
		if topics[i].Rank != i+1 {
			t.Errorf("topics[%d].Rank = %d, want %d", i, topics[i].Rank, i+1)
		}
	}
}

func TestParseTrendingBatchExecuteHeuristicFallback(t *testing.T) {
	// A body that doesn't match the strict wrb.fr frame shape at all, but
	// still contains quoted, plausible-looking terms — the fallback path.
	body := []byte(`)]}'
garbage-that-is-not-a-frame
["some other unrelated data", {"nested": "value here"}]
`)
	topics, err := parseTrendingBatchExecute(body)
	if err != nil {
		t.Fatalf("parseTrendingBatchExecute: %v", err)
	}
	if len(topics) == 0 {
		t.Fatalf("expected heuristic fallback to find at least one candidate term")
	}
}

func TestParseTrendingBatchExecuteTotalGarbageReturnsTypedError(t *testing.T) {
	_, err := parseTrendingBatchExecute([]byte(`)]}'` + "\n" + `123` + "\n" + `456` + "\n"))
	if !errors.Is(err, ErrTrendingParseFailed) {
		t.Fatalf("expected ErrTrendingParseFailed, got %v", err)
	}
}

func TestIsPlausibleTrendingTerm(t *testing.T) {
	cases := map[string]bool{
		"Taylor Swift":          true,
		"a":                     false, // too short
		"12345":                 false, // no letter
		"https://example.com/x": false, // URL
		"/trending/explore?q=x": false, // path
		"NBA Playoffs 2026":     true,
	}
	for input, want := range cases {
		if got := isPlausibleTrendingTerm(input); got != want {
			t.Errorf("isPlausibleTrendingTerm(%q) = %v, want %v", input, got, want)
		}
	}
}
