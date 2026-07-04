// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built piloted-Chrome solver profiles. Each profile is a dedicated,
// isolated Chrome user-data-dir on its own CDP port so multiple Suno accounts
// can be solved/generated against concurrently. This is intentionally NOT in
// the generated config.go body; only the `Captcha` field is added there.

package config

const (
	// captchaBasePort is the first CDP port handed to a profile. Picked high to
	// avoid colliding with the user's main Chrome.
	captchaBasePort = 9233
	// defaultCaptchaProfile is the CLI's own default profile. It is a dedicated
	// directory (chrome-profiles/default), NOT the user's real Chrome Default.
	defaultCaptchaProfile = "default"
)

// CaptchaConfig is the [captcha] config block.
type CaptchaConfig struct {
	DefaultProfile string                     `toml:"default_profile,omitempty"`
	Profiles       map[string]*CaptchaProfile `toml:"profiles,omitempty"`
}

// CaptchaProfile is one [captcha.profiles.<name>] section.
type CaptchaProfile struct {
	UserDataDir  string `toml:"user_data_dir,omitempty"` // empty => derived path
	CDPPort      int    `toml:"cdp_port"`
	AccountLabel string `toml:"account_label,omitempty"` // optional human note
}

// ResolveCaptchaProfile applies precedence: explicit flag > config
// default_profile > "default". Env precedence is applied by the caller before
// this (it passes the env value as flagValue when set).
func (c *Config) ResolveCaptchaProfile(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if c.Captcha != nil && c.Captcha.DefaultProfile != "" {
		return c.Captcha.DefaultProfile
	}
	return defaultCaptchaProfile
}

// EnsureCaptchaProfile returns the named profile, creating it (with the next
// free CDP port) if absent. It mutates the in-memory config; the caller is
// responsible for persisting via SaveCaptcha.
func (c *Config) EnsureCaptchaProfile(name string) *CaptchaProfile {
	if c.Captcha == nil {
		c.Captcha = &CaptchaConfig{}
	}
	if c.Captcha.Profiles == nil {
		c.Captcha.Profiles = map[string]*CaptchaProfile{}
	}
	if p, ok := c.Captcha.Profiles[name]; ok {
		return p
	}
	p := &CaptchaProfile{CDPPort: c.nextFreeCaptchaPort()}
	c.Captcha.Profiles[name] = p
	return p
}

// nextFreeCaptchaPort returns the lowest port >= captchaBasePort not already
// assigned to a profile.
func (c *Config) nextFreeCaptchaPort() int {
	used := map[int]bool{}
	if c.Captcha != nil {
		for _, p := range c.Captcha.Profiles {
			used[p.CDPPort] = true
		}
	}
	port := captchaBasePort
	for used[port] {
		port++
	}
	return port
}

// SaveCaptcha persists the current config (including the [captcha] block) to
// disk, reusing the existing private save().
func (c *Config) SaveCaptcha() error {
	return c.save()
}
