// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// Config loading intentionally redacts secrets in CLI output; env vars override config file values.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	EnvConfigPath   = "TIKTOK_SHOP_CONFIG"
	EnvAppKey       = "TIKTOK_SHOP_APP_KEY"
	EnvAppSecret    = "TIKTOK_SHOP_APP_SECRET"
	EnvAccessToken  = "TIKTOK_SHOP_ACCESS_TOKEN"
	EnvRefreshToken = "TIKTOK_SHOP_REFRESH_TOKEN"
	EnvShopID       = "TIKTOK_SHOP_SHOP_ID"
	EnvShopCipher   = "TIKTOK_SHOP_SHOP_CIPHER"
	EnvBaseURL      = "TIKTOK_SHOP_BASE_URL"
	EnvAuthBaseURL  = "TIKTOK_SHOP_AUTH_BASE_URL"
)

var OfficialDocs = []string{
	"https://partner.tiktokshop.com/docv2/page/650b1f2ff1fd3102b93c6d3d",
	"https://partner.tiktokshop.com/docv2/page/678e3a3292b0f40314a92d75",
	"https://partner.tiktokshop.com/docv2/page/678e3a2dbd083702fd17455c",
	"https://partner.tiktokshop.com/docv2/page/678e3a3d4ddec3030b238faf",
	"https://partner.tiktokshop.com/docv2/page/6507ead7b99d5302be949ba9",
	"https://partner.tiktokshop.com/docv2/page/650a69e24a0bb702c067291c",
	"https://partner.tiktokshop.com/docv2/page/650aa8094a0bb702c06df242",
	"https://partner.tiktokshop.com/docv2/page/650aa8ccc16ffe02b8f167a0",
	"https://partner.tiktokshop.com/docv2/page/6503081a56e2bb0289dd6d7d",
	"https://partner.tiktokshop.com/docv2/page/6509d85b4a0bb702c057fdda",
	"https://partner.tiktokshop.com/docv2/page/650a9191c16ffe02b8eec161",
	"https://partner.tiktokshop.com/docv2/page/6503068fc20ad60284b38858",
	"https://partner.tiktokshop.com/docv2/page/650aa418defece02be6e66b6",
	"https://partner.tiktokshop.com/docv2/page/650aa592bace3e02b75db748",
	"https://partner.tiktokshop.com/docv2/page/650aa39fbace3e02b75d8617",
	"https://partner.tiktokshop.com/docv2/page/69c3070c441217049711fdea",
}

type Config struct {
	BaseURL      string    `toml:"base_url"`
	AuthBaseURL  string    `toml:"auth_base_url"`
	AppKey       string    `toml:"app_key"`
	AppSecret    string    `toml:"app_secret"`
	AccessToken  string    `toml:"access_token"`
	RefreshToken string    `toml:"refresh_token"`
	ShopID       string    `toml:"shop_id"`
	ShopCipher   string    `toml:"shop_cipher"`
	TokenExpiry  time.Time `toml:"token_expiry"`
	Path         string    `toml:"-"`
	AuthSource   string    `toml:"-"`
}

func Load(configPath string) (*Config, error) {
	cfg := &Config{}
	path := configPath
	if path == "" {
		path = os.Getenv(EnvConfigPath)
	}
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "tiktok-shop-pp-cli", "config.toml")
	}
	cfg.Path = path

	data, err := os.ReadFile(path)
	if err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	if v := os.Getenv(EnvBaseURL); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv(EnvAuthBaseURL); v != "" {
		cfg.AuthBaseURL = v
	}
	if v := os.Getenv(EnvAppKey); v != "" {
		cfg.AppKey = v
		cfg.AuthSource = "env:" + EnvAppKey
	}
	if v := os.Getenv(EnvAppSecret); v != "" {
		cfg.AppSecret = v
	}
	if v := os.Getenv(EnvAccessToken); v != "" {
		cfg.AccessToken = v
		if cfg.AuthSource == "" {
			cfg.AuthSource = "env:" + EnvAccessToken
		}
	}
	if v := os.Getenv(EnvRefreshToken); v != "" {
		cfg.RefreshToken = v
	}
	if v := os.Getenv(EnvShopID); v != "" {
		cfg.ShopID = v
	}
	if v := os.Getenv(EnvShopCipher); v != "" {
		cfg.ShopCipher = v
	}
	if cfg.AuthSource == "" && (cfg.AppKey != "" || cfg.AccessToken != "") {
		cfg.AuthSource = "config:" + cfg.Path
	}

	return cfg, nil
}

func (c *Config) HasAppCredentials() bool {
	return c.AppKey != "" && c.AppSecret != ""
}

func (c *Config) HasTokenBundle() bool {
	return c.AccessToken != "" && c.RefreshToken != ""
}

func (c *Config) HasShopSelector() bool {
	return c.ShopID != "" || c.ShopCipher != ""
}

func (c *Config) SaveTokens(accessToken, refreshToken string, expiry time.Time) error {
	c.AccessToken = accessToken
	c.RefreshToken = refreshToken
	c.TokenExpiry = expiry
	return c.save()
}

func (c *Config) save() error {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(c.Path, data, 0o600)
}
