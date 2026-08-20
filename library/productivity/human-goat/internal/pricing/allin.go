package pricing

import (
	"fmt"
	"math"
	"strings"
)

const (
	// ServiceFeeRate is a calibrated-from-observation default. TaskRabbit's
	// confirm endpoint is authoritative when available; this estimate is for
	// browse/ranking surfaces where confirm is not called per Tasker.
	ServiceFeeRate = 0.15

	// TrustSupportRate is a calibrated-from-observation default subject to metro
	// variance. It is set to match the observed $33.33 base -> $44.66 all-in
	// checkout uplift when combined with ServiceFeeRate.
	TrustSupportRate = 0.19
)

// Breakdown describes the estimated client-visible all-in hourly price.
type Breakdown struct {
	BaseCents       int    `json:"base_cents"`
	ServiceFeeCents int    `json:"service_fee_cents"`
	TrustFeeCents   int    `json:"trust_fee_cents"`
	AllInCents      int    `json:"all_in_cents"`
	State           string `json:"state,omitempty"`
	ServiceFeeOnly  bool   `json:"service_fee_only"`
}

// IsServiceFeeOnlyState reports whether TaskRabbit charges only the service
// fee because of the California/Massachusetts regulatory carve-out.
func IsServiceFeeOnlyState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "ca", "california", "ma", "massachusetts":
		return true
	default:
		return false
	}
}

// AllIn estimates TaskRabbit's all-in hourly client price from the browse
// poster_hourly_rate_cents base rate. The confirm endpoint returns the real
// all-in total and is authoritative; this estimate is intended only for
// ranking/browse flows where confirm is not called per Tasker.
func AllIn(baseCents int, state string) Breakdown {
	if baseCents < 0 {
		baseCents = 0
	}

	cleanState := strings.TrimSpace(state)
	serviceFee := roundCents(float64(baseCents) * ServiceFeeRate)
	serviceFeeOnly := IsServiceFeeOnlyState(cleanState)

	trustFee := 0
	if !serviceFeeOnly {
		trustFee = roundCents(float64(baseCents) * TrustSupportRate)
	}

	return Breakdown{
		BaseCents:       baseCents,
		ServiceFeeCents: serviceFee,
		TrustFeeCents:   trustFee,
		AllInCents:      baseCents + serviceFee + trustFee,
		State:           cleanState,
		ServiceFeeOnly:  serviceFeeOnly,
	}
}

// AllInDollars returns the estimated all-in hourly client price as dollars.
func AllInDollars(baseCents int, state string) float64 {
	return float64(AllIn(baseCents, state).AllInCents) / 100.0
}

// FormatCents formats a cent amount as US dollars.
func FormatCents(cents int) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

func roundCents(value float64) int {
	return int(math.Round(value))
}
