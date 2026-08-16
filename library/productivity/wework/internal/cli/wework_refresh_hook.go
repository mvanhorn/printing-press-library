// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) auto-refresh: before any command's first request,
// if the stored access token is expired and a refresh token is present, mint a
// fresh one via auth0. The generated client reads Config.AuthHeader() per
// request, so refreshing the config here (registered before the first request
// runs) makes the new token take effect immediately.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil"
)

func init() {
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.Config == nil {
			return nil
		}
		// Never hit the network under dry-run or the verifier's mock mode.
		if c.DryRun || cliutil.IsVerifyEnv() {
			return nil
		}
		c.Config.ApplyWeworkAuthBootstrap()
		if _, err := c.Config.RefreshWeworkTokenIfNeeded(nil); err != nil {
			return authErr(fmt.Errorf("refreshing renewable WeWork session before request: %w; run 'wework-pp-cli auth refresh --force' for details or re-import a complete session", err))
		}
		return nil
	})
}
