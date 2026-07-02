package cli

import (
	"strings"
	"testing"
)

func TestDefaultWootGraphQLQueryIsReadOnlySearchOffers(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"searchOffers",
		"Sort:BestSelling",
		"Limit:1",
		"TotalHits",
	} {
		if !strings.Contains(defaultWootGraphQLQuery, want) {
			t.Fatalf("defaultWootGraphQLQuery missing %q: %s", want, defaultWootGraphQLQuery)
		}
	}
}

func TestContainsGraphQLWriteOperation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{
			name:  "anonymous query",
			query: `{ searchOffers(Filter:{}, Sort:BestSelling, Limit:1, Skip:0){ TotalHits } }`,
			want:  false,
		},
		{
			name:  "named query",
			query: `query Deals { searchOffers(Filter:{}, Sort:BestSelling, Limit:1, Skip:0){ TotalHits } }`,
			want:  false,
		},
		{
			name:  "mutation",
			query: `mutation AddThing { addThing { id } }`,
			want:  true,
		},
		{
			name:  "subscription",
			query: `subscription WatchThing { thingChanged { id } }`,
			want:  true,
		},
		{
			name:  "ignores comments and strings",
			query: "# mutation ignored\nquery Deals { searchOffers(Filter:{Term:\"subscription mutation\"}, Sort:BestSelling, Limit:1, Skip:0){ TotalHits } }",
			want:  false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := containsGraphQLWriteOperation(tc.query); got != tc.want {
				t.Fatalf("containsGraphQLWriteOperation(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}
