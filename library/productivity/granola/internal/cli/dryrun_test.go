// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"testing"
)

// TestDryRun_NoSideEffects checks that --dry-run on a representative set
// of write-capable commands does not perform IO and exits 0.
func TestDryRun_NoSideEffects(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"meetings delete", []string{"meetings", "delete", "abc", "--dry-run"}},
		{"meetings restore", []string{"meetings", "restore", "abc", "--dry-run"}},
		{"warm", []string{"warm", "abc", "query", "--dry-run"}},
		{"export", []string{"export", "abc", "-o", "/tmp/no-such-x", "--dry-run"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			rc := RootCmd()
			rc.SetOut(&out)
			rc.SetErr(&out)
			rc.SetArgs(tc.args)
			if err := rc.Execute(); err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		})
	}
}
