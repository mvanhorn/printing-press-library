// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// Package extract pulls structured attributes (origin, producer,
// process, varietal, altitude) out of a roaster's freeform body
// description. Every roaster phrases these differently, so the
// approach is a registry of regex probes ordered from most-specific
// (key: value) to most-permissive (substring match on a known
// vocabulary).
package extract

import (
	"regexp"
	"strings"
)

// Attributes is the structured shape extracted from a body_html or
// body_text payload. Empty strings mean "not detected" — never
// fabricate a value to fill a slot.
type Attributes struct {
	Origin   string
	Producer string
	Process  string
	Varietal string
	Altitude string
}

// Cleanup strips angle-bracket HTML tags and collapses whitespace so
// downstream regex passes don't have to handle them. Doesn't try to
// be a real HTML parser — Shopify body_html is shallow-tagged enough
// that a single-pass strip is sufficient for our extraction needs.
func Cleanup(bodyHTML string) string {
	stripped := tagRE.ReplaceAllString(bodyHTML, " ")
	collapsed := wsRE.ReplaceAllString(stripped, " ")
	return strings.TrimSpace(collapsed)
}

var tagRE = regexp.MustCompile(`<[^>]+>`)
var wsRE = regexp.MustCompile(`[\s ]+`)

// FromBody runs the extraction probes against bodyText (already
// HTML-stripped via Cleanup, or plain text) and returns whatever
// could be detected. Callers should run Cleanup first.
func FromBody(bodyText string) Attributes {
	a := Attributes{}
	lower := strings.ToLower(bodyText)

	for _, p := range originProbes {
		if m := p.FindStringSubmatch(bodyText); len(m) > 1 {
			a.Origin = capWords(strings.TrimSpace(m[1]))
			break
		}
	}
	if a.Origin == "" {
		for _, country := range originVocab {
			if strings.Contains(lower, strings.ToLower(country)) {
				a.Origin = country
				break
			}
		}
	}

	for _, p := range producerProbes {
		if m := p.FindStringSubmatch(bodyText); len(m) > 1 {
			a.Producer = strings.TrimSpace(m[1])
			break
		}
	}

	for _, p := range processProbes {
		if m := p.FindStringSubmatch(bodyText); len(m) > 1 {
			a.Process = strings.ToLower(strings.TrimSpace(m[1]))
			break
		}
	}
	if a.Process == "" {
		for _, kw := range processVocab {
			if strings.Contains(lower, kw) {
				a.Process = kw
				break
			}
		}
	}

	for _, p := range varietalProbes {
		if m := p.FindStringSubmatch(bodyText); len(m) > 1 {
			a.Varietal = strings.TrimSpace(m[1])
			break
		}
	}
	if a.Varietal == "" {
		for _, v := range varietalVocab {
			if strings.Contains(lower, strings.ToLower(v)) {
				a.Varietal = v
				break
			}
		}
	}

	for _, p := range altitudeProbes {
		if m := p.FindStringSubmatch(bodyText); len(m) > 1 {
			a.Altitude = strings.TrimSpace(m[1])
			break
		}
	}

	return a
}

// capWords title-cases each word in s, used to normalize country
// names extracted via key:value probes (e.g. "ethiopia" -> "Ethiopia").
func capWords(s string) string {
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

var originProbes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Origin\s*[:\-]\s*([A-Za-z ]+?)(?:\.|,|\n|$)`),
	regexp.MustCompile(`(?i)Country\s*[:\-]\s*([A-Za-z ]+?)(?:\.|,|\n|$)`),
}

var producerProbes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Producer\s*[:\-]\s*([A-Za-z0-9 .'\-]+?)(?:\.|,|\n|$)`),
	regexp.MustCompile(`(?i)Farm\s*[:\-]\s*([A-Za-z0-9 .'\-]+?)(?:\.|,|\n|$)`),
}

var processProbes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Process(?:ing)?\s*[:\-]\s*([A-Za-z \-]+?)(?:\.|,|\n|$)`),
}

var varietalProbes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Variet(?:al|y|ies)\s*[:\-]\s*([A-Za-z0-9 ,&\-]+?)(?:\.|\n|$)`),
	regexp.MustCompile(`(?i)Cultivar\s*[:\-]\s*([A-Za-z0-9 ,&\-]+?)(?:\.|\n|$)`),
}

var altitudeProbes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:Altitude|Elevation)\s*[:\-]\s*([0-9, \-masl]+?)(?:\.|,|\n|$)`),
	regexp.MustCompile(`(\d{3,4}\s*[\-–]\s*\d{3,4}\s*m(?:asl)?)`),
	regexp.MustCompile(`(\d{3,4}\s*masl)`),
}

// originVocab is the closed set of country names recognised by the
// fallback substring matcher. Order matters: longer/specific names
// before short prefixes (e.g. "Costa Rica" before "Rica" would be
// listed but it isn't — only full names).
var originVocab = []string{
	"Ethiopia", "Kenya", "Rwanda", "Burundi", "Tanzania", "Uganda",
	"Yemen", "Colombia", "Brazil", "Peru", "Ecuador", "Bolivia",
	"Honduras", "Guatemala", "Mexico", "Nicaragua", "El Salvador",
	"Costa Rica", "Panama", "Indonesia", "Sumatra", "Java", "Bali",
	"Vietnam", "Thailand", "Laos", "China", "India",
}

// processVocab is the closed set of process methods recognised by
// the fallback matcher. Lowercase here because the lookup is
// lowercased; output is preserved lowercase per the brief.
var processVocab = []string{
	"natural", "washed", "honey", "anaerobic", "carbonic maceration",
	"wet hulled", "semi-washed", "decaf",
}

// varietalVocab is the closed set of common varietals. Same casing
// rules as originVocab.
var varietalVocab = []string{
	"Gesha", "Geisha", "Bourbon", "Typica", "Caturra", "Catuai",
	"SL28", "SL34", "Heirloom", "Pacamara", "Maragogype", "Pink Bourbon",
	"Wush Wush", "Sidra", "Castillo", "Mundo Novo", "Yellow Bourbon",
}
