package nsdl

import "testing"

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   float64
		wantOk bool
	}{
		{"plain integer", "13", 13, true},
		{"indian comma grouping", "5,11,510", 511510, true},
		{"negative", "-147", -147, true},
		{"decimal", "94.5975", 94.5975, true},
		{"dash placeholder", "-", 0, false},
		{"double dash", "--", 0, false},
		{"empty", "", 0, false},
		{"whitespace only", "   ", 0, false},
		{"NA", "NA", 0, false},
		{"parenthesized negative", "(147)", -147, true},
		{"non-numeric text", "Sr. No.", 0, false},
		{"large indian comma value", "3,52,86,502", 35286502, true},
		{"zero", "0", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseNumber(tt.input)
			if ok != tt.wantOk {
				t.Fatalf("ParseNumber(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && (got-tt.want > 1e-9 || tt.want-got > 1e-9) {
				t.Fatalf("ParseNumber(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
