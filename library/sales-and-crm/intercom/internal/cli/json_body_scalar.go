// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// PATCH(json-body-scalars-keep-declared-type): request-body fields that the
// API declares as integer, number, or boolean are backed by string flags in
// this print (so an omitted flag stays distinguishable from zero/false). The
// value must still go on the wire with its declared JSON type: strict APIs
// reject "page":"1" or "enabled":"true" with HTTP 400/422.

var jsonNumberLiteral = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// jsonBodyNullableKeys lists the scalar body fields this spec declares
// nullable; only these accept the literal null to clear a value.
var jsonBodyNullableKeys = map[string]bool{"help_center_id": true}

// setJSONBodyScalar stores an explicitly supplied flag in body[key] using the
// declared JSON kind ("int", "number", or "bool"). An empty value is an
// error; the literal null clears only fields declared nullable by the spec.
func setJSONBodyScalar(body map[string]any, key, flag, kind, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("--%s must not be empty", flag)
	}
	if strings.TrimSpace(raw) == "null" && !jsonBodyNullableKeys[key] {
		return fmt.Errorf("--%s cannot be null", flag)
	}
	value, ok, err := jsonBodyScalar(flag, kind, raw)
	if err != nil {
		return err
	}
	if ok {
		body[key] = value
	}
	return nil
}

func jsonBodyScalar(flag, kind, raw string) (any, bool, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, false, nil
	}
	if s == "null" {
		return nil, true, nil
	}
	switch kind {
	case "int":
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, false, fmt.Errorf("--%s must be an integer, got %q", flag, raw)
		}
		return json.Number(strconv.FormatInt(n, 10)), true, nil
	case "number":
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, false, fmt.Errorf("--%s must be a number, got %q", flag, raw)
		}
		if jsonNumberLiteral.MatchString(s) {
			return json.Number(s), true, nil
		}
		return json.Number(strconv.FormatFloat(f, 'f', -1, 64)), true, nil
	case "bool":
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, false, fmt.Errorf("--%s must be true or false, got %q", flag, raw)
		}
		return b, true, nil
	}
	return raw, true, nil
}
