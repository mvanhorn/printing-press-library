// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestBEAIndustryRequestContextMarksInputsAsNotApplied(t *testing.T) {
	context := beaIndustryRequestContext("541511", "software", "CA")

	if context["naics"] != "541511" || context["industry"] != "software" || context["state"] != "CA" {
		t.Fatalf("request context did not preserve caller inputs: %#v", context)
	}
	if context["applied_to_bea_query"] != false {
		t.Fatalf("applied_to_bea_query = %v, want false", context["applied_to_bea_query"])
	}
}
