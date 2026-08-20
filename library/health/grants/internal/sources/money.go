package sources

import (
	"strconv"
	"strings"
)

// ParseMoney is a tolerant money parser:
// "$1,500,000", "500000.00", 500000.0 (float64), nil, "none", "" → int64 (0 when not a number).
func ParseMoney(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(x)
	case string:
		s := strings.TrimSpace(x)
		s = strings.NewReplacer("$", "", ",", "", " ", "").Replace(s)
		if s == "" {
			return 0
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}
