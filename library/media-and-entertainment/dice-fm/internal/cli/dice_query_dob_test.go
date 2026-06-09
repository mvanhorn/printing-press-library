// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Test for review finding #3: dob is collected + stored but never read. The
// fan GraphQL selection must stop requesting dob (data minimization) while
// keeping the fields that ARE consumed (email/phoneNumber/optInPartners).
package cli

import (
	"regexp"
	"testing"
)

func TestFanSelectionOmitsDOB(t *testing.T) {
	// Match dob only as a whole word so a field like "dobValue" wouldn't false-
	// positive (none exists, but be precise).
	if regexp.MustCompile(`\bdob\b`).MatchString(fanSelection) {
		t.Errorf("fanSelection still requests dob (data minimization): %q", fanSelection)
	}
	// The fields that ARE consumed must remain.
	for _, field := range []string{"email", "phoneNumber", "optInPartners", "firstName", "lastName", "id"} {
		if !regexp.MustCompile(`\b` + field + `\b`).MatchString(fanSelection) {
			t.Errorf("fanSelection unexpectedly dropped %q: %q", field, fanSelection)
		}
	}
}
