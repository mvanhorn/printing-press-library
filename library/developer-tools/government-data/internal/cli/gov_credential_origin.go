package cli

import (
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/config"
)

// Catalog requests use a fixed federal origin rather than the profile's route.
// Keep even environment-derived credentials bound to the selected route: an
// explicit demo request can still explore the catalog without a foreign key.
func govFederalKey(cfg *config.Config, demo bool) string {
	// CredentialOriginsMatch includes the API path in its identity; this
	// boundary checks the HTTP origin, since catalog endpoints have fixed paths.
	var origin string
	if cfg != nil {
		if u, err := url.Parse(cfg.BaseURL); err == nil && u.User == nil {
			origin = (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
		}
	}
	if cfg != nil && cliutil.CredentialOriginsMatch(origin, "https://api.gsa.gov") {
		if key := cfg.AuthHeader(); key != "" {
			return key
		}
	}
	if demo {
		return "DEMO_KEY"
	}
	return ""
}
