package cli

import "testing"

func TestIsJobTerminalAmazonReportStatuses(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		want bool
	}{
		{name: "success", body: map[string]any{"status": "SUCCESS"}, want: true},
		{name: "failure", body: map[string]any{"status": "FAILURE"}, want: true},
		{name: "completed", body: map[string]any{"status": "COMPLETED"}, want: true},
		{name: "in progress", body: map[string]any{"status": "IN_PROGRESS"}, want: false},
		{name: "pending", body: map[string]any{"status": "PENDING"}, want: false},
		{name: "missing", body: map[string]any{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJobTerminal(tt.body); got != tt.want {
				t.Fatalf("isJobTerminal(%v) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}
