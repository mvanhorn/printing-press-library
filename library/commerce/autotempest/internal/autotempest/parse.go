// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

import (
	"strconv"
	"strings"
)

// ParsePriceCents converts a display price like "$30,497" or "$30,497.50" into
// integer cents (3049700 / 3049750). Returns -1 for an empty or non-numeric
// value so "unknown" is distinguishable from a real $0.
func ParsePriceCents(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	// Strip currency symbols, thousands separators, and surrounding noise.
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if cleaned == "" || cleaned == "-" || cleaned == "." {
		return -1
	}
	f, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return -1
	}
	if f < 0 {
		return -1
	}
	// Round to nearest cent.
	return int64(f*100 + 0.5)
}

// ParseMileage converts a display mileage like "24,755" into 24755. Returns -1
// for empty/non-numeric input.
func ParseMileage(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if cleaned == "" {
		return -1
	}
	n, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// ParseYear converts a year string like "2016" into 2016. Returns 0 for
// empty/non-numeric input.
func ParseYear(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
