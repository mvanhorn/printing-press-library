package store

import "testing"

func TestIsSelectOnly(t *testing.T) {
	cases := []struct {
		name string
		q    string
		ok   bool
	}{
		{"plain select", "SELECT 1", true},
		{"stacked delete", "SELECT 1;DELETE FROM urls", false},
		{"newline update", "select 1\nupdate urls set title='x'", false},
		{"comment obscured insert", "SELECT 1 /*x*/ INSERT INTO t VALUES (1)", false},
		{"cte select", "WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"keyword in like literal", "SELECT url FROM history_items WHERE url LIKE '%create%'", true},
		{"delete in like literal", "SELECT title FROM history_visits WHERE title LIKE '%delete%'", true},
		{"replace function", "SELECT REPLACE(url, 'http', 'https') FROM history_items", true},
	}
	for _, tc := range cases {
		if got := IsSelectOnly(tc.q); got != tc.ok {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.ok)
		}
	}
}
