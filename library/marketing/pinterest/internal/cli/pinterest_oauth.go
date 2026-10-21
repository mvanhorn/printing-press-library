// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/config"
)

// pinterestClientAuthHeaders returns the HTTP Basic client authentication
// Pinterest requires on /oauth/token and /oauth/token/revoke. Those endpoints
// reject the per-call bearer token the generated client would otherwise send.
func pinterestClientAuthHeaders(flags *rootFlags) (map[string]string, error) {
	id := strings.TrimSpace(os.Getenv("PINTEREST_CLIENT_ID"))
	secret := strings.TrimSpace(os.Getenv("PINTEREST_CLIENT_SECRET"))
	if id == "" || secret == "" {
		if cfg, err := config.Load(flags.configPath); err == nil {
			if id == "" {
				id = strings.TrimSpace(cfg.ClientID)
			}
			if secret == "" {
				secret = strings.TrimSpace(cfg.ClientSecret)
			}
		}
	}
	if id == "" || secret == "" {
		return nil, usageErr(fmt.Errorf("oauth token endpoints need app credentials: set PINTEREST_CLIENT_ID and PINTEREST_CLIENT_SECRET, or run 'pinterest-pp-cli auth login' first"))
	}
	return map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(id+":"+secret))}, nil
}

// pinterestResourceReadConfig applies Pinterest's bookmark cursor to the
// generated export/tail read configs, which page with ?bookmark= like sync.
func pinterestResourceReadConfig(c resourceReadConfig) resourceReadConfig {
	c.cursorParam, c.paginationType, c.nextCursorPath = pinterestBookmarkCursor(c.cursorParam, c.paginationType, c.limitParam, c.nextCursorPath)
	return c
}
