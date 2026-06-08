package cli

import "testing"

func TestCompareProductsSummary_PackAware(t *testing.T) {
	t.Parallel()

	left := productResponse{
		Price:          15.84,
		UnitPrice:      0.88,
		UnitLabel:      "lt",
		PackLabel:      "emb. 18 x 1 lt",
		SavingsPercent: 12,
	}
	right := productResponse{
		Price:          0.90,
		UnitPrice:      0.90,
		UnitLabel:      "lt",
		PackLabel:      "1 x 1 lt",
		SavingsPercent: 10,
	}

	got := compareProductsSummary(left, right)
	if len(got) < 2 {
		t.Fatalf("summary = %v, want pack-aware output", got)
	}
	if got[0] != "right is 0.02 EUR/lt more expensive" {
		t.Fatalf("unexpected first summary line: %v", got)
	}
	if got[1] != "different pack sizes" {
		t.Fatalf("expected pack-size warning, got %v", got)
	}
}
