// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"path/filepath"
	"testing"
)

func TestSetTokenRequiresBoundedExpiry(t *testing.T) {
	flags := rootFlags{configPath: filepath.Join(t.TempDir(), "config.toml")}
	cmd := newAuthSetTokenCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"synthetic-token", "--env", "sandbox"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("set-token accepted a token without an expiry")
	}

	cmd = newAuthSetTokenCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"synthetic-token", "--expires-in", "55m", "--env", "sandbox"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token with bounded expiry: %v", err)
	}
}
