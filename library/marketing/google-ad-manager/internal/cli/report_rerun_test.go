// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// report rerun reuses the shared run->poll->fetch helpers exercised in
// report_run_test.go and report_watch_test.go. Its own rerun-specific logic is
// composing the saved report's resource name from a network code and report id;
// this guards that composition (including an already-prefixed network code).
func TestRerunReportNameComposition(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		reportID string
		want     string
	}{
		{"bare code", "123456", "9876", "networks/123456/reports/9876"},
		{"already-prefixed code", "networks/123456", "9876", "networks/123456/reports/9876"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := networkParent(tt.code) + "/reports/" + tt.reportID
			if got != tt.want {
				t.Fatalf("rerun report name = %q, want %q", got, tt.want)
			}
		})
	}
}
