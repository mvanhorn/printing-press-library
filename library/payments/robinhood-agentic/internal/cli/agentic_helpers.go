// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-written helpers shared by the transcendence commands (brief, guard,
// audit, surface, portfolio history/winrate, wheel, equities settle). Keeping
// these in one place avoids duplicate definitions across the novel command
// files.

package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// parseSince turns a duration shorthand ("30d", "12h", "90m", "2w") or an
// absolute date ("2026-07-01") into an absolute lower-bound time. An empty
// string defaults to 30 days ago.
func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Add(-30 * 24 * time.Hour), nil
	}
	// Relative: <n><unit>
	if len(s) >= 2 {
		unit := s[len(s)-1]
		if n, err := strconv.Atoi(s[:len(s)-1]); err == nil {
			switch unit {
			case 'd', 'D':
				return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
			case 'h', 'H':
				return time.Now().Add(-time.Duration(n) * time.Hour), nil
			case 'm', 'M':
				return time.Now().Add(-time.Duration(n) * time.Minute), nil
			case 'w', 'W':
				return time.Now().Add(-time.Duration(n) * 7 * 24 * time.Hour), nil
			}
		}
	}
	// Absolute date forms.
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since %q: use forms like 30d, 12h, 2w, or 2026-07-01", s)
}

// sparklineRunes are the eight ascending block glyphs used by renderSparkline.
var sparklineRunes = []rune("▁▂▃▄▅▆▇█")

// renderSparkline maps a value series to a compact block-glyph sparkline. A
// flat or single-point series renders at the mid level.
func renderSparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	span := max - min
	for _, v := range values {
		if span == 0 {
			b.WriteRune(sparklineRunes[len(sparklineRunes)/2])
			continue
		}
		idx := int(math.Round((v - min) / span * float64(len(sparklineRunes)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparklineRunes) {
			idx = len(sparklineRunes) - 1
		}
		b.WriteRune(sparklineRunes[idx])
	}
	return b.String()
}

// parseMoney parses a decimal string (Robinhood returns money as strings) into
// a float, tolerating empty and malformed values by returning 0.
func parseMoney(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
