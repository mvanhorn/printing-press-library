// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// parseInt parses a string to int for novel commands that accept an ID argument.
func parseInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("must be an integer: %q", s)
	}
	return n, nil
}

// parseOptionalDomain parses an Odoo domain JSON string into []interface{}.
// Returns an empty domain if s is empty or invalid.
func parseOptionalDomain(s string) []interface{} {
	if s == "" {
		return []interface{}{}
	}
	var d []interface{}
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return []interface{}{}
	}
	return d
}
