// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"reflect"
	"testing"
)

func TestArrayQueryValuesRequiresWholeJSONArray(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []string
	}{
		{`[1]rc`, []string{`[1]rc`}},
		{`[1] [2]`, []string{`[1] [2]`}},
		{`[1] null`, []string{`[1] null`}},
		{` [1]suffix `, []string{` [1]suffix `}},
		{"[1] \n\t", []string{"1"}},
		{`["release",9007199254740993]`, []string{"release", "9007199254740993"}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := ArrayQueryValues(tc.raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ArrayQueryValues(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
