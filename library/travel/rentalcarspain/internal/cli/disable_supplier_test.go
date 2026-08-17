// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// --disable-supplier lets a user skip any supplier, matched case-insensitively.
func TestSupplierEnabled(t *testing.T) {
	flags := &rootFlags{disabledSuppliers: "GoldCar, doyouspain"}
	if supplierEnabled(flags, "goldcar") {
		t.Error("goldcar should be disabled (case-insensitive)")
	}
	if supplierEnabled(flags, "doyouspain") {
		t.Error("doyouspain should be disabled")
	}
	if !supplierEnabled(flags, "centauro") {
		t.Error("centauro should stay enabled")
	}
	// Empty flag → everything enabled.
	if !supplierEnabled(&rootFlags{}, "goldcar") {
		t.Error("no --disable-supplier should leave all enabled")
	}
}

// enabledDirectCompanies drops exactly the disabled direct suppliers and keeps
// the rest.
func TestEnabledDirectCompanies(t *testing.T) {
	all := directCompanies()
	// Nothing disabled → full set.
	if got := len(enabledDirectCompanies(&rootFlags{})); got != len(all) {
		t.Errorf("no disable = %d companies, want %d", got, len(all))
	}
	// Disable two → set shrinks by two and excludes them.
	flags := &rootFlags{disabledSuppliers: "goldcar,delpaso"}
	got := enabledDirectCompanies(flags)
	if len(got) != len(all)-2 {
		t.Fatalf("disabling two = %d companies, want %d", len(got), len(all)-2)
	}
	for _, co := range got {
		if co.name == "Goldcar" || co.name == "Delpaso" {
			t.Errorf("disabled company %q must not appear", co.name)
		}
	}
}
