// Package normalize converts domain inputs to canonical ASCII (punycode) form.
package normalize

import (
	"errors"
	"strings"

	"golang.org/x/net/idna"
)

// FQDN normalizes a domain name to lowercase ASCII (punycode for IDN).
// Returns ("", error) when the input cannot be encoded.
func FQDN(in string) (string, error) {
	s := strings.TrimSpace(in)
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return "", errors.New("empty domain")
	}
	s = strings.ToLower(s)
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return "", err
	}
	if !strings.Contains(ascii, ".") {
		return "", errors.New("no TLD")
	}
	return ascii, nil
}

// SplitTLD returns (label, tld) from a normalized FQDN. For "example.co.uk"
// it returns ("example", "co.uk") when "co.uk" is detected as a multi-label
// public suffix — for now we use a simple last-dot split which works for
// the common .com/.io/.ai/.app/.dev TLDs the CLI targets.
func SplitTLD(fqdn string) (label, tld string) {
	idx := strings.Index(fqdn, ".")
	if idx < 0 {
		return fqdn, ""
	}
	return fqdn[:idx], fqdn[idx+1:]
}

// Unicode returns the Unicode (non-punycode) form of a domain for display.
func Unicode(ascii string) string {
	out, err := idna.Lookup.ToUnicode(ascii)
	if err != nil {
		return ascii
	}
	return out
}
