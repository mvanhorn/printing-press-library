// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestFilterTraceMonthlyRecords(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	records := []map[string]any{
		{"beginningOfMonth": "2025-12-01", "subProductCode": "CORP"}, // before window
		{"beginningOfMonth": "2026-01-01", "subProductCode": "CORP"}, // in window
		{"beginningOfMonth": "2026-02-01", "subProductCode": "AGCY"}, // in window, different product
		{"beginningOfMonth": "2026-06-01", "subProductCode": "CORP"}, // after window
		{"subProductCode": "CORP"},                                   // no date field
	}

	t.Run("no sub-product filter returns everything in the window", func(t *testing.T) {
		t.Parallel()
		got := filterTraceMonthlyRecords(records, "", start, end)
		if len(got) != 2 {
			t.Fatalf("filterTraceMonthlyRecords matched %d records, want 2", len(got))
		}
	})

	t.Run("sub-product filter narrows by exact case-insensitive match", func(t *testing.T) {
		t.Parallel()
		got := filterTraceMonthlyRecords(records, "corp", start, end)
		if len(got) != 1 {
			t.Fatalf("filterTraceMonthlyRecords matched %d records, want 1", len(got))
		}
		if got[0]["beginningOfMonth"] != "2026-01-01" {
			t.Fatalf("filterTraceMonthlyRecords kept unexpected record: %v", got[0])
		}
	})

	t.Run("sub-product filter with no matches returns empty", func(t *testing.T) {
		t.Parallel()
		got := filterTraceMonthlyRecords(records, "MUNI", start, end)
		if len(got) != 0 {
			t.Fatalf("filterTraceMonthlyRecords matched %d records, want 0", len(got))
		}
	})
}
