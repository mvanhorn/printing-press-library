package config

import "time"

// SaveSendfoxToken replaces stored credentials in one save; environment
// overrides still take precedence when a subsequent invocation loads config.
func (c *Config) SaveSendfoxToken(token string) error {
	c.AuthHeaderVal, c.SendfoxApiToken, c.SendfoxBearerAuth = "", "", ""
	for _, field := range []string{"AuthHeaderVal", "SendfoxApiToken", "SendfoxBearerAuth"} {
		delete(c.envOverrides, field)
		c.updateFileConfigField(field)
	}
	return c.SaveTokens("", "", token, "", time.Time{})
}
