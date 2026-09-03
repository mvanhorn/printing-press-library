// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func setMCPTestAuth(t *testing.T, baseURL string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &config.Config{Path: path, BaseURL: baseURL, CacheAccessToken: true}
	if err := cfg.SaveTokens("", "", "synthetic-test-token", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("save test token: %v", err)
	}
	t.Setenv("FEDEX_CONFIG", path)
	t.Setenv("FEDEX_BASE_URL", baseURL)
}
