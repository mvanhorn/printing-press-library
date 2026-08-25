package cli

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

// parseFlexible: flag.Parse stops at the first positional argument, so in
// `search keyword --rows 5` the flags would be swallowed into the keyword.
// Re-parsing peels off positionals, letting flags appear anywhere.
func parseFlexible(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	return pos, nil
}

// Pure, testable filter logic.

const grantsDateLayout = "01/02/2006" // Grants.gov: MM/DD/YYYY

// ParseGrantsDate parses a Grants.gov MM/DD/YYYY date.
func ParseGrantsDate(s string) (time.Time, error) {
	return time.Parse(grantsDateLayout, strings.TrimSpace(s))
}

// ClosingBefore keeps opportunities whose close date is valid and on/before cutoff.
func ClosingBefore(opps []sources.Opportunity, cutoff time.Time) []sources.Opportunity {
	var out []sources.Opportunity
	for _, o := range opps {
		d, err := ParseGrantsDate(o.CloseDate)
		if err != nil {
			continue // no parseable close date; the filter asks about deadlines, so drop it
		}
		if !d.After(cutoff) {
			out = append(out, o)
		}
	}
	return out
}

// EligibilityMatches reports a case-insensitive substring hit in any applicant type.
func EligibilityMatches(types []string, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	for _, t := range types {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// FormatMoney: 1234567 → "$1,234,567"; 0 → "—".
func FormatMoney(n int64) string {
	if n <= 0 {
		return "—"
	}
	s := fmt.Sprint(n)
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return "$" + b.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
