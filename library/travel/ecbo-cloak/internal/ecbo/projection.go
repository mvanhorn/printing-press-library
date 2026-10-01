package ecbo

import (
	"fmt"
	"strings"
)

// Project narrows an object using dot paths; missing source values remain null.
func Project(v map[string]any, fields string) (map[string]any, error) {
	if fields == "" {
		return v, nil
	}
	paths := strings.Split(fields, ",")
	for i, a := range paths {
		for j, b := range paths {
			if i != j && (a == b || strings.HasPrefix(a, b+".")) {
				return nil, &Error{2, "select", "duplicate or overlapping projection paths"}
			}
		}
	}
	out := map[string]any{}
	for _, path := range strings.Split(fields, ",") {
		if strings.TrimSpace(path) != path || path == "" {
			return nil, &Error{2, "select", "use nonempty comma-separated dot paths"}
		}
		parts := strings.Split(path, ".")
		src, dst := v, out
		for i, p := range parts {
			value, ok := src[p]
			if !ok {
				return nil, &Error{2, "select", fmt.Sprintf("unknown field %q", path)}
			}
			if i == len(parts)-1 {
				if prior, exists := dst[p]; exists {
					if _, ok := prior.(map[string]any); ok {
						return nil, &Error{2, "select", "overlapping projection paths"}
					}
				}
				dst[p] = value
				break
			}
			m, ok := value.(map[string]any)
			if !ok {
				return nil, &Error{2, "select", fmt.Sprintf("field %q is not an object", p)}
			}
			src = m
			if prior, exists := dst[p]; exists {
				next, ok := prior.(map[string]any)
				if !ok {
					return nil, &Error{2, "select", "overlapping projection paths"}
				}
				dst = next
			} else {
				next := map[string]any{}
				dst[p] = next
				dst = next
			}
		}
	}
	return out, nil
}

// ValidateListProjection uses the public list schema even when there are no rows.
func ValidateListProjection(fields string) error {
	schema := map[string]any{"id": nil, "name": nil, "name_ja": nil, "latitude": nil, "longitude": nil, "distance_km": nil, "booking_url": nil, "canonical_uuid": nil, "url_note": nil, "listed": nil, "availability": nil, "confirmed_available_capacity": nil, "daily_prices": []any{}, "acceptance_cutoff": nil, "pickup_cutoff": nil, "source_item_counts": map[string]any{"small": nil, "large": nil, "meaning": nil}, "listed_hours": map[string]any{"from": nil, "to": nil, "is_24_hours": nil}}
	_, e := Project(schema, fields)
	return e
}
