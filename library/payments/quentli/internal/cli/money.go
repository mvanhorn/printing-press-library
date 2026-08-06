// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Money formatting helper for minor-currency API amounts.
package cli

import "fmt"

// formatMoneyMinor renders an amount in minor currency units (e.g. 150000)
// as a localized string with two decimals (e.g. "MXN 1,500.00").
func formatMoneyMinor(amount float64, currency string) string {
	if currency == "" {
		currency = "MXN"
	}
	return fmt.Sprintf("%s %.2f", currency, amount/100.0)
}
