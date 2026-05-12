package cli

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// truncStr returns s truncated to n runes (not bytes), safe for unicode.
func truncStr(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		count++
		if count > n {
			return s[:i]
		}
	}
	return s
}

func itoa(i int) string { return strconv.Itoa(i) }

func ftoa(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

// parseInt is best-effort: empty or unparseable values return 0 rather than
// erroring, because AppsFlyer Pull V2 CSVs occasionally blank a metric for
// "restricted" media-source rows.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		// Some CSVs report floats where we expect ints (e.g. "1.0").
		if f, ferr := strconv.ParseFloat(s, 64); ferr == nil {
			return int(f)
		}
		return 0
	}
	return n
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
