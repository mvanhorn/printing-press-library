// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"strings"
)

func (c *Client) PlatformSession() *platform.Session {
	if c == nil {
		return nil
	}
	return c.platformSession
}

func AccountFingerprint(cfg *config.Config) string {
	if cfg == nil || cfg.AuthHeader() == "" {
		return "unconfigured"
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(cfg.BaseURL) + "\x00" + cfg.AuthHeader()))
	return hex.EncodeToString(sum[:6])
}
