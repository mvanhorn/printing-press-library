// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/sprocket/internal/config"
)

func TestCacheScopeKeyIsolatesAccountsAndClubs(t *testing.T) {
	base := &config.Config{
		Path:          filepath.Join(t.TempDir(), "account-a.toml"),
		BaseURL:       "https://club-a.sprocketsports.com/",
		SprocketToken: "account-a-secret",
	}
	baseKey, err := cacheScopeKey(base)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		cfg  config.Config
	}{
		{name: "separate config", cfg: config.Config{Path: filepath.Join(t.TempDir(), "account-b.toml"), BaseURL: base.BaseURL, SprocketToken: base.SprocketToken}},
		{name: "separate account token", cfg: config.Config{Path: base.Path, BaseURL: base.BaseURL, SprocketToken: "account-b-secret"}},
		{name: "separate club", cfg: config.Config{Path: base.Path, BaseURL: "https://club-b.sprocketsports.com", SprocketToken: base.SprocketToken}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cacheScopeKey(&tc.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got == baseKey {
				t.Fatalf("scope %q was not isolated", got)
			}
		})
	}
	if strings.Contains(baseKey, base.SprocketToken) {
		t.Fatal("cache scope exposed credential text")
	}
}

func TestCacheScopeKeyIsStableAcrossEquivalentBaseURLs(t *testing.T) {
	cfg := &config.Config{Path: filepath.Join(t.TempDir(), "account.toml"), BaseURL: "HTTPS://CLUB.SPROCKETSPORTS.COM/", SprocketToken: "secret"}
	want, err := cacheScopeKey(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.BaseURL = "https://club.sprocketsports.com"
	got, err := cacheScopeKey(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("equivalent URL scopes differ: %q != %q", got, want)
	}
}
