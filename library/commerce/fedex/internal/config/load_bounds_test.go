// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsCachedTokenWithoutOptInOrBoundedExpiry(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing opt in",
			data: "access_token = 'legacy-token'\ntoken_expiry = 2026-09-03T00:00:00Z\n",
		},
		{
			name: "far future expiry",
			data: "access_token = 'unbounded-token'\ntoken_expiry = 2099-01-01T00:00:00Z\ncache_access_token = true\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.AccessToken != "" || !cfg.TokenExpiry.IsZero() {
				t.Fatalf("unsafe cached token survived load: %#v", cfg)
			}
		})
	}
}
