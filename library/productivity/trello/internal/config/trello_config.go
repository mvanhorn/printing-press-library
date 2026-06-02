// Hand-authored Trello-specific config (not generated). Trello authenticates
// with TWO query parameters: `key` (the API key) and `token` (the user token).
// The generated config/client only wired the single `key` credential, so the
// `token` was never sent and every authenticated call returned "invalid token".
//
// This file adds the second credential. It is read from TRELLO_API_TOKEN (env)
// or the `api_token` config-file key, and surfaced via TrelloToken() which the
// client appends to the request query alongside `key`.

package config

import "os"

// trelloTokenOverride caches the env-resolved token so the client can read it
// without re-loading config. Populated by LoadTrelloToken, which Load calls.
var trelloTokenOverride string

// LoadTrelloToken resolves the Trello user token from the environment, falling
// back to the config-file value already unmarshaled into the Config. Call this
// after the base Load has run.
func (c *Config) LoadTrelloToken() {
	if v := os.Getenv("TRELLO_API_TOKEN"); v != "" {
		c.TrelloApiToken = v
	}
	trelloTokenOverride = c.TrelloApiToken
}

// TrelloToken returns the resolved Trello user token, or "" when none is set.
func (c *Config) TrelloToken() string {
	if c.TrelloApiToken != "" {
		return c.TrelloApiToken
	}
	return trelloTokenOverride
}
