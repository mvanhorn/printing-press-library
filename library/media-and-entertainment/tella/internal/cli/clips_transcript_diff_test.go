// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"strings"
	"testing"
)

// TestDiffRemovedWords pins the round-9 fix: the previous multiset bag-count
// produced correct removal counts but wrong position fields for repeated
// words. The two-pointer walk preserves sequence information so each
// `removed_words[].position` references the actual uncut index that was
// deleted, which downstream tooling ("show the words removed near
// timestamp T") relies on.
func TestDiffRemovedWords(t *testing.T) {
	type pos struct {
		Word     string
		Position int
	}
	cases := []struct {
		name  string
		uncut string
		cut   string
		want  []pos
	}{
		{
			name:  "single repeated word removed at head",
			uncut: "alpha beta gamma alpha",
			cut:   "beta gamma alpha",
			// The previous bag-count algorithm would have reported
			// removal at position 3 (the second alpha). The two-pointer
			// walk correctly reports position 0 — the actual first
			// position where the subsequence diverges.
			want: []pos{{"alpha", 0}},
		},
		{
			name:  "repeated word removed in middle",
			uncut: "the cat the cat the dog",
			cut:   "the cat the dog",
			want:  []pos{{"cat", 3}, {"the", 4}},
		},
		{
			name:  "filler words removed",
			uncut: "uh the cat uh sat",
			cut:   "the cat sat",
			want:  []pos{{"uh", 0}, {"uh", 3}},
		},
		{
			name:  "case-insensitive match",
			uncut: "The Cat",
			cut:   "the cat",
			want:  []pos{},
		},
		{
			name:  "identical inputs produce no removals",
			uncut: "alpha beta gamma",
			cut:   "alpha beta gamma",
			want:  []pos{},
		},
		{
			name:  "empty cut means everything removed",
			uncut: "alpha beta gamma",
			cut:   "",
			want:  []pos{{"alpha", 0}, {"beta", 1}, {"gamma", 2}},
		},
		{
			name:  "empty uncut produces no removals",
			uncut: "",
			cut:   "alpha",
			want:  []pos{},
		},
		{
			name:  "tail removed",
			uncut: "alpha beta gamma delta",
			cut:   "alpha beta",
			want:  []pos{{"gamma", 2}, {"delta", 3}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uncutWords := strings.Fields(tc.uncut)
			cutWords := strings.Fields(tc.cut)
			got := diffRemovedWords(uncutWords, cutWords)
			gotPos := make([]pos, 0, len(got))
			for _, r := range got {
				gotPos = append(gotPos, pos{Word: r.Word, Position: r.Position})
			}
			if len(gotPos) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(gotPos, tc.want) {
				t.Fatalf("diffRemovedWords(%q, %q) = %v, want %v", tc.uncut, tc.cut, gotPos, tc.want)
			}
		})
	}
}
