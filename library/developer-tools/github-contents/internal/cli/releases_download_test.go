// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"runtime"
	"testing"
)

func TestSafeAssetName(t *testing.T) {
	t.Parallel()

	// Colon-bearing names inherit SafeRelPath's OS-conditional policy:
	// rejected on Windows (drive-relative / NTFS stream syntax), ordinary
	// single-level filenames on Unix.
	colonRejected := runtime.GOOS == "windows"
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"plain file name", "tool-v1.2.3-linux-amd64.tar.gz", false},
		{"name with spaces", "release notes.txt", false},
		{"empty", "", true},
		{"path traversal", "../../.bashrc", true},
		{"absolute path", "/etc/cron.d/x", true},
		{"subdirectory", "bin/tool", true},
		{"windows drive-relative", "C:evil", colonRejected},
		{"ntfs alternate data stream", "foo:bar", colonRejected},
		{"backslash separator", `..\..\evil.exe`, true},
		{"bare dot", ".", true},
		{"bare dotdot", "..", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := safeAssetName(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("safeAssetName(%q) = nil, want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("safeAssetName(%q) unexpected error: %v", tc.input, err)
			}
		})
	}
}
