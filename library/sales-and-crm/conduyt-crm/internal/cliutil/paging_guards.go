package cliutil

import (
	"fmt"
	"math"
	"strings"
)

// Hand-written paging guards shared by the duplicate-proposal list commands and the MCP code-orchestration executor
// (duplicate proposals look, board #95). Not generated: the generator emits plain int and string flags with no bounds, so a
// reprint would send 0, 101 or a blank cursor to the API. A reprint must keep these.

// CheckPageSize requires a page size (or page number) in 1..max.
func CheckPageSize(name string, v, max int) error {
	if v < 1 || v > max {
		return fmt.Errorf("%s must be a whole number 1-%d (got %d)", name, max, v)
	}
	return nil
}

// CheckCursor refuses an opaque cursor that was given but is blank: the API would answer the first page, silently restarting.
func CheckCursor(name string, given bool, v string) error {
	if given && strings.TrimSpace(v) == "" {
		return fmt.Errorf("%s was given but is blank: pass the cursor from the previous page unchanged, or omit it for the first page", name)
	}
	return nil
}

// CheckPagingParams applies the same rules to an executor params map: each present size key (name -> max) must be a whole
// JSON number in 1..max, and each present cursor key a non-blank string.
func CheckPagingParams(params map[string]any, sizes map[string]int, cursors []string) error {
	for name, max := range sizes {
		v, ok := params[name]
		if !ok {
			continue
		}
		f, isNum := v.(float64)
		if !isNum || f != math.Trunc(f) {
			return fmt.Errorf("%s must be a whole number 1-%d", name, max)
		}
		if f < 1 || f > float64(max) {
			return fmt.Errorf("%s must be a whole number 1-%d (got %v)", name, max, v)
		}
	}
	for _, name := range cursors {
		v, ok := params[name]
		if !ok {
			continue
		}
		s, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("%s must be the opaque cursor string from the previous page", name)
		}
		if err := CheckCursor(name, true, s); err != nil {
			return err
		}
	}
	return nil
}
