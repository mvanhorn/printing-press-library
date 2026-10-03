//go:build !windows

// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hostelworld

import (
	"os"
	"reflect"
)

func cacheLinkCount(_ string, info os.FileInfo) (uint64, error) {
	v := reflect.Indirect(reflect.ValueOf(info.Sys()))
	if v.IsValid() && v.Kind() == reflect.Struct {
		n := v.FieldByName("Nlink")
		if n.IsValid() {
			switch n.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return n.Uint(), nil
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if n.Int() > 0 {
					return uint64(n.Int()), nil
				}
			}
		}
	}
	return 0, &CacheVisibilityError{Reason: "the filesystem cannot verify a unique cache file identity"}
}
