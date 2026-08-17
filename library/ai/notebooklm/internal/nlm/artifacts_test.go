package nlm

import "testing"

func TestArtifactReady(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"", true},
		{"ready", true},
		{"Ready", true},
		{"complete", true},
		{"completed", true},
		{"pending", false},
		{"in_progress", false},
		{"failed", false},
	}
	for _, tc := range tests {
		if got := artifactReady(tc.status); got != tc.want {
			t.Errorf("artifactReady(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestArtifactFailed(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"", false},
		{"ready", false},
		{"pending", false},
		{"failed", true},
		{"error", true},
		{"generation_error", true},
	}
	for _, tc := range tests {
		if got := artifactFailed(tc.status); got != tc.want {
			t.Errorf("artifactFailed(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
