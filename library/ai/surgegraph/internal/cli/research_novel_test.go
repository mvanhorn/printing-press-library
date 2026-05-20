package cli

import (
	"encoding/json"
	"sort"
	"testing"
)

// TestCollectDocTitles pins PATCH(greptile-7)'s envelope-tolerant doc-title
// extraction. research drift's gap classification depends on this helper
// returning titles for both /v1/get_writer_documents and
// /v1/get_optimized_documents responses, regardless of which envelope shape
// the API returns.
func TestCollectDocTitles(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "bare-array", raw: `[{"title":"A"},{"title":"B"}]`, want: []string{"a", "b"}},
		{name: "data-envelope", raw: `{"data":[{"title":"X"}]}`, want: []string{"x"}},
		{name: "documents-envelope", raw: `{"documents":[{"title":"Y"}]}`, want: []string{"y"}},
		{name: "skips-empty-titles", raw: `[{"title":""},{"title":"Z"}]`, want: []string{"z"}},
		{name: "lowercases-titles", raw: `[{"title":"MiXeDcAsE Title"}]`, want: []string{"mixedcase title"}},
		{name: "ignores-other-keys", raw: `[{"id":"123","title":"Real","body":"unused"}]`, want: []string{"real"}},
		{name: "invalid-json-returns-empty", raw: `{not json`, want: nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := collectDocTitles(json.RawMessage(tc.raw))
			sort.Strings(got)
			sort.Strings(tc.want)
			if len(got) != len(tc.want) {
				t.Fatalf("len: want %d %v, got %d %v", len(tc.want), tc.want, len(got), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d]: want %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}
