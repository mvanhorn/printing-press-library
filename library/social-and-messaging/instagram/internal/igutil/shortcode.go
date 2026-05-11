// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// Package igutil holds Instagram-specific helpers used across novel commands:
// shortcode/URL parsing, base64-url media-ID conversion, and small utilities
// that don't need their own package.

package igutil

import (
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strings"
)

// igAlphabet is Instagram's base64-url variant used to encode the numeric
// media ID into the human-readable shortcode that appears in /p/, /reel/, and
// /tv/ URLs. It matches RFC 4648 base64url: A-Z, a-z, 0-9, '-', '_'.
const igAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// shortcodeRegexp matches a bare Instagram shortcode (the [A-Za-z0-9_-]+
// portion that appears between /p/, /reel/, or /tv/ and the trailing slash).
// IG shortcodes are typically 11 characters today but historically 7-10 also
// existed; we accept 5-32 to remain forgiving for edge cases.
var shortcodeRegexp = regexp.MustCompile(`^[A-Za-z0-9_-]{5,32}$`)

// urlPathRegexp pulls the shortcode out of a /p/<sc>/, /reel/<sc>/, or
// /tv/<sc>/ URL path. The trailing slash is optional so we accept hand-typed
// URLs without one. Anchored to the path segment so query strings don't leak
// in.
var urlPathRegexp = regexp.MustCompile(`/(?:p|reel|tv|reels)/([A-Za-z0-9_-]{5,32})(?:/|$)`)

// ParseShortcode accepts either a bare shortcode (B0Lz_4QH), a full Instagram
// post URL (https://www.instagram.com/p/<sc>/), a reel URL, or an IGTV URL,
// and returns just the shortcode portion. Errors when the input doesn't look
// like any of those — bad input is louder than silent best-effort.
func ParseShortcode(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("empty shortcode or URL")
	}

	// Bare shortcode: matches the alphabet and is plausibly the right length.
	if shortcodeRegexp.MatchString(s) && !strings.Contains(s, "/") && !strings.Contains(s, ":") {
		return s, nil
	}

	// URL-shaped input. Parse it through net/url so we can canonicalize the
	// path before matching; this also tolerates schemes like instagram://
	// which net/url accepts even without a host.
	u, err := url.Parse(s)
	if err == nil && u.Path != "" {
		if m := urlPathRegexp.FindStringSubmatch(u.Path); len(m) == 2 {
			return m[1], nil
		}
	}
	// Fall back to a regex over the raw string in case it's a path fragment
	// rather than a full URL.
	if m := urlPathRegexp.FindStringSubmatch(s); len(m) == 2 {
		return m[1], nil
	}

	return "", fmt.Errorf("could not parse shortcode from %q", input)
}

// ShortcodeToMediaID converts an Instagram shortcode (e.g. "B0Lz_4QH") into
// the numeric prefix of the media ID (e.g. "2125193768487596424"). The full
// IG media ID has a "_<owner_id>" suffix that this function does NOT produce
// — that's an owner-side lookup, not a property of the shortcode. Pair the
// returned numeric prefix with /api/v1/media/<numeric>/info/ which accepts
// the bare numeric.
func ShortcodeToMediaID(shortcode string) (string, error) {
	s := strings.TrimSpace(shortcode)
	if s == "" {
		return "", fmt.Errorf("empty shortcode")
	}
	if !shortcodeRegexp.MatchString(s) {
		return "", fmt.Errorf("shortcode %q has invalid characters", shortcode)
	}
	n := new(big.Int)
	for _, ch := range s {
		idx := strings.IndexRune(igAlphabet, ch)
		if idx < 0 {
			return "", fmt.Errorf("character %q not in IG alphabet", ch)
		}
		n.Mul(n, big.NewInt(64))
		n.Add(n, big.NewInt(int64(idx)))
	}
	return n.String(), nil
}

// MediaIDToShortcode is the inverse of ShortcodeToMediaID. It accepts either
// a bare numeric ID or the full "<numeric>_<owner_id>" form (the suffix is
// stripped before conversion).
func MediaIDToShortcode(mediaID string) string {
	s := strings.TrimSpace(mediaID)
	if i := strings.Index(s, "_"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return ""
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return ""
	}
	if n.Sign() == 0 {
		return string(igAlphabet[0])
	}
	var b []byte
	zero := big.NewInt(0)
	mod := big.NewInt(0)
	div := big.NewInt(64)
	for n.Cmp(zero) > 0 {
		n.DivMod(n, div, mod)
		b = append([]byte{igAlphabet[mod.Int64()]}, b...)
	}
	return string(b)
}
