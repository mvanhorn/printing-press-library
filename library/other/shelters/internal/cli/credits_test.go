// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// TestGratitudeAndDisclaimer pins the user-requested gratitude and the safety
// disclaimer wording.
func TestGratitudeAndDisclaimer(t *testing.T) {
	for _, want := range []string{"first responders", "emergency management practitioners", "relief nonprofit organizations"} {
		if !strings.Contains(sheltersGratitude, want) {
			t.Errorf("gratitude missing %q", want)
		}
	}
	for _, want := range []string{"unofficial tool", "FEMA", "call 911", "lag reality"} {
		if !strings.Contains(sheltersDisclaimer, want) {
			t.Errorf("disclaimer missing %q", want)
		}
	}
}

// TestNoContactEmailInUserAgent enforces the standing "scrub my email" rule:
// the User-Agent must carry the GitHub URL and no email address.
func TestNoContactEmailInUserAgent(t *testing.T) {
	if strings.Contains(userAgent, "@") {
		t.Errorf("userAgent must not contain an email address: %q", userAgent)
	}
	if !strings.Contains(userAgent, "github.com/abe238/shelters-pp-cli") {
		t.Errorf("userAgent should reference the repo URL: %q", userAgent)
	}
}
