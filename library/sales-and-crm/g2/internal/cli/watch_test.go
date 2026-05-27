// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestParseProductList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "slack", []string{"slack"}},
		{"multi", "slack,zoom,asana", []string{"slack", "zoom", "asana"}},
		{"whitespace", " slack , zoom ,asana ", []string{"slack", "zoom", "asana"}},
		{"empty entries", "slack,,zoom", []string{"slack", "zoom"}},
		{"empty input", "", nil},
		{"all whitespace", ", , ,", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := parseProductList(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseProductList(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatchCategory(t *testing.T) {
	cases := []struct {
		name   string
		row    string
		target string
		want   bool
	}{
		{"exact", "cat-1", "cat-1", true},
		{"different", "cat-1", "cat-2", false},
		{"empty row passes", "", "cat-1", true},
		{"empty target rejects", "cat-1", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := matchCategory(tc.row, tc.target); got != tc.want {
				t.Fatalf("matchCategory(%q,%q) = %v, want %v", tc.row, tc.target, got, tc.want)
			}
		})
	}
}

func TestNewWatchCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newWatchCmd(flags)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("watch --help failed: %v", err)
	}
}
