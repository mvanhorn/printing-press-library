package cli

import (
	"strings"
	"testing"
)

func TestBuildBookmarkFindQuery(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		authorID    string
		limit       int
		wantClauses []string // substrings the SQL must contain
		wantAbsent  []string // substrings the SQL must NOT contain
		wantArgs    []any
	}{
		{
			name:        "keyword only",
			query:       "Rust",
			limit:       50,
			wantClauses: []string{"WHERE", "lower(json_extract(data, '$.text')) LIKE ?", "ORDER BY", "LIMIT ?"},
			wantAbsent:  []string{"$.author_id"},
			wantArgs:    []any{"%rust%", 50},
		},
		{
			name:        "author only",
			authorID:    "44196397",
			limit:       20,
			wantClauses: []string{"WHERE", "json_extract(data, '$.author_id') = ?", "LIMIT ?"},
			wantAbsent:  []string{"$.text"},
			wantArgs:    []any{"44196397", 20},
		},
		{
			name:        "keyword and author",
			query:       "LLM",
			authorID:    "12",
			limit:       10,
			wantClauses: []string{"$.text", "AND", "$.author_id"},
			wantArgs:    []any{"%llm%", "12", 10},
		},
		{
			name:        "no filters returns unfiltered order+limit",
			limit:       5,
			wantClauses: []string{"SELECT data FROM bookmarks", "ORDER BY", "LIMIT ?"},
			wantAbsent:  []string{"WHERE"},
			wantArgs:    []any{5},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sqlStr, args := buildBookmarkFindQuery(tc.query, tc.authorID, tc.limit)
			for _, want := range tc.wantClauses {
				if !strings.Contains(sqlStr, want) {
					t.Errorf("SQL missing %q:\n%s", want, sqlStr)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(sqlStr, absent) {
					t.Errorf("SQL unexpectedly contains %q:\n%s", absent, sqlStr)
				}
			}
			if len(args) != len(tc.wantArgs) {
				t.Fatalf("arg count: got %d %v, want %d %v", len(args), args, len(tc.wantArgs), tc.wantArgs)
			}
			for i := range args {
				if args[i] != tc.wantArgs[i] {
					t.Errorf("arg[%d]: got %v, want %v", i, args[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"44196397", true},
		{"0", true},
		{"", false},
		{"karpathy", false},
		{"@elonmusk", false},
		{"12a", false},
		{"1 2", false},
	}
	for _, tc := range tests {
		if got := isAllDigits(tc.in); got != tc.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
