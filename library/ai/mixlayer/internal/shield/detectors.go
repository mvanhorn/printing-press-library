// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package shield

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Entity struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Risk  int    `json:"risk"`
}

var detectors = []struct {
	kind string
	risk int
	re   *regexp.Regexp
}{
	{"EMAIL", 9, regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)},
	{"PHONE", 7, regexp.MustCompile(`\b(?:\+?1[\s.\-]?)?(?:\(?\d{3}\)?[\s.\-]?)\d{3}[\s.\-]?\d{4}\b`)},
	{"SSN", 10, regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
	{"IP", 5, regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)},
	{"URL", 4, regexp.MustCompile(`\bhttps?://[^\s<>"']+`)},
	{"CARD_CANDIDATE", 10, regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)},
}

func Detect(text string) []Entity {
	var out []Entity
	for _, d := range detectors {
		matches := d.re.FindAllStringIndex(text, -1)
		for _, m := range matches {
			value := text[m[0]:m[1]]
			kind := d.kind
			if kind == "CARD_CANDIDATE" {
				if !validLuhn(value) {
					continue
				}
				kind = "CARD"
			}
			if kind == "IP" && !validIPv4(value) {
				continue
			}
			out = append(out, Entity{Kind: kind, Value: value, Start: m[0], End: m[1], Risk: d.risk})
		}
	}
	out = append(out, detectLikelyNames(text)...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start == out[j].Start {
			return out[i].End > out[j].End
		}
		return out[i].Start < out[j].Start
	})
	return dedupeOverlaps(out)
}

func RiskScore(entities []Entity) int {
	maxRisk := 0
	for _, e := range entities {
		if e.Risk > maxRisk {
			maxRisk = e.Risk
		}
	}
	return maxRisk
}

func dedupeOverlaps(in []Entity) []Entity {
	var out []Entity
	for i := 0; i < len(in); {
		best := in[i]
		clusterEnd := in[i].End
		j := i + 1
		for j < len(in) && in[j].Start < clusterEnd {
			if in[j].End > clusterEnd {
				clusterEnd = in[j].End
			}
			if betterEntity(in[j], best) {
				best = in[j]
			}
			j++
		}
		out = append(out, best)
		i = j
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

func betterEntity(candidate, current Entity) bool {
	if candidate.Risk != current.Risk {
		return candidate.Risk > current.Risk
	}
	candidateLen := candidate.End - candidate.Start
	currentLen := current.End - current.Start
	if candidateLen != currentLen {
		return candidateLen > currentLen
	}
	return candidate.Start < current.Start
}

func validLuhn(s string) bool {
	var digits []int
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := digits[i]
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}

func validIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		n := 0
		for _, r := range p {
			if !unicode.IsDigit(r) {
				return false
			}
			n = n*10 + int(r-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}

func detectLikelyNames(text string) []Entity {
	words := likelyNameRe.FindAllStringIndex(text, -1)
	var out []Entity
	for _, m := range words {
		value := text[m[0]:m[1]]
		if shouldSkipLikelyName(text, m[0], value) {
			continue
		}
		out = append(out, Entity{Kind: "PERSON", Value: value, Start: m[0], End: m[1], Risk: 6})
	}
	return out
}

var likelyNameRe = regexp.MustCompile(`\b[A-Z][a-z]{2,}\s+[A-Z][a-z]{2,}\b`)

var nonPersonPhrase = map[string]struct{}{
	"Amazon Web":    {},
	"Google Cloud":  {},
	"Los Angeles":   {},
	"Mixlayer Labs": {},
	"New York":      {},
	"San Francisco": {},
}

var nonPersonToken = map[string]struct{}{
	"City":       {},
	"Cloud":      {},
	"County":     {},
	"Inc":        {},
	"Labs":       {},
	"Limited":    {},
	"Mixlayer":   {},
	"Platform":   {},
	"Services":   {},
	"Systems":    {},
	"University": {},
	"York":       {},
}

var locationPrepositions = map[string]struct{}{
	"at":   {},
	"from": {},
	"in":   {},
	"near": {},
	"to":   {},
}

func shouldSkipLikelyName(text string, start int, value string) bool {
	if _, ok := nonPersonPhrase[value]; ok {
		return true
	}
	parts := strings.Fields(value)
	for _, part := range parts {
		if _, ok := nonPersonToken[part]; ok {
			return true
		}
	}
	if prevWord := wordBefore(text[:start]); prevWord != "" {
		if _, ok := locationPrepositions[strings.ToLower(prevWord)]; ok {
			return true
		}
	}
	return false
}

func wordBefore(prefix string) string {
	prefix = strings.TrimRightFunc(prefix, func(r rune) bool {
		return !unicode.IsLetter(r)
	})
	if prefix == "" {
		return ""
	}
	start := len(prefix)
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(prefix[:start])
		if !unicode.IsLetter(r) {
			break
		}
		start -= size
	}
	return prefix[start:]
}
