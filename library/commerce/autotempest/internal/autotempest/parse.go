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

// FormatCents renders integer cents as a whole-dollar display string with a
// leading "$" and thousands separators, e.g. 3049700 -> "$30,497". Cents are
// truncated to whole dollars (display convention; the precise value stays in
// the store). Returns "" for a negative/unknown value so callers can render
// blanks for missing prices.
func FormatCents(cents int64) string {
	if cents < 0 {
		return ""
	}
	return "$" + groupThousands(cents/100)
}

// groupThousands formats a non-negative integer with commas every three digits:
// 30497 -> "30,497", 1234567 -> "1,234,567", 0 -> "0".
func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	// Number of leading digits before the first comma group.
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
