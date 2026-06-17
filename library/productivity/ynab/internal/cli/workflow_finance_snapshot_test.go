package cli

import "testing"

func TestYNABAssetKeywordDoesNotMatchVisa(t *testing.T) {
	if hasYNABAssetKeyword("Visa Debit") {
		t.Fatal("Visa should not match the ISA asset keyword")
	}
	if !isYNABOperatingAccount("Visa Debit", "checking") {
		t.Fatal("Visa checking account should remain operating cash")
	}
	if !hasYNABAssetKeyword("Trading 212 ISA") {
		t.Fatal("Trading 212 ISA should match asset keywords")
	}
	if isYNABOperatingAccount("Trading 212 ISA", "checking") {
		t.Fatal("ISA account should not be operating cash")
	}
}

func TestAverageMonthSpendExcludesPartialMonthsForFullAverages(t *testing.T) {
	months := []ynabSnapshotMonth{
		{Month: "2026-04", Spend: 1000},
		{Month: "2026-05", Spend: 2000},
		{Month: "2026-06", Spend: 999, Partial: true},
	}
	full := make([]ynabSnapshotMonth, 0, len(months))
	for _, m := range months {
		if !m.Partial && m.SourceError == "" {
			full = append(full, m)
		}
	}
	got := averageMonthSpend(full)
	if got != 1500 {
		t.Fatalf("full-month average = %v, want 1500", got)
	}
}
