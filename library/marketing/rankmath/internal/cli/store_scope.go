// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/config"
)

func init() {
	config.StoreScopeIdentity = func(c *config.Config) string {
		if c == nil {
			return ""
		}
		site := normalizeSiteHost(c.TemplateVars["site"])
		if site == "" {
			return ""
		}
		return "site:" + site + "\x00" + c.StoreScopeLegacyCredential()
	}
}

func normalizeSiteHost(site string) string {
	site = strings.TrimSpace(site)
	if site == "" {
		return ""
	}
	if !strings.Contains(site, "://") {
		site = "https://" + site
	}
	u, err := url.Parse(site)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(u.Host, "."))
}
