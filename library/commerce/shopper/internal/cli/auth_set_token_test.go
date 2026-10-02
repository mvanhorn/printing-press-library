// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/config"
)

func TestAuthSetTokenReadsSecretFromStdin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, name := range []string{
		"SHOPPER_CONFIG",
		"SHOPPER_CONFIG_DIR",
		"SHOPPER_DATA_DIR",
		"SHOPPER_HOME",
		"SHOPPER_TOKEN",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
	} {
		t.Setenv(name, "")
	}
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatalf("set home override: %v", err)
	}
	t.Cleanup(restore)

	const token = "stdin-jwt-placeholder"
	cmd := newAuthSetTokenCmd(&rootFlags{asJSON: true})
	var stdout bytes.Buffer
	cmd.SetIn(strings.NewReader(token + "\n"))
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set-token --stdin: %v", err)
	}
	if strings.Contains(stdout.String(), token) {
		t.Fatalf("command output echoed token: %s", stdout.String())
	}

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg.AccessToken != token {
		t.Fatalf("persisted access token = %q, want stdin token", cfg.AccessToken)
	}
}

func TestAuthSetTokenRejectsStdinAndPositionalToken(t *testing.T) {
	cmd := newAuthSetTokenCmd(&rootFlags{})
	cmd.SetIn(strings.NewReader("stdin-token\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--stdin", "argv-token"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutually exclusive input error", err)
	}
}
