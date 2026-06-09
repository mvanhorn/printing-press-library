// Package config manages the multi-profile Amazon CLI configuration.
//
// Layout on disk:
//
//	~/.config/amazon-pp-cli/
//	  config.json                       <- global state (profile list, active profile)
//	  profiles/<name>/
//	    cookies.json                    <- Chrome cookies for this profile
//	    history.db                      <- SQLite history for this profile
//
// Every command takes an optional --profile <name> flag that overrides the
// active profile in config.json for that invocation only. `profiles use <name>`
// persists the active profile.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	appDirName = "amazon-pp-cli"
)

// Profile describes one Amazon account.
type Profile struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	// MarketplaceBaseURL is "https://www.amazon.com" for the US storefront.
	// Other marketplaces (amazon.co.uk, amazon.de) are accepted but only
	// the US flow has been exercised so far.
	MarketplaceBaseURL string `json:"marketplace_base_url,omitempty"`
}

// File is the JSON document persisted to ~/.config/amazon-pp-cli/config.json.
type File struct {
	ActiveProfile string    `json:"active_profile,omitempty"`
	Profiles      []Profile `json:"profiles,omitempty"`
}

// Config is the in-memory view used by every command.
type Config struct {
	File File

	// activeOverride is the profile name from --profile <flag>. When non-empty
	// it shadows File.ActiveProfile without persisting.
	activeOverride string

	// baseDir defaults to ~/.config/amazon-pp-cli but can be overridden via
	// AMAZON_PP_CLI_HOME for tests.
	baseDir string
}

// Load reads config.json (creating it if absent) and returns a Config.
func Load() (*Config, error) {
	base, err := baseDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("creating config dir: %w", err)
	}
	cfg := &Config{baseDir: base}
	path := filepath.Join(base, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		// First run — leave File empty.
		return cfg, nil
	}
	if err := json.Unmarshal(b, &cfg.File); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// Save writes config.json back to disk.
func (c *Config) Save() error {
	path := filepath.Join(c.baseDir, "config.json")
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(c.File, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetActiveOverride applies a --profile <name> flag for this invocation.
// Returns an error if the named profile does not exist.
func (c *Config) SetActiveOverride(name string) error {
	if name == "" {
		return nil
	}
	if _, ok := c.profile(name); !ok {
		return fmt.Errorf("profile %q not found (run `amazon-pp-cli profiles list`)", name)
	}
	c.activeOverride = name
	return nil
}

// ActiveProfile returns the profile in use for this invocation, considering
// any --profile override.
func (c *Config) ActiveProfile() (Profile, error) {
	name := c.activeOverride
	if name == "" {
		name = c.File.ActiveProfile
	}
	if name == "" {
		if len(c.File.Profiles) == 1 {
			return c.File.Profiles[0], nil
		}
		if len(c.File.Profiles) == 0 {
			return Profile{}, errors.New("no profiles configured (run `amazon-pp-cli profiles add <name>`)")
		}
		return Profile{}, errors.New("no active profile; pass --profile <name> or `amazon-pp-cli profiles use <name>`")
	}
	p, ok := c.profile(name)
	if !ok {
		return Profile{}, fmt.Errorf("active profile %q is missing from the profile list", name)
	}
	return p, nil
}

// AddProfile registers a new profile. Returns an error if the name already exists.
func (c *Config) AddProfile(name, label, marketplace string) error {
	if name == "" {
		return errors.New("profile name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\ \t") {
		return fmt.Errorf("profile name %q must not contain slashes or whitespace", name)
	}
	if _, ok := c.profile(name); ok {
		return fmt.Errorf("profile %q already exists", name)
	}
	if marketplace == "" {
		marketplace = "https://www.amazon.com"
	}
	c.File.Profiles = append(c.File.Profiles, Profile{
		Name:               name,
		Label:              label,
		MarketplaceBaseURL: marketplace,
	})
	sort.Slice(c.File.Profiles, func(i, j int) bool {
		return c.File.Profiles[i].Name < c.File.Profiles[j].Name
	})
	if c.File.ActiveProfile == "" {
		c.File.ActiveProfile = name
	}
	if err := os.MkdirAll(c.ProfileDir(name), 0o700); err != nil {
		return fmt.Errorf("creating profile dir: %w", err)
	}
	return c.Save()
}

// SetActive persists name as the active profile.
func (c *Config) SetActive(name string) error {
	if _, ok := c.profile(name); !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	c.File.ActiveProfile = name
	return c.Save()
}

// ListProfiles returns all known profiles, sorted by name.
func (c *Config) ListProfiles() []Profile {
	out := make([]Profile, len(c.File.Profiles))
	copy(out, c.File.Profiles)
	return out
}

func (c *Config) profile(name string) (Profile, bool) {
	for _, p := range c.File.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// BaseDir returns the on-disk config root.
func (c *Config) BaseDir() string { return c.baseDir }

// ProfileDir returns ~/.config/amazon-pp-cli/profiles/<name>/.
func (c *Config) ProfileDir(name string) string {
	return filepath.Join(c.baseDir, "profiles", name)
}

// CookiesPath returns the cookie store path for the named profile.
func (c *Config) CookiesPath(name string) string {
	return filepath.Join(c.ProfileDir(name), "cookies.json")
}

// DBPath returns the SQLite path for the named profile.
func (c *Config) DBPath(name string) string {
	return filepath.Join(c.ProfileDir(name), "history.db")
}

func baseDir() (string, error) {
	if v := os.Getenv("AMAZON_PP_CLI_HOME"); v != "" {
		return v, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDirName), nil
}
