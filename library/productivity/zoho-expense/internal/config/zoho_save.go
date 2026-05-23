package config

// Save persists the config to disk via the unexported save() helper.
// Hand-authored novel commands (org use, etc.) call this to update
// arbitrary fields without going through the token-specific SaveTokens.
func (c *Config) Save() error {
	return c.save()
}
