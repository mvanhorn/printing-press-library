package azure

import "strings"

// PlanTier is the human-facing hosting model a Function App runs under.
type PlanTier string

const (
	TierConsumption PlanTier = "Consumption" // Y1 / Dynamic — scales to zero, cold starts apply
	TierPremium     PlanTier = "Premium"     // EP1/EP2/EP3 — Elastic Premium, pre-warmed, no cold start
	TierDedicated   PlanTier = "Dedicated"   // App Service plan (B*/S*/P*V*) — always on, no dynamic scale
	TierUnknown     PlanTier = "Unknown"
)

// ClassifyPlanTier maps an App Service plan SKU name (Microsoft.Web/serverfarms
// sku.name) to its hosting model. The distinction drives plan-fit and coldstart:
// cold starts only occur on Consumption.
//
//   - "Y1" (Dynamic) => Consumption
//   - SKU starting with "EP" (EP1/EP2/EP3, ElasticPremium) => Premium
//   - anything else with a recognized dedicated prefix (B/S/P/I/WS) => Dedicated
//   - empty/unrecognized => Unknown
//
// Matching is case-insensitive. "P*" is Dedicated, not Premium: App Service
// plan SKUs starting with "P" (e.g. P1V2) are dedicated and do not scale
// elastically — a documented Azure footgun.
func ClassifyPlanTier(sku string) PlanTier {
	s := strings.ToUpper(strings.TrimSpace(sku))
	switch {
	case s == "":
		return TierUnknown
	case s == "Y1" || strings.HasPrefix(s, "Y"):
		return TierConsumption
	case strings.HasPrefix(s, "EP"):
		return TierPremium
	case strings.HasPrefix(s, "P"),
		strings.HasPrefix(s, "S"),
		strings.HasPrefix(s, "B"),
		strings.HasPrefix(s, "I"),
		strings.HasPrefix(s, "WS"):
		return TierDedicated
	default:
		return TierUnknown
	}
}

// HasColdStarts reports whether a tier is subject to cold starts. Only
// Consumption scales to zero; Premium keeps pre-warmed instances and Dedicated
// is always on.
func (t PlanTier) HasColdStarts() bool {
	return t == TierConsumption
}
