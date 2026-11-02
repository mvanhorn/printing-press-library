// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AppendArrayQuery appends one form-style array query param to path as repeated
// keys (field=a&field=b), the shape Sentry's API expects. Each value is sent
// verbatim: Sentry fields such as count_if(a,equals,b) contain commas, so values
// are never split on commas. A value written as a JSON array is expanded, which
// keeps the generator's --flag '["a","b"]' form working.
func AppendArrayQuery(path, name string, rawValues []string) string {
	values := url.Values{}
	for _, raw := range rawValues {
		for _, v := range ArrayQueryValues(raw) {
			values.Add(name, v)
		}
	}
	return AppendQueryValues(path, values)
}

// ArrayQueryValues expands a JSON-array value into its elements and returns any
// other value unchanged. JSON numbers keep their exact digits (project IDs
// exceed float64 precision).
func ArrayQueryValues(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "[") {
		return []string{raw}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(trimmed)))
	dec.UseNumber()
	var decoded []any
	if dec.Decode(&decoded) != nil {
		return []string{raw}
	}
	out := make([]string, 0, len(decoded))
	for _, v := range decoded {
		if s, ok := v.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, fmt.Sprint(v))
		}
	}
	return out
}

// AppendQueryValues appends already-built query values to path, after any query
// the path already carries.
func AppendQueryValues(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	// url.Values.Encode writes a space as "+"; send %20 so strict parsers read a space.
	return path + separator + strings.ReplaceAll(values.Encode(), "+", "%20")
}
