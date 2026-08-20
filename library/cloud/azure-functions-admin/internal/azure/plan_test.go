package azure

import "testing"

func TestClassifyPlanTier(t *testing.T) {
	cases := []struct {
		sku  string
		want PlanTier
	}{
		{"Y1", TierConsumption},
		{"y1", TierConsumption},
		{"EP1", TierPremium},
		{"EP2", TierPremium},
		{"ep3", TierPremium},
		{"P1V2", TierDedicated}, // "P" is Dedicated, not Premium — the Azure footgun
		{"P1V3", TierDedicated},
		{"S1", TierDedicated},
		{"B1", TierDedicated},
		{"I1", TierDedicated},
		{"WS1", TierDedicated}, // Workflow Standard (Logic Apps) dedicated
		{"", TierUnknown},
		{"FANCY", TierUnknown},
	}
	for _, c := range cases {
		if got := ClassifyPlanTier(c.sku); got != c.want {
			t.Errorf("ClassifyPlanTier(%q) = %q, want %q", c.sku, got, c.want)
		}
	}
}

func TestHasColdStarts(t *testing.T) {
	if !TierConsumption.HasColdStarts() {
		t.Error("Consumption should have cold starts")
	}
	for _, tier := range []PlanTier{TierPremium, TierDedicated, TierUnknown} {
		if tier.HasColdStarts() {
			t.Errorf("%q should not report cold starts", tier)
		}
	}
}
