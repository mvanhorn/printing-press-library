// Hand-authored tests for the DOI<->PMID converter helpers. Not generated.
package cli

import "testing"

func TestNormPMID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare", "12345", "12345"},
		{"surrounding space", "  12345  ", "12345"},
		{"lower prefix", "pmid:12345", "12345"},
		{"upper prefix", "PMID:12345", "12345"},
		{"prefix with space", "PMID: 12345", "12345"},
		{"empty", "", ""},
		{"only spaces", "   ", ""},
		{"non-numeric passthrough", "abc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normPMID(tt.in); got != tt.want {
				t.Errorf("normPMID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
