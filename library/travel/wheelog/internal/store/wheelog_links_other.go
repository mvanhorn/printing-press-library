//go:build !windows

// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"fmt"
	"os"
	"reflect"
)

// Unix Stat exposes Nlink. Unknown metadata fails closed because alternate
// hard-link WAL names cannot be safely inferred from the selected path.
func wheelogFileHasSingleLink(_ string, info os.FileInfo) (bool, error) {
	v := reflect.ValueOf(info.Sys())
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		links := v.FieldByName("Nlink")
		if links.CanUint() {
			return links.Uint() == 1, nil
		}
		if links.CanInt() && links.Int() >= 0 {
			return links.Int() == 1, nil
		}
	}
	return false, fmt.Errorf("OS file metadata does not expose a link count")
}
