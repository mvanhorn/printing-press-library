// uk-train-goat hand-authored: tests for fare provenance note builder.
package cli

import (
	"strings"
	"testing"
)

func TestFareProvenanceNote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		publishDate string
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "with date",
			publishDate: "Mon, 01 Jan 2026 00:00:00 GMT",
			wantContain: "(published Mon, 01 Jan 2026 00:00:00 GMT)",
		},
		{
			name:        "blank date omits parenthetical",
			publishDate: "",
			wantAbsent:  "(published ",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fareProvenanceNote(tc.publishDate)
			if tc.wantContain != "" && !strings.Contains(got, tc.wantContain) {
				t.Errorf("fareProvenanceNote(%q) = %q; want it to contain %q", tc.publishDate, got, tc.wantContain)
			}
			if tc.wantAbsent != "" && strings.Contains(got, tc.wantAbsent) {
				t.Errorf("fareProvenanceNote(%q) = %q; want it NOT to contain %q", tc.publishDate, got, tc.wantAbsent)
			}
			// Both branches must include the disclaimer suffix.
			const suffix = "confirm the price at point of sale"
			if !strings.Contains(got, suffix) {
				t.Errorf("fareProvenanceNote(%q) = %q; missing disclaimer suffix %q", tc.publishDate, got, suffix)
			}
		})
	}
}
