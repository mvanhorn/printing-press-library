// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncRejectsPublicationPathContextOverrideOfSubdomain(t *testing.T) {
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	root := RootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--subdomain", "trevinsays",
		"sync",
		"--path-context", "publication=otherpub",
		"--resources", "drafts",
		"--latest-only",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("sync accepted --path-context publication override of --subdomain")
	}
	if !strings.Contains(err.Error(), "cannot override --subdomain") {
		t.Fatalf("error = %q, want override guidance", err)
	}
}

func TestSyncRejectsInvalidPublicationPathContext(t *testing.T) {
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	root := RootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"sync",
		"--path-context", "publication=trevinsays.evil.test",
		"--resources", "drafts",
		"--latest-only",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("sync accepted invalid --path-context publication value")
	}
	if !strings.Contains(err.Error(), "single DNS label") {
		t.Fatalf("error = %q, want single-label guidance", err)
	}
}
