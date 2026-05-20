// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runLocationResolve(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	cmd := newRootCmd(&rootFlags{})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	full := append([]string{"location", "resolve"}, args...)
	cmd.SetArgs(full)
	err = cmd.Execute()
	// Strip whitespace from output to make substring assertions
	// format-agnostic (printJSONFiltered uses pretty-printed JSON by
	// default).
	stripped := stripJSONWhitespace(out.String())
	return stripped, errOut.String(), err
}

// stripJSONWhitespace removes formatting whitespace from JSON so
// substring assertions can match `"key":"value"` regardless of
// whether the output was pretty-printed.
func stripJSONWhitespace(s string) string {
	var b strings.Builder
	inString := false
	prevEscape := false
	for _, r := range s {
		if inString {
			b.WriteRune(r)
			if r == '\\' && !prevEscape {
				prevEscape = true
				continue
			}
			if r == '"' && !prevEscape {
				inString = false
			}
			prevEscape = false
			continue
		}
		if r == '"' {
			inString = true
			b.WriteRune(r)
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestLocationResolve_HighConfidence — specific city+state returns a
// GeoContext with HIGH-tier source-of-truth fields populated.
func TestLocationResolve_HighConfidence(t *testing.T) {
	stdout, _, err := runLocationResolve(t, "bellevue, wa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, needle := range []string{
		`"resolved_to":"Bellevue, WA"`,
		`"source":"explicit_flag"`,
		`"radius_km":25`,
	} {
		if !strings.Contains(stdout, needle) {
			t.Errorf("expected %q in stdout, got:\n%s", needle, stdout)
		}
	}
	if strings.Contains(stdout, `"needs_clarification":true`) {
		t.Errorf("specific input should not return envelope; got:\n%s", stdout)
	}
}

// TestLocationResolve_Envelope — bare ambiguous input returns the
// disambiguation envelope at exit 0. The envelope's JSON is the only
// thing on stdout (no GeoContext wrapper).
func TestLocationResolve_Envelope(t *testing.T) {
	stdout, _, err := runLocationResolve(t, "bellevue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, needle := range []string{
		`"needs_clarification":true`,
		`"error_kind":"location_ambiguous"`,
		`"what_was_asked":"bellevue"`,
	} {
		if !strings.Contains(stdout, needle) {
			t.Errorf("expected %q in stdout, got:\n%s", needle, stdout)
		}
	}
	// At least one Bellevue candidate must be present.
	if !strings.Contains(stdout, "Bellevue, WA") {
		t.Errorf("expected Bellevue, WA candidate in envelope; got:\n%s", stdout)
	}
}

// TestLocationResolve_BatchAcceptAmbiguous — --batch-accept-ambiguous
// on a low-confidence input forces a pick and returns the GeoContext
// instead of the envelope. The verbose flag name signals batch-only
// usage and discourages interactive misuse.
func TestLocationResolve_BatchAcceptAmbiguous(t *testing.T) {
	stdout, _, err := runLocationResolve(t, "bellevue", "--batch-accept-ambiguous")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stdout, `"needs_clarification":true`) {
		t.Errorf("--batch-accept-ambiguous should suppress envelope; got envelope:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"resolved_to":"Bellevue, WA"`) {
		t.Errorf("expected Bellevue, WA forced pick; got:\n%s", stdout)
	}
}

// TestLocationResolve_Coords — coords resolve via reverse-lookup to
// the smallest containing Place.
func TestLocationResolve_Coords(t *testing.T) {
	stdout, _, err := runLocationResolve(t, "47.6101,-122.2015")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, `"resolved_to":"Bellevue, WA"`) {
		t.Errorf("expected reverse-lookup to Bellevue WA; got:\n%s", stdout)
	}
}

// TestLocationResolve_Unknown — bare token that doesn't match any
// place returns an envelope with location_unknown error_kind.
func TestLocationResolve_Unknown(t *testing.T) {
	stdout, _, err := runLocationResolve(t, "totally-fake-place-12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, `"error_kind":"location_unknown"`) {
		t.Errorf("expected location_unknown envelope; got:\n%s", stdout)
	}
}

// TestLocationResolve_ParseError — invalid coords surface as a
// command error (non-nil error from Execute).
func TestLocationResolve_ParseError(t *testing.T) {
	_, _, err := runLocationResolve(t, "100.5,200.3")
	if err == nil {
		t.Errorf("expected parse error for out-of-range coords")
	}
}
