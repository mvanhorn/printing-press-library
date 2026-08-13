// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// errExitCode returns the cliError exit code for err, or -1 if err is not a
// *cliError. Mirrors how the top-level Execute maps errors to process exits.
func errExitCode(err error) int {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	return -1
}

func TestAssertLiveJSONBody(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode int // -1 means expect nil error
	}{
		{"html doctype login page", "<!doctype html><html><body>login</body></html>", 4},
		{"html with leading whitespace", "\n  <html></html>", 4},
		{"empty array", "[]", -1},
		{"empty object", "{}", -1},
		{"designs array", `[{"id":1}]`, -1},
		{"empty body", "", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertLiveJSONBody(json.RawMessage(tc.body))
			if got := errExitCode(err); got != tc.wantCode {
				t.Fatalf("assertLiveJSONBody(%q) exit code = %d, want %d (err=%v)", tc.body, got, tc.wantCode, err)
			}
		})
	}
}

func TestWrapWithProvenanceRejectsHTML(t *testing.T) {
	prov := DataProvenance{Source: "live"}

	// The reported bug: an HTML login page (HTTP 200 on expired session) must
	// not be stringified into results with a success exit — it must error (4).
	wrapped, err := wrapWithProvenance(json.RawMessage("<!doctype html><html></html>"), prov)
	if got := errExitCode(err); got != 4 {
		t.Fatalf("wrapWithProvenance(HTML) exit code = %d, want 4 (err=%v)", got, err)
	}
	if wrapped != nil {
		t.Fatalf("wrapWithProvenance(HTML) returned non-nil envelope: %s", wrapped)
	}

	// Other non-JSON garbage is an API error (5), not auth.
	if got := errExitCode(func() error { _, e := wrapWithProvenance(json.RawMessage("not json"), prov); return e }()); got != 5 {
		t.Fatalf("wrapWithProvenance(garbage) exit code = %d, want 5", got)
	}

	// Valid JSON still wraps into the {meta, results} envelope.
	wrapped, err = wrapWithProvenance(json.RawMessage(`[{"id":1}]`), prov)
	if err != nil {
		t.Fatalf("wrapWithProvenance(valid JSON) unexpected error: %v", err)
	}
	if !strings.Contains(string(wrapped), `"source":"live"`) || !strings.Contains(string(wrapped), `"results"`) {
		t.Fatalf("wrapWithProvenance(valid JSON) envelope missing meta/results: %s", wrapped)
	}
}
