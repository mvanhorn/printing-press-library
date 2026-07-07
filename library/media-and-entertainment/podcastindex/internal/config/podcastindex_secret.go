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

// SaveClientSecret persists the PodcastIndex shared secret to client_secret in
// the config file, mirroring SaveCredential for the key. PodcastindexSecret()
// reads PODCASTINDEX_SECRET first and falls back to this stored value, so a
// saved secret lets non-interactive / --agent runs sign requests without the
// PODCASTINDEX_SECRET environment variable. Lives here (not config.go) so it
// survives generator regen. See .printing-press-patches/find-appearances-and-auth-config.json.
func (c *Config) SaveClientSecret(secret string) error {
	c.ClientSecret = strings.TrimSpace(secret)
	delete(c.envOverrides, "ClientSecret")
	c.updateFileConfigField("ClientSecret")
	return c.save()
}
