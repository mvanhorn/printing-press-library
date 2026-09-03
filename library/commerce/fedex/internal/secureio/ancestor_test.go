// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package secureio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptorTraversalRejectsSymlinkAncestorWithoutChangingAncestors(t *testing.T) {
	base := t.TempDir()
	parentInfo, err := os.Stat(filepath.Dir(base))
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	realDir := filepath.Join(base, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	linkDir := filepath.Join(base, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := WriteFileAtomic(filepath.Join(linkDir, "secret.json"), []byte("secret")); err == nil {
		t.Fatal("WriteFileAtomic accepted a symlinked ancestor")
	}
	if _, err := os.Stat(filepath.Join(realDir, "secret.json")); !os.IsNotExist(err) {
		t.Fatalf("write escaped through symlink ancestor: %v", err)
	}
	after, err := os.Stat(filepath.Dir(base))
	if err != nil {
		t.Fatalf("stat parent after: %v", err)
	}
	if after.Mode().Perm() != parentInfo.Mode().Perm() {
		t.Fatalf("ancestor mode changed from %#o to %#o", parentInfo.Mode().Perm(), after.Mode().Perm())
	}
}
