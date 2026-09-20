// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-multi-segment-path-param.json.

package cli

import (
	"fmt"
	"net/url"
	"strings"
)

// replaceEncodedSubPath substitutes an ALREADY PERCENT-ENCODED, possibly
// multi-segment value into a path template, verbatim.
//
// It exists because the generated replacePathParam routes every path param
// through url.PathEscape, which is correct for a single segment and wrong for
// a parameter that IS a sub-path. NEPRA's reliability resource is the latter:
// the spec declares `/Standards/{path}` where {path} is
// "2020/PER%20DISCOs%202018-19.pdf". PathEscape turns the separator into %2F
// and re-escapes the already-escaped spaces, so the request became
//
//	/Standards/2020%2FPER%2520DISCOs%25202018-19.pdf
//
// against a real path of
//
//	/Standards/2020/PER%20DISCOs%202018-19.pdf
//
// which 404s. That blocked every Performance Evaluation Report — the entire
// input surface of internal/nepraper.
//
// The value is taken verbatim rather than re-escaped because the spec's own
// description says the path is discovered from `sources` and copied, its
// example and happy_args are both pre-encoded, and one published filename
// carries a LOAD-BEARING TRAILING SPACE that survives only as %20. Re-escaping
// a value that is already encoded cannot be done safely without guessing.
//
// A value that is not already encoded is REFUSED rather than repaired, so a
// caller never gets a silently wrong URL.
func replaceEncodedSubPath(path, name, value string) (string, error) {
	if err := validateEncodedSubPath(name, value); err != nil {
		return "", err
	}
	return strings.ReplaceAll(path, "{"+name+"}", value), nil
}

// validateEncodedSubPath rejects a value that is not safe to place in a URL
// path verbatim. It deliberately does NOT try to fix anything.
func validateEncodedSubPath(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.HasPrefix(value, "/") {
		return fmt.Errorf("%s must be relative, not start with %q: got %q", name, "/", value)
	}
	// TRAVERSAL IS CHECKED AFTER DECODING, NOT BEFORE.
	//
	// This function deliberately accepts and preserves percent-escapes, so a
	// literal substring test for ".." is trivially bypassed: "%2e%2e" is the
	// same path segment to any server and contains no '.' at all. MEASURED
	// before the fix: `reliability '2020/%2e%2e/%2e%2e/etc/passwd'` built
	// GET /Standards/2020/%2e%2e/%2e%2e/etc/passwd and exited 0. The raw form
	// is still checked too, because a value may be only partly encoded.
	if strings.Contains(value, "..") {
		return fmt.Errorf("%s must not contain %q: got %q", name, "..", value)
	}
	if decoded, err := url.PathUnescape(value); err == nil && strings.Contains(decoded, "..") {
		return fmt.Errorf("%s must not contain %q once percent-escapes are decoded: %q decodes to %q",
			name, "..", value, decoded)
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c == '%':
			// A '%' must begin a valid two-hex-digit escape, or the value is
			// half-encoded and we cannot tell which half.
			if i+2 >= len(value) || !isHexDigit(value[i+1]) || !isHexDigit(value[i+2]) {
				return fmt.Errorf("%s contains a stray %% at byte %d: %q is not percent-encoded correctly; "+
					"pass the published path exactly as %s reports it", name, i, value, "sources")
			}
			i += 2
		case c == '?' || c == '#' || c == '\\':
			// None of these is a path byte. A '?' turns everything after it
			// into a query string on the outgoing request and a '#' into a
			// fragment the server never sees, so either one silently fetches
			// a DIFFERENT document than the caller named — and a published
			// NEPRA filename contains none of them. A backslash is not a
			// separator here either.
			return fmt.Errorf("%s contains %q at byte %d, which is not a path character: %q would fetch a "+
				"different document than it names. Percent-encode it if it is genuinely part of the "+
				"filename, and pass the published path exactly as %s reports it",
				name, string(c), i, value, "sources")
		case c == ' ':
			return fmt.Errorf("%s contains a raw space: %q must be percent-encoded (use %%20); "+
				"pass the published path exactly as %s reports it", name, value, "sources")
		case c < 0x21 || c > 0x7E:
			return fmt.Errorf("%s contains a character that must be percent-encoded at byte %d: %q", name, i, value)
		}
	}
	return nil
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
