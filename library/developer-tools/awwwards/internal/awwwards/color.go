// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
package awwwards

import (
	"fmt"
	"math"
	"strings"
)

// RGB is a parsed hex color.
type RGB struct {
	R, G, B int
}

// ParseHex parses "#RRGGBB" or "RRGGBB" (case-insensitive) into an RGB.
func ParseHex(s string) (RGB, error) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return RGB{}, fmt.Errorf("invalid hex color %q: want #RRGGBB", s)
	}
	var c RGB
	if _, err := fmt.Sscanf(strings.ToLower(h), "%02x%02x%02x", &c.R, &c.G, &c.B); err != nil {
		return RGB{}, fmt.Errorf("invalid hex color %q: %w", s, err)
	}
	return c, nil
}

// Distance returns the Euclidean RGB distance between two colors (0-441.7).
func Distance(a, b RGB) float64 {
	dr, dg, db := float64(a.R-b.R), float64(a.G-b.G), float64(a.B-b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// HueFamily buckets a color into a coarse named family for trend grouping.
func HueFamily(c RGB) string {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l := (max + min) / 2
	d := max - min

	if d < 0.09 {
		switch {
		case l < 0.15:
			return "black"
		case l > 0.87:
			return "white"
		default:
			return "gray"
		}
	}

	var h float64
	switch max {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}

	switch {
	case h < 15 || h >= 345:
		return "red"
	case h < 45:
		return "orange"
	case h < 70:
		return "yellow"
	case h < 165:
		return "green"
	case h < 200:
		return "cyan"
	case h < 260:
		return "blue"
	case h < 300:
		return "purple"
	default:
		return "pink"
	}
}
