// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

// Zimmo's public data needs no account: when no ZIMMO_TOKEN or stored
// credential is configured, give the generated client the site's
// anonymous web token (minted from user-api, cached on disk for 24h).
func init() {
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.Config == nil || c.Config.AuthHeader() != "" {
			return nil
		}
		if tok := strings.TrimSpace(os.Getenv("ZIMMO_TOKEN")); tok != "" {
			c.Config.AuthHeaderVal = "Bearer " + tok
			c.Config.AuthSource = "env:ZIMMO_TOKEN"
			return nil
		}
		if c.DryRun || (cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv()) {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		tok, err := zimmo.AnonymousToken(ctx, nil)
		if err != nil {
			return fmt.Errorf("minting anonymous Zimmo token: %w", err)
		}
		c.Config.AuthHeaderVal = "Bearer " + tok
		if c.Config.AuthSource == "" {
			c.Config.AuthSource = "anonymous"
		}
		return nil
	})
}
