// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noStdin() ([]byte, error) { return nil, errors.New("stdin should not be read") }

// Neither --md nor --from-md set must be a no-op so the caller falls through
// to --content. Regression: the unset pair used to reach the empty-source
// check and fail every plain `--content` call on posts create/update/comment.
func TestResolveUnsetFallsThroughToContent(t *testing.T) {
	var f markdownBodyFlags
	got, err := f.resolve(noStdin)
	if err != nil {
		t.Fatalf("unset flags returned error %v, want nil", err)
	}
	if got != "" {
		t.Fatalf("unset flags returned %q, want empty string", got)
	}
	if f.set() {
		t.Fatal("set() reported true for unset flags")
	}
}

// An explicitly empty --md is still a usage error: the caller asked for
// markdown conversion and supplied nothing to convert.
func TestResolveEmptyMarkdownIsUsageError(t *testing.T) {
	for _, md := range []string{" ", "\n", "\t\n "} {
		f := markdownBodyFlags{md: md}
		if _, err := f.resolve(noStdin); err == nil {
			t.Fatalf("--md=%q returned nil error, want usage error", md)
		}
	}
}

func TestResolveMutuallyExclusive(t *testing.T) {
	f := markdownBodyFlags{md: "x", fromMD: "y"}
	if _, err := f.resolve(noStdin); err == nil {
		t.Fatal("--md with --from-md returned nil error, want usage error")
	}
}

func TestResolveInlineMarkdownConverts(t *testing.T) {
	f := markdownBodyFlags{md: "Hello **bold**"}
	got, err := f.resolve(noStdin)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(got, `"type":"doc"`) || !strings.Contains(got, `"bold"`) {
		t.Fatalf("resolve returned %q, want TipTap doc with a bold mark", got)
	}
}

func TestResolveFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lesson.md")
	if err := os.WriteFile(path, []byte("A paragraph."), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	f := markdownBodyFlags{fromMD: path}
	got, err := f.resolve(noStdin)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(got, "A paragraph.") {
		t.Fatalf("resolve returned %q, want the file body", got)
	}
}

func TestResolveFromMissingFile(t *testing.T) {
	f := markdownBodyFlags{fromMD: filepath.Join(t.TempDir(), "absent.md")}
	if _, err := f.resolve(noStdin); err == nil {
		t.Fatal("missing file returned nil error, want read error")
	}
}
