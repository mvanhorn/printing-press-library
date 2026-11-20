// etsy-ext: hand-authored helper for comma-separated Etsy form fields (tags, materials).
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// etsyCSVFieldValue normalizes a --tags/--materials flag to the comma-separated
// string Etsy's form API expects. A JSON array of strings (the old generated
// spelling, e.g. '["a","b"]') is joined with commas; anything else is treated
// as already comma-separated, trimming spaces around each element and dropping empties.
func etsyCSVFieldValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		var values []any
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return "", fmt.Errorf("invalid JSON array of strings: %w", err)
		}
		parts := make([]string, 0, len(values))
		for _, value := range values {
			s, ok := value.(string)
			if !ok {
				return "", fmt.Errorf("invalid JSON array of strings: non-string element")
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ","), nil
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ","), nil
}
