package cli

import "testing"

func TestParseAccountBudgetDataHandlesWrappedCreditCount(t *testing.T) {
	balance := []byte(`{"results":{"success":true,"creditCount":42551,"message":"You have 42551 credits remaining."}}`)
	daily := []byte(`{"results":{"usage":[{"count":100},{"count":50}]}}`)

	creditsRemaining, dailyBurn := parseAccountBudgetData(balance, daily)
	if creditsRemaining != 42551 {
		t.Fatalf("creditsRemaining = %v, want 42551", creditsRemaining)
	}
	if dailyBurn != 75 {
		t.Fatalf("dailyBurn = %v, want 75", dailyBurn)
	}
}

func TestParseAccountBudgetDataHandlesLegacyShape(t *testing.T) {
	balance := []byte(`{"credits_remaining":1234}`)
	daily := []byte(`{"usage":[{"count":10},{"count":20}]}`)

	creditsRemaining, dailyBurn := parseAccountBudgetData(balance, daily)
	if creditsRemaining != 1234 {
		t.Fatalf("creditsRemaining = %v, want 1234", creditsRemaining)
	}
	if dailyBurn != 15 {
		t.Fatalf("dailyBurn = %v, want 15", dailyBurn)
	}
}
