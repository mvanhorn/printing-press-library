// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import "testing"

func TestDiffDuplicateMetricsUseDeterministicContentPairing(t *testing.T) {
	left := []Row{
		duplicateDiffRow("left-random-z", "2020-02-01", 20),
		duplicateDiffRow("left-random-z", "2020-01-01", 10),
		duplicateDiffRow("left-random-a", "2020-01-01", 30),
		duplicateDiffRow("left-random-a", "2020-02-01", 40),
	}
	right := []Row{
		duplicateDiffRow("right-random-a", "2020-02-01", 20),
		duplicateDiffRow("right-random-a", "2020-01-01", 10),
		duplicateDiffRow("right-random-z", "2020-02-01", 40),
		duplicateDiffRow("right-random-z", "2020-01-01", 30),
	}

	t.Run("reversed metric ids are unchanged", func(t *testing.T) {
		var summary DiffSummary
		changes := diffRows(left, right, Snapshot{}, Snapshot{}, &summary)
		if len(changes) != 0 {
			t.Fatalf("changes = %#v", changes)
		}
		if summary.ChangedRows != 0 || summary.AddedRows != 0 || summary.RemovedRows != 0 || summary.UnchangedRows != 4 {
			t.Fatalf("summary = %#v", summary)
		}
	})

	t.Run("one duplicate series change remains visible", func(t *testing.T) {
		rightChanged := []Row{
			duplicateDiffRow("right-random-a", "2020-02-01", 20),
			duplicateDiffRow("right-random-a", "2020-01-01", 10),
			duplicateDiffRow("right-random-z", "2020-02-01", 41),
			duplicateDiffRow("right-random-z", "2020-01-01", 30),
		}
		var summary DiffSummary
		changes := diffRows(left, rightChanged, Snapshot{}, Snapshot{}, &summary)
		if len(changes) != 1 || changes[0].Kind != "changed" || changes[0].Key.ObservationOrdinal != 1 || !hasDiffField(changes[0].ChangedFields, "monthly_searches") {
			t.Fatalf("changes = %#v", changes)
		}
		if summary.ChangedRows != 1 || summary.AddedRows != 0 || summary.RemovedRows != 0 || summary.UnchangedRows != 3 {
			t.Fatalf("summary = %#v", summary)
		}
	})

	t.Run("duplicate count delta remains visible", func(t *testing.T) {
		rightWithExtra := append([]Row(nil), right...)
		rightWithExtra = append(rightWithExtra,
			duplicateDiffRow("right-random-extra", "2020-01-01", 10),
			duplicateDiffRow("right-random-extra", "2020-02-01", 20),
		)
		var summary DiffSummary
		changes := diffRows(left, rightWithExtra, Snapshot{}, Snapshot{}, &summary)
		if len(changes) != 2 {
			t.Fatalf("changes = %#v", changes)
		}
		for _, change := range changes {
			if change.Kind != "added" {
				t.Fatalf("change = %#v", change)
			}
		}
		if summary.ChangedRows != 0 || summary.AddedRows != 2 || summary.RemovedRows != 0 || summary.UnchangedRows != 4 {
			t.Fatalf("summary = %#v", summary)
		}
	})
}

func duplicateDiffRow(metricID, month string, monthlySearches int64) Row {
	return Row{
		MetricID:         metricID,
		Keyword:          "duplicate term",
		SubmittedKeyword: "duplicate term",
		VariantGroup:     "base",
		Month:            month,
		MonthlySearches:  int64Pointer(monthlySearches),
		ValueState:       "valid",
		Status:           "ok",
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

func hasDiffField(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}
