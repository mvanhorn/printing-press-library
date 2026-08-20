package mr

import "testing"

func TestFilterMatchesTitleBodyAuthorAndCategory(t *testing.T) {
	items := []Item{
		{
			Title:       "Assorted links",
			Author:      "Tyler Cowen",
			Categories:  []string{"Economics", "Web/Tech"},
			ContentText: "An item about productivity and institutions.",
		},
		{
			Title:       "Book notes",
			Author:      "Alex Tabarrok",
			Categories:  []string{"Books"},
			ContentText: "A paragraph about history.",
		},
	}

	got := Filter(items, "institutions", "tyler", "economics", 10)
	if len(got) != 1 || got[0].Title != "Assorted links" {
		t.Fatalf("unexpected filter result: %#v", got)
	}
}

func TestSortedCountsOrdersByCountThenName(t *testing.T) {
	got := SortedCounts(map[string]int{"Books": 1, "Economics": 2, "Data Source": 2})
	if got[0].Name != "Data Source" || got[1].Name != "Economics" || got[2].Name != "Books" {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestFindMatchesURLGuidTitleAndSlug(t *testing.T) {
	item := Item{
		Title: "Self-fulfilling misalignment?",
		Link:  "https://marginalrevolution.com/marginalrevolution/2026/05/self-fulfilling-misalignment.html",
		GUID:  "https://marginalrevolution.com/?p=92970",
	}
	items := []Item{item}

	for _, needle := range []string{item.Link, item.GUID, item.Title, "self-fulfilling-misalignment"} {
		got, ok := Find(items, needle)
		if !ok || got.Title != item.Title {
			t.Fatalf("Find(%q) = %#v, %v", needle, got, ok)
		}
	}
}

func TestStripTrackingDecodesEntitiesBeforeRemovingUTM(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "plain utm query",
			raw:  "https://example.com/post?utm_source=rss&utm_medium=feed",
			want: "https://example.com/post",
		},
		{
			name: "encoded ampersand before utm parameter",
			raw:  "https://example.com/post?foo=bar&#038;utm_source=rss",
			want: "https://example.com/post?foo=bar",
		},
		{
			name: "encoded ampersand without tracking",
			raw:  "https://example.com/post?foo=bar&#038;baz=qux",
			want: "https://example.com/post?foo=bar&baz=qux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripTracking(tt.raw); got != tt.want {
				t.Fatalf("stripTracking(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
