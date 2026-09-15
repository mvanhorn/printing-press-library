package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// A UniFi OS console answers a path it does not serve with its own HTML page rather
// than a JSON 404 body. That makes an HTML response evidence about WHERE the request
// went at least as often as evidence about the credential, so it must not be reported
// as an authentication failure: the guidance would point at a key that is intact, and
// the exit code would tell scripts to re-authenticate.
func TestNonJSONPayloadErrorClassifiesConsolePageAsAPIError(t *testing.T) {
	consolePage := json.RawMessage("<!doctype html><html><head><title>UniFi OS</title>" +
		`<script>window.UNIFI_OS_MANIFEST = {"modules":[]}</script></head><body></body></html>`)

	err := nonJSONPayloadError(consolePage)
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("want a *cliError, got %T", err)
	}
	if ce.code != 5 {
		t.Errorf("console-page exit code = %d, want 5 (apiErr); this is not an authentication failure", ce.code)
	}
	msg := err.Error()
	if !strings.Contains(msg, "base_url") || !strings.Contains(msg, "/proxy/network") {
		t.Errorf("message must name base_url and the /proxy/network prefix, got: %s", msg)
	}
	if strings.Contains(msg, "Set your API key") {
		t.Errorf("message still sends the operator to the credential, got: %s", msg)
	}
}

// An HTML body that is not the console's own page keeps the original classification.
// With no evidence that the URL was wrong, an auth-shaped failure stays the best
// available reading; this pins that as a decision rather than an accident.
func TestNonJSONPayloadErrorKeepsAuthClassForOtherHTML(t *testing.T) {
	err := nonJSONPayloadError(json.RawMessage("<html><body>403 Forbidden</body></html>"))
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("want a *cliError, got %T", err)
	}
	if ce.code != 4 {
		t.Errorf("non-console HTML exit code = %d, want 4 (authErr)", ce.code)
	}
}
