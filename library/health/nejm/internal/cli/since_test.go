// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

// TestNejmSinceCutoffString pins the timezone contract: the SQL cutoff must be
// the true UTC instant in RFC3339, not local wall-clock time with a pasted "Z".
func TestNejmSinceCutoffString(t *testing.T) {
	plusTwo := time.FixedZone("UTC+2", 2*60*60)
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			name: "non-UTC local time converts to UTC",
			in:   time.Date(2026, 7, 5, 12, 0, 0, 0, plusTwo),
			want: "2026-07-05T10:00:00Z",
		},
		{
			name: "UTC time passes through",
			in:   time.Date(2026, 7, 5, 10, 30, 0, 0, time.UTC),
			want: "2026-07-05T10:30:00Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nejmSinceCutoffString(tt.in); got != tt.want {
				t.Errorf("nejmSinceCutoffString(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
