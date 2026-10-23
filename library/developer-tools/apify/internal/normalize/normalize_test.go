// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.

package normalize

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func decodeRows(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode rows: %v", err)
	}
	return rows
}

func itemTitles(items []*Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		if it == nil {
			continue
		}
		out[i] = it.Title
	}
	return out
}

// TestNormalizeBatch_ItemsExpansion covers the items-path expansion contract:
// no path leaves rows unchanged; a nested array path expands map elements; an
// empty array yields zero items; a missing or non-array path keeps the row;
// mixed array elements keep only maps.
func TestNormalizeBatch_ItemsExpansion(t *testing.T) {
	t.Parallel()

	articleProfile := &Profile{
		Actor: "test/expander",
		Items: "news_results",
		Title: []string{"title"},
		URL:   []string{"link"},
		Body:  []string{"snippet"},
	}
	nestedProfile := &Profile{
		Actor: "test/nested",
		Items: "page.articles",
		Title: []string{"title"},
		URL:   []string{"link"},
	}
	noItemsProfile := &Profile{
		Actor: "test/plain",
		Title: []string{"title"},
		URL:   []string{"link"},
	}

	cases := []struct {
		name       string
		profile    *Profile
		raw        string
		wantTitles []string
	}{
		{
			name:    "profile without items keeps each row",
			profile: noItemsProfile,
			raw: `[
				{"title":"One","link":"https://example.com/1","news_results":[{"title":"nested"}]},
				{"title":"Two","link":"https://example.com/2"}
			]`,
			wantTitles: []string{"One", "Two"},
		},
		{
			name:    "nested array path expands inner maps",
			profile: nestedProfile,
			raw: `[{
				"page": {
					"articles": [
						{"title":"Inner A","link":"https://example.com/a"},
						{"title":"Inner B","link":"https://example.com/b"}
					]
				}
			}]`,
			wantTitles: []string{"Inner A", "Inner B"},
		},
		{
			name:       "empty array yields no items",
			profile:    articleProfile,
			raw:        `[{"news_results":[]}]`,
			wantTitles: []string{},
		},
		{
			name:       "missing path keeps the row",
			profile:    articleProfile,
			raw:        `[{"title":"Page title","link":"https://example.com/page"}]`,
			wantTitles: []string{"Page title"},
		},
		{
			name:       "non-array string path keeps the row",
			profile:    articleProfile,
			raw:        `[{"news_results":"not-an-array","title":"Page title"}]`,
			wantTitles: []string{"Page title"},
		},
		{
			name:       "non-array object path keeps the row",
			profile:    articleProfile,
			raw:        `[{"news_results":{"title":"Object"},"title":"Page title"}]`,
			wantTitles: []string{"Page title"},
		},
		{
			name:    "mixed array elements keep only maps",
			profile: articleProfile,
			raw: `[{
				"news_results": [
					{"title":"Keep me","link":"https://example.com/keep"},
					"skip string",
					12,
					null,
					true,
					["nested","array"],
					{"title":"Keep two","link":"https://example.com/keep2"}
				]
			}]`,
			wantTitles: []string{"Keep me", "Keep two"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &Registry{
				profiles: map[string]*Profile{
					normalizeActorKey(tc.profile.Actor): tc.profile,
				},
				fallback: defaultFallback(),
			}
			got := itemTitles(r.NormalizeBatch(tc.profile.Actor, decodeRows(t, tc.raw)))
			if !reflect.DeepEqual(got, tc.wantTitles) {
				t.Fatalf("titles = %#v, want %#v", got, tc.wantTitles)
			}
		})
	}
}

// TestNormalizeBatch_EmbeddedGoogleNewsAPIProfile JSON-decodes a page-shaped
// dataset row and runs it through the embedded johnvc/GoogleNewsAPI profile so
// the YAML items path and expansion are checked together.
func TestNormalizeBatch_EmbeddedGoogleNewsAPIProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	p := r.Lookup("johnvc/GoogleNewsAPI")
	if p.Actor != "johnvc/GoogleNewsAPI" || p.Items != "news_results" {
		t.Fatalf("embedded profile actor=%q items=%q, want johnvc/GoogleNewsAPI / news_results", p.Actor, p.Items)
	}

	pageJSON := `{
		"search_query": "AI",
		"news_results": [
			{
				"title": "First article",
				"link": "https://example.com/first",
				"snippet": "alpha",
				"source": {"name": "Wire"},
				"published_at": "2026-10-22 15:04:05 UTC"
			},
			{
				"title": "Second article",
				"link": "https://example.com/second",
				"snippet": "beta",
				"source": "Reuters",
				"date": "2026-10-21"
			}
		]
	}`
	var row map[string]any
	if err := json.Unmarshal([]byte(pageJSON), &row); err != nil {
		t.Fatalf("decode page: %v", err)
	}

	got := r.NormalizeBatch("johnvc/GoogleNewsAPI", []map[string]any{row})
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2 (one per news_results element)", len(got))
	}
	if got[0].Title != "First article" || got[0].URL != "https://example.com/first" || got[0].Body != "alpha" || got[0].Author != "Wire" {
		t.Errorf("item 0 = title=%q url=%q body=%q author=%q", got[0].Title, got[0].URL, got[0].Body, got[0].Author)
	}
	if got[1].Title != "Second article" || got[1].URL != "https://example.com/second" || got[1].Body != "beta" || got[1].Author != "Reuters" {
		t.Errorf("item 1 = title=%q url=%q body=%q author=%q", got[1].Title, got[1].URL, got[1].Body, got[1].Author)
	}
	wantPub := time.Date(2026, 10, 22, 15, 4, 5, 0, time.UTC)
	if !got[0].PublishedAt.Equal(wantPub) {
		t.Errorf("item 0 published_at = %v, want %v", got[0].PublishedAt, wantPub)
	}
	wantDate := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	if !got[1].PublishedAt.Equal(wantDate) {
		t.Errorf("item 1 published_at = %v, want %v", got[1].PublishedAt, wantDate)
	}

	empty := decodeRows(t, `[{"news_results":[]}]`)
	if gotEmpty := r.NormalizeBatch("johnvc/GoogleNewsAPI", empty); len(gotEmpty) != 0 {
		t.Errorf("empty news_results yielded %d items, want 0", len(gotEmpty))
	}
}
