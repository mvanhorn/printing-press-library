package cli

import (
	"testing"
	"time"
)

func TestStartOfWeek(t *testing.T) {
	tests := []struct {
		name string
		date string // YYYY-MM-DD
		want string // expected start of week
	}{
		{"Monday returns same day", "2026-05-11", "2026-05-11"},
		{"Tuesday returns prior Monday", "2026-05-12", "2026-05-11"},
		{"Wednesday returns Monday", "2026-05-13", "2026-05-11"},
		{"Sunday returns prior Monday", "2026-05-10", "2026-05-04"},
		{"Friday returns Monday", "2026-05-15", "2026-05-11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := time.Parse("2006-01-02", tt.date)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := startOfWeek(d).Format("2006-01-02")
			if got != tt.want {
				t.Errorf("startOfWeek(%s) = %s, want %s", tt.date, got, tt.want)
			}
		})
	}
}

func TestRenderWindowEmpty(t *testing.T) {
	w := standupWindow{Installs: 0, Cost: 0, Revenue: 0}
	got := renderWindow(w)
	if got != "—" {
		t.Errorf("empty window should render as '—', got %q", got)
	}
}

func TestRenderWindowFormat(t *testing.T) {
	w := standupWindow{Installs: 150, Cost: 500, Revenue: 750, ROAS: 1.5}
	got := renderWindow(w)
	// We don't assert exact format (table renderer is allowed to evolve), but
	// the major numeric values must appear.
	for _, must := range []string{"150", "500", "750", "150.0%"} {
		if !contains(got, must) {
			t.Errorf("renderWindow output missing %q: %s", must, got)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
