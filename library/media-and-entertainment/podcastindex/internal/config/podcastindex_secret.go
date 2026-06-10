package config

import (
	"os"
	"strings"
)

// PodcastindexSecret returns the PodcastIndex API shared secret used to compute
// the per-request SHA1 Authorization signature. It is read from
// PODCASTINDEX_SECRET, falling back to the stored client_secret.
//
// Lives in its own file so it survives generator regen (see the printing-press
// "hand-edits must be regen-mergeable" rule).
func (c *Config) PodcastindexSecret() string {
	if v := strings.TrimSpace(os.Getenv("PODCASTINDEX_SECRET")); v != "" {
		return v
	}
	return strings.TrimSpace(c.ClientSecret)
}

// PodcastindexAuthKey returns the PodcastIndex API key (the X-Auth-Key value).
func (c *Config) PodcastindexAuthKey() string {
	return strings.TrimSpace(c.AuthHeader())
}
