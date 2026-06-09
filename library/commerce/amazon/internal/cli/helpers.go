package cli

import (
	"context"
	"time"
)

// contextWithTimeout is a tiny shim so commands don't import "context" twice.
func contextWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// truncate caps s at n runes (with an ellipsis) for tabular output.
func truncate(s string, n int) string {
	if n <= 0 || len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// formatCents renders an integer cents amount as "$xx.yy". Returns "" for 0.
func formatCents(c int64) string {
	if c == 0 {
		return ""
	}
	dollars := c / 100
	rem := c % 100
	if rem < 0 {
		rem = -rem
	}
	return formatDollars(dollars, rem)
}

func formatDollars(d, c int64) string {
	if c < 10 {
		return "$" + itoa(d) + ".0" + itoa(c)
	}
	return "$" + itoa(d) + "." + itoa(c)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
