// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/cliutil/testenv"
)

func TestSaveProfileStoreDoesNotFollowPredictableTempSymlink(t *testing.T) {
	testenv.Isolate(t, cliutil.ConfigDir)
	profilePath, err := profileStorePath()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "unrelated.txt")
	if err := os.WriteFile(target, []byte("preserve me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, profilePath+".tmp"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{"safe": {Name: "safe", Values: map[string]string{"json": "true"}}}}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "preserve me" {
		t.Fatalf("pre-planted temp symlink target changed: error=%v", err)
	}
	data, err := os.ReadFile(profilePath)
	if err != nil || string(data) == "" {
		t.Fatalf("profile store was not saved: error=%v", err)
	}
	info, err := os.Stat(profilePath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("profile file permissions are not private: error=%v", err)
	}
}
