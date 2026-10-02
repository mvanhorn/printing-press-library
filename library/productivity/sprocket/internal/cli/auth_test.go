// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/sprocket/internal/config"
)

func TestAuthSetTokenReadsCredentialFromStdin(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cmd := newAuthSetTokenCmd(&rootFlags{configPath: configPath})
	cmd.SetIn(strings.NewReader("synthetic-test-token\n"))
	cmd.SetOut(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer synthetic-test-token" {
		t.Fatalf("saved auth header = %q, want synthetic token", got)
	}
}

func TestAuthSetTokenRejectsCredentialArgument(t *testing.T) {
	cmd := newAuthSetTokenCmd(&rootFlags{configPath: filepath.Join(t.TempDir(), "config.toml")})
	cmd.SetArgs([]string{"must-not-enter-argv"})
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("set-token accepted a positional credential")
	}
}
