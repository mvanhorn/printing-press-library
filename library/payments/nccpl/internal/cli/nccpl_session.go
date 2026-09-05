package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/config"
)

// nccplHasSession reports whether a captured NCCPL session is available.
//
// This API has no API key: access requires a Cloudflare clearance cookie plus a
// Laravel session, captured by `auth login --chrome`. Without them every request
// lands on a challenge page, which surfaces as a slow timeout rather than the real
// problem, so live commands check this first and fail fast with exit 4.
func nccplHasSession(configPath string) bool {
	cfg, err := config.Load(configPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(cfg.AuthHeader()) != "" || strings.TrimSpace(cfg.CookieCredential()) != ""
}
