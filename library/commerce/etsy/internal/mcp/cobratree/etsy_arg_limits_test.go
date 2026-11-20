// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestMirrorArgLimits(t *testing.T) {
	for _, tc := range []struct {
		name, os string
		argv     []string
		bad      bool
	}{
		{"UnixUnder", "linux", []string{strings.Repeat("a", 122879)}, false},
		{"UnixAt", "linux", []string{"--rules-json=" + strings.Repeat("a", 122880-13)}, false},
		{"UnixOver", "linux", []string{"--rules-json=" + strings.Repeat("a", 122881-13)}, true},
		{"UnixMultiple", "darwin", []string{strings.Repeat("a", 100000), strings.Repeat("a", 100000)}, false},
		{"WindowsUnder", "windows", []string{strings.Repeat("a", 31744-262)}, false},
		{"WindowsAt", "windows", []string{strings.Repeat("a", 31744-261)}, false},
		{"WindowsOver", "windows", []string{strings.Repeat("a", 31744-260)}, true},
		{"WindowsEscaping", "windows", []string{"--rules-json=" + strings.Repeat("\\\"", 8000)}, true},
		{"WindowsMultibyte", "windows", []string{strings.Repeat("é", 10000) + " \""}, false},
		{"UnixMultibyte", "linux", []string{strings.Repeat("é", 61440)}, false},
		{"WindowsSpaces", "windows", []string{strings.Repeat("a", 31744-262), "a"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkMirrorArgLimits(tc.os, tc.argv)
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
			if tc.bad && tc.name == "UnixOver" && !strings.Contains(err.Error(), "argument --rules-json is 122881 bytes") {
				t.Fatal(err)
			}
			if tc.bad && tc.os == "windows" && !strings.Contains(err.Error(), "31744-byte") {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct{ arg, want string }{
		{"", `""`}, {"a", "a"}, {"a b", `"a b"`}, {`a"b`, `a\"b`}, {`a b\`, `"a b\\"`}, {`a\"b`, `a\\\"b`},
	} {
		if got := mirrorWindowsEscape(tc.arg); got != tc.want {
			t.Fatalf("escape %q got %q want %q", tc.arg, got, tc.want)
		}
	}
}

func TestMirrorArgGuardBeforeExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is Unix only")
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper")
	marker := filepath.Join(dir, "spawned")
	t.Setenv("ETSY_ARG_TEST_MARKER", marker)
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf x >> \"$ETSY_ARG_TEST_MARKER\"\nprintf '{}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	handler := shellOutToCLI(func() (string, error) { return helper, nil }, nil, nil, map[string]bool{"rules-json": true}, nil, true, nil)
	call := func(n int) *mcplib.CallToolResult {
		r, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "market_gaps", Arguments: map[string]any{"rules-json": strings.Repeat("a", n-13)}}})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := call(122881)
	if !r.IsError || !strings.Contains(mirrorTestText(t, r), "--rules-json") {
		t.Fatal(r)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("child spawned: %v", err)
	}
	r = call(122880)
	if r.IsError {
		t.Fatal(mirrorTestText(t, r))
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "x" {
		t.Fatalf("spawns %q: %v", data, err)
	}
}
