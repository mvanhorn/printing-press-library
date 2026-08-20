package trustpilot

import (
	"strings"
	"testing"
)

// A reviews-endpoint 404 (Domain set) usually means the domain has no
// Trustpilot review page -- the message must lead with that hypothesis and
// point at `search`, mentioning the stale-build-id alternative second. The
// pre-patch message ("re-run auth login") sent callers chasing auth when the
// domain was simply wrong.
func TestBuildIDStaleErrorWithDomainLeadsWithDomainGuidance(t *testing.T) {
	err := &BuildIDStaleError{Status: 404, Domain: "nosuchbrandxyz"}
	msg := err.Error()
	if !strings.Contains(msg, `"nosuchbrandxyz"`) {
		t.Fatalf("message must name the missing domain, got: %s", msg)
	}
	if !strings.Contains(msg, "no Trustpilot review page found") {
		t.Fatalf("message must lead with the domain-not-found hypothesis, got: %s", msg)
	}
	if !strings.Contains(msg, "trustpilot-pp-cli search") {
		t.Fatalf("message must suggest the search command, got: %s", msg)
	}
	if !strings.Contains(msg, "auth login") {
		t.Fatalf("message must still mention the stale-build-id remediation, got: %s", msg)
	}
}

// The search endpoint's path has no domain component, so its 404 stays the
// stale-build-id signal with the original message (Domain empty).
func TestBuildIDStaleErrorWithoutDomainKeepsBuildIDMessage(t *testing.T) {
	err := &BuildIDStaleError{Status: 404}
	msg := err.Error()
	if !strings.Contains(msg, "build id rejected") {
		t.Fatalf("domain-less 404 must keep the stale-build-id message, got: %s", msg)
	}
	if strings.Contains(msg, "review page") {
		t.Fatalf("domain-less 404 must not claim a missing review page, got: %s", msg)
	}
}

// Regression guards: the sibling error types' messages are matched by callers
// and user muscle memory; this patch must not drift them.
func TestSiblingErrorMessagesUnchanged(t *testing.T) {
	if msg := (&CookieExpiredError{Status: 403}).Error(); !strings.Contains(msg, "aws-waf-token rejected") {
		t.Fatalf("CookieExpiredError message drifted: %s", msg)
	}
	if msg := (&FilterUnsupportedError{ParamHint: "date=last30days"}).Error(); !strings.Contains(msg, "unsupported filter") {
		t.Fatalf("FilterUnsupportedError message drifted: %s", msg)
	}
}
