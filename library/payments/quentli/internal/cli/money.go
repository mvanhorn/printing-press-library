// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Money formatting helper for minor-currency API amounts.
package cli

import "fmt"

// defaultCurrency is the assumed currency when the API omits one. Quentli is a
// Mexican platform (SAT CFDI billing), so unspecified amounts are pesos.
const defaultCurrency = "MXN"

// normalizeCurrency resolves an unspecified currency to defaultCurrency.
// Aggregations must group on the normalized value, not the raw one: grouping on
// the raw value and only resolving at print time yields two buckets that render
// under the same label, which reads as duplicate or contradictory totals.
func normalizeCurrency(currency string) string {
	if currency == "" {
		return defaultCurrency
	}
	return currency
}

// formatMoneyMinor renders an amount in minor currency units (e.g. 150000)
// as a localized string with two decimals (e.g. "MXN 1,500.00").
func formatMoneyMinor(amount float64, currency string) string {
	return fmt.Sprintf("%s %.2f", normalizeCurrency(currency), amount/100.0)
}
