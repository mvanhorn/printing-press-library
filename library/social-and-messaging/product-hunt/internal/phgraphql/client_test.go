package phgraphql

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New("test-token", 10*time.Second, 0)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.token != "test-token" {
		t.Errorf("token = %q, want %q", c.token, "test-token")
	}
	if c.dryRun {
		t.Error("dryRun should be false for New")
	}
}

func TestNewDryRun(t *testing.T) {
	c := NewDryRun("tok")
	if c == nil {
		t.Fatal("NewDryRun returned nil")
	}
	if !c.dryRun {
		t.Error("dryRun should be true for NewDryRun")
	}
}

func TestIsNumericID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"abc", false},
		{"12abc", false},
		{"", false},
		{"developer-tools", false},
	}
	for _, tt := range tests {
		got := isNumericID(tt.input)
		if got != tt.want {
			t.Errorf("isNumericID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
