package cli

import (
	"strings"
	"testing"
)

func TestNonJSONPayloadErrorHTMLUsesPublicSiteGuidance(t *testing.T) {
	err := nonJSONPayloadError([]byte("<html><title>blocked</title></html>"))
	if err == nil {
		t.Fatal("HTML payload must be rejected")
	}
	message := err.Error()
	for _, unwanted := range []string{"not authenticated", "session expired", "credentials"} {
		if strings.Contains(strings.ToLower(message), unwanted) {
			t.Fatalf("HTML guidance incorrectly contains %q: %q", unwanted, message)
		}
	}
	for _, wanted := range []string{"unexpected HTML", "public site", "bot protection", "doctor", "connectivity and local state"} {
		if !strings.Contains(strings.ToLower(message), strings.ToLower(wanted)) {
			t.Fatalf("HTML guidance missing %q: %q", wanted, message)
		}
	}
}
