// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package rechtspraak

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ParsedECLI holds the destructured pieces of an ECLI.
type ParsedECLI struct {
	Raw      string `json:"raw"`
	Country  string `json:"country"`           // ISO 3166-1 alpha-2, e.g. NL, EU, CE
	Court    string `json:"court"`             // e.g. HR, RBAMS, C, ECHR
	Year     string `json:"year"`              // e.g. 2024
	Sequence string `json:"sequence"`          // e.g. 1312, 0204JUD003675797
	Variant  string `json:"variant,omitempty"` // standard | eu | echr | legacy
	URL      string `json:"url,omitempty"`     // deeplink for NL ECLIs
}

// ecliCore matches the basic 5-segment ECLI: ECLI:<country>:<court>:<year>:<seq>
// All four segments after ECLI: are required and non-empty.
var ecliCore = regexp.MustCompile(`^ECLI:([A-Z][A-Z]):([A-Za-z0-9]+):(\d{4}):([A-Za-z0-9_]+(?:JUD\d+)?)$`)

// ljnRe matches a bare LJN (4-letter prefix + 4 alphanumerics) or LJN:AA1234.
var ljnRe = regexp.MustCompile(`^(?:LJN:)?([A-Z]{2}\d{4})$`)

// ParseECLI parses an ECLI string. Handles NL, EU, CE/ECHR formats.
//
// This is a SYNTAX-ONLY check: the regex enforces shape (country code,
// court segment, four-digit year, sequence) but does not validate the
// court segment against the Instanties vocabulary or the country code
// against ISO 3166. A well-formed ECLI with an unknown court will parse
// successfully and surface as a 404 from the upstream content endpoint.
// Callers that need semantic validation should resolve the parsed Court
// against the Instanties cache via internal/cli.getCourtIndex.
//
// Input is normalized before matching: leading/trailing whitespace is
// trimmed, the "ECLI:" prefix is added if missing (case-insensitive),
// and the country/court segments are upper-cased. The IVO 1.15 doc and
// the e-justice ECLI spec both define ECLIs as case-insensitive with
// uppercase country/court segments preferred; lower-case pastes from
// search engines or browser bars now parse rather than rejecting.
func ParseECLI(s string) (ParsedECLI, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ParsedECLI{}, errors.New("empty ECLI")
	}
	// Strip a case-insensitive "ECLI:" prefix if present; add one otherwise.
	if len(s) >= 5 && strings.EqualFold(s[:5], "ECLI:") {
		s = "ECLI:" + s[5:]
	} else {
		s = "ECLI:" + s
	}
	// Uppercase the country (2 chars) and court segments. Sequence is left
	// case-preserved because ECHR sequences embed "JUD" labels and other
	// case-sensitive markers; the regex below tolerates them either way.
	if parts := strings.SplitN(s, ":", 5); len(parts) == 5 {
		parts[1] = strings.ToUpper(parts[1])
		parts[2] = strings.ToUpper(parts[2])
		s = strings.Join(parts, ":")
	}
	m := ecliCore.FindStringSubmatch(s)
	if m == nil {
		// Try legacy LJN as a soft fallback so callers can route to the LJN
		// resolver rather than failing here.
		if ljn := ljnRe.FindStringSubmatch(s); ljn != nil {
			return ParsedECLI{
				Raw:     s,
				Variant: "legacy-ljn",
				// Court / year / sequence are not directly decodable from an LJN.
				Sequence: ljn[1],
			}, nil
		}
		return ParsedECLI{}, fmt.Errorf("not a well-formed ECLI: %q", s)
	}
	p := ParsedECLI{
		Raw:      s,
		Country:  m[1],
		Court:    m[2],
		Year:     m[3],
		Sequence: m[4],
		Variant:  "standard",
	}
	switch {
	case p.Country == "NL":
		p.URL = "https://uitspraken.rechtspraak.nl/details?id=" + s
	case p.Country == "EU":
		p.Variant = "eu"
		p.URL = "https://curia.europa.eu/juris/liste.jsf?ecli=" + s
	case p.Country == "CE":
		p.Variant = "echr"
		p.URL = "https://hudoc.echr.coe.int/eng?i=" + s
	default:
		p.URL = "https://e-justice.europa.eu/ecliSearch?ecli=" + s
	}
	return p, nil
}

// IsLJN reports whether s is a bare LJN code (without the ECLI: prefix).
func IsLJN(s string) bool {
	return ljnRe.MatchString(strings.TrimSpace(s))
}

// LJNCode extracts the LJN value if s is recognizable, else empty string.
func LJNCode(s string) string {
	m := ljnRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return ""
	}
	return m[1]
}

// ValidateECLI returns nil iff s is a well-formed ECLI; uses ParseECLI under the hood.
func ValidateECLI(s string) error {
	_, err := ParseECLI(s)
	if err != nil {
		return err
	}
	return nil
}

// DeeplinkURL returns the rechtspraak.nl deeplink for an NL ECLI, or empty if
// the input is not an NL ECLI.
func DeeplinkURL(s string) string {
	p, err := ParseECLI(s)
	if err != nil || p.Country != "NL" {
		return ""
	}
	return p.URL
}
