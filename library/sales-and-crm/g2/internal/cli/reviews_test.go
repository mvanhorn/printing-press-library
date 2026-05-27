// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestExtractSyndicationFlag(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		want  bool
	}{
		{
			name:  "canonical bool true",
			attrs: map[string]any{"syndication_eligible": true},
			want:  true,
		},
		{
			name:  "canonical bool false",
			attrs: map[string]any{"syndication_eligible": false},
			want:  false,
		},
		{
			name:  "alt key eligible_for_syndication",
			attrs: map[string]any{"eligible_for_syndication": true},
			want:  true,
		},
		{
			name:  "alt key syndicateable",
			attrs: map[string]any{"syndicateable": true},
			want:  true,
		},
		{
			name:  "string yes",
			attrs: map[string]any{"syndication_eligible": "yes"},
			want:  true,
		},
		{
			name:  "string false",
			attrs: map[string]any{"syndication_eligible": "false"},
			want:  false,
		},
		{
			name:  "numeric 1",
			attrs: map[string]any{"syndication_eligible": float64(1)},
			want:  true,
		},
		{
			name:  "numeric 0",
			attrs: map[string]any{"syndication_eligible": float64(0)},
			want:  false,
		},
		{
			name:  "absent",
			attrs: map[string]any{"title": "a"},
			want:  false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := extractSyndicationFlag(tc.attrs); got != tc.want {
				t.Fatalf("extractSyndicationFlag(%v) = %v, want %v", tc.attrs, got, tc.want)
			}
		})
	}
}

func TestNewReviewsCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newReviewsCmd(flags)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reviews --help failed: %v", err)
	}
}

func TestNewReviewsListCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newReviewsListCmd(flags)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reviews list --help failed: %v", err)
	}
}
