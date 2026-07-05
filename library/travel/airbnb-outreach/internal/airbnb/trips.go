// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"regexp"
)

// deferredStateRe extracts the SSR Apollo cache embedded in Airbnb pages.
var deferredStateRe = regexp.MustCompile(`(?s)<script id="data-deferred-state-0"[^>]*>(.*?)</script>`)

// Trips returns the signed-in user's reservations. Airbnb server-renders the
// trips page rather than exposing a stable client query, so this fetches the
// logged-in /trips page and extracts reservation objects (those carrying a
// confirmation code) from the embedded state. Returns an empty array when there
// are no trips.
func (c *Client) Trips() (json.RawMessage, error) {
	html, err := c.GetHTML("/trips/v1")
	if err != nil {
		return nil, err
	}
	m := deferredStateRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return json.RawMessage("[]"), nil
	}
	var state any
	if err := json.Unmarshal([]byte(m[1]), &state); err != nil {
		return json.RawMessage("[]"), nil
	}
	reservations := collectObjectsWithKey(state, "confirmationCode", 0)
	out, _ := json.Marshal(reservations)
	return out, nil
}

// collectObjectsWithKey walks a decoded JSON value and returns every object that
// contains the given key, deduped by that key's value.
func collectObjectsWithKey(v any, key string, depth int) []map[string]any {
	var out []map[string]any
	seen := map[string]bool{}
	var walk func(any, int)
	walk = func(node any, d int) {
		if d > 12 {
			return
		}
		switch t := node.(type) {
		case map[string]any:
			if val, ok := t[key]; ok {
				if s, ok := val.(string); ok && s != "" && !seen[s] {
					seen[s] = true
					out = append(out, t)
				}
			}
			for _, child := range t {
				walk(child, d+1)
			}
		case []any:
			for _, child := range t {
				walk(child, d+1)
			}
		}
	}
	walk(v, depth)
	return out
}
