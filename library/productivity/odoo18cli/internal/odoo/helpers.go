// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package odoo

import (
	"fmt"
	"strconv"
)

// IDFromMany2one extracts the integer ID from an Odoo Many2one field.
// Odoo returns Many2one as [id, "name"] or false.
func IDFromMany2one(v interface{}) int {
	if v == nil {
		return 0
	}
	arr, ok := v.([]interface{})
	if !ok || len(arr) < 1 {
		return 0
	}
	switch id := arr[0].(type) {
	case int:
		return id
	case int64:
		return int(id)
	case float64:
		return int(id)
	case string:
		n, _ := strconv.Atoi(id)
		return n
	}
	return 0
}

// NameFromMany2one extracts the display name from an Odoo Many2one field.
func NameFromMany2one(v interface{}) string {
	if v == nil {
		return ""
	}
	arr, ok := v.([]interface{})
	if !ok || len(arr) < 2 {
		return ""
	}
	return fmt.Sprintf("%v", arr[1])
}

// StringVal safely converts an interface{} to string (Odoo returns false for empty fields).
func StringVal(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case bool:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// BoolVal safely converts an interface{} to bool.
func BoolVal(v interface{}) bool {
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// IntVal safely converts an interface{} to int.
func IntVal(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// FloatVal safely converts an interface{} to float64.
func FloatVal(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
