package nsdl

import "strings"

// ParseNumber converts an NSDL table cell into a float64. NSDL formats
// figures with Indian-style comma grouping ("5,11,510") and uses a bare "-"
// or blank cell for zero/not-applicable, never actual dashes-as-negative.
// Returns 0, false for non-numeric or empty cells so callers can distinguish
// "genuinely zero" from "not a number" when needed.
func ParseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" || s == "NA" || s == "N/A" {
		return 0, false
	}
	s = strings.ReplaceAll(s, ",", "")
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
		s = strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
	}
	if s == "" {
		return 0, false
	}
	var intPart, fracPart string
	dotIdx := strings.Index(s, ".")
	if dotIdx >= 0 {
		intPart = s[:dotIdx]
		fracPart = s[dotIdx+1:]
	} else {
		intPart = s
	}
	startIdx := 0
	sign := 1.0
	if len(intPart) > 0 && (intPart[0] == '-' || intPart[0] == '+') {
		if intPart[0] == '-' {
			sign = -1
		}
		startIdx = 1
	}
	var value float64
	for i := startIdx; i < len(intPart); i++ {
		c := intPart[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		value = value*10 + float64(c-'0')
	}
	if fracPart != "" {
		div := 1.0
		for i := 0; i < len(fracPart); i++ {
			c := fracPart[i]
			if c < '0' || c > '9' {
				return 0, false
			}
			div *= 10
			value += float64(c-'0') / div
		}
	}
	value *= sign
	if neg {
		value = -value
	}
	return value, true
}
