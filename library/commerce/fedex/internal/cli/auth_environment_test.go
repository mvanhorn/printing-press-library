// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func TestSetTokenPersistsSelectedFedExEnvironment(t *testing.T) {
	t.Setenv("FEDEX_BASE_URL", "")
	path := filepath.Join(t.TempDir(), "config.toml")
	flags := rootFlags{configPath: path}
	cmd := newAuthSetTokenCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"production-token", "--expires-in", "55m", "--env", "prod"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != fedexProdBase || cfg.TokenBaseURL != fedexProdBase || cfg.AccessToken != "production-token" {
		t.Fatalf("token environment not persisted: %#v", cfg)
	}
}
