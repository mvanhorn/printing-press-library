// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

func TestPlatformArtifactOverridesCannotEscapeSelectedProfile(t *testing.T) {
	session := &platform.Session{
		Profile: &platform.Profile{Name: "tenant-a"}, CLI: "sample-pp-cli",
		Paths: platform.Paths{DataFile: filepath.Join(t.TempDir(), "data.db"), CacheDir: t.TempDir(), StateDir: t.TempDir()},
	}
	command := &cobra.Command{Use: "sync"}
	var dbPath string
	command.Flags().StringVar(&dbPath, "db", "", "fixture")
	if err := command.Flags().Parse([]string{"--db", filepath.Join(t.TempDir(), "wrong.db")}); err != nil {
		t.Fatal(err)
	}
	if err := validatePlatformArtifactOverrides(command, session); err == nil || !strings.Contains(err.Error(), "tenant artifact conflict") {
		t.Fatalf("arbitrary database override error = %v", err)
	}
	t.Setenv("SAMPLE_DB", filepath.Join(t.TempDir(), "ambient.db"))
	if err := validatePlatformArtifactOverrides(&cobra.Command{Use: "sync"}, session); err == nil || !strings.Contains(err.Error(), "SAMPLE_DB") {
		t.Fatalf("ambient database override error = %v", err)
	}
}
