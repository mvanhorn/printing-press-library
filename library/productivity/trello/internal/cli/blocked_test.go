// Hand-authored tests for the blocked-scan novel command's matching logic.

package cli

import (
	"regexp"
	"testing"
)

func TestMatchAnyLabel(t *testing.T) {
	re := regexp.MustCompile("(?i)blocked|waiting on")
	cases := []struct {
		name   string
		labels []string
		want   bool
	}{
		{"blocked label", []string{"Blocked"}, true},
		{"waiting phrase", []string{"waiting on design"}, true},
		{"case insensitive", []string{"BLOCKED"}, true},
		{"no match", []string{"ready", "p1"}, false},
		{"no labels", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c trelloCard
			for _, l := range tc.labels {
				c.Labels = append(c.Labels, struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{Name: l})
			}
			if got := matchAnyLabel(c, re); got != tc.want {
				t.Fatalf("matchAnyLabel(%v) = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}
