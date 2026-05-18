package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const (
	AppName = "instacart"
)

// Profile is a named snapshot of location data for one delivery address.
// PATCH (instacart-address-profiles): introduced to let users save several
// addresses (home, work, second residence) and switch the active one with
// a single command instead of re-running `config set-address` each time.
type Profile struct {
	Name       string  `json:"name"`
	Label      string  `json:"label,omitempty"`
	PostalCode string  `json:"postal_code,omitempty"`
	AddressID  string  `json:"address_id,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	ZoneID     string  `json:"zone_id,omitempty"`
}

type Config struct {
	UserAgent  string  `json:"user_agent"`
	PostalCode string  `json:"postal_code,omitempty"`
	AddressID  string  `json:"address_id,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	// ZoneID is Instacart's user-level delivery zone id. It is not returned
	// by ShopCollectionScoped and not encoded in the inventory token (field
	// [7] of the token is always "0" in practice). The Items GraphQL
	// operation requires it as a non-null variable, so the CLI reads it
	// from config with "38" as a fallback. The value appears to be per-user
	// (derived from postal code), not per-retailer -- "38" worked across
	// multiple retailers at the same address.
	ZoneID          string    `json:"zone_id,omitempty"`
	DefaultRetailer string    `json:"default_retailer,omitempty"`
	BundleSHA       string    `json:"bundle_sha,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`

	// PATCH (instacart-address-profiles): named address profiles + which one
	// is active. The top-level location fields above still drive every
	// GraphQL call; switching profiles is implemented by copying the
	// selected profile's fields into them. When no profiles are defined,
	// behavior is identical to pre-profile config files.
	Profiles      map[string]Profile `json:"profiles,omitempty"`
	ActiveProfile string             `json:"active_profile,omitempty"`
}

// EffectiveZoneID returns the user's configured zone id, defaulting to "38"
// when unset. "38" is observed to work for the original developer's postal code; users in
// other regions should run `instacart config set zone_id <value>` once.
func (c *Config) EffectiveZoneID() string {
	if c.ZoneID == "" {
		return "38"
	}
	return c.ZoneID
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent()
	}
	return &c, nil
}

func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	c.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func defaultConfig() *Config {
	return &Config{UserAgent: defaultUserAgent()}
}

func defaultUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
}

// --- Profile helpers ---------------------------------------------------
// PATCH (instacart-address-profiles): kept in this file rather than a
// separate `profiles.go` so a fresh generator print only has to merge
// one config-shaped file.

// profileNameRE constrains profile names to a forgiving but predictable
// shape: lowercase letters, digits, dot, dash, underscore. Length 1-40.
// This is intentionally narrower than "anything that can be a map key" so
// the same name round-trips cleanly through shells, JSON, and the verifier.
var profileNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)

// ValidProfileName reports whether s is acceptable as a profile key.
func ValidProfileName(s string) bool {
	return profileNameRE.MatchString(s)
}

// ProfileNames returns the names of all stored profiles in sorted order.
// Returns nil when no profiles exist (callers should treat this as "no
// profiles configured", not an error).
func (c *Config) ProfileNames() []string {
	if len(c.Profiles) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// GetProfile returns a copy of the named profile and whether it existed.
// Returning a copy (not a pointer into the map) keeps callers from
// mutating the persisted state by accident.
func (c *Config) GetProfile(name string) (Profile, bool) {
	if c.Profiles == nil {
		return Profile{}, false
	}
	p, ok := c.Profiles[name]
	return p, ok
}

// SetProfile inserts or replaces p in the profile map. It does not save.
func (c *Config) SetProfile(p Profile) error {
	if !ValidProfileName(p.Name) {
		return fmt.Errorf("invalid profile name %q: must match %s", p.Name, profileNameRE.String())
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	c.Profiles[p.Name] = p
	return nil
}

// DeleteProfile removes the named profile. If it was the active profile,
// ActiveProfile is cleared. Does not save.
func (c *Config) DeleteProfile(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(c.Profiles, name)
	if c.ActiveProfile == name {
		c.ActiveProfile = ""
	}
	return nil
}

// ApplyProfile copies the named profile's location fields onto the
// top-level config so existing code paths (gql client, doctor, etc.)
// pick them up unchanged. It does NOT mark the profile active or save —
// that is what UseProfile is for. ApplyProfile is the right hook for a
// per-call `--profile <name>` override.
//
// ZoneID is copied verbatim, including when the new profile's ZoneID is
// empty: leaving the previous value in place would silently route GraphQL
// calls for the new address through the previous address's delivery zone.
// `EffectiveZoneID` already handles the empty case by falling back to "38".
func (c *Config) ApplyProfile(name string) error {
	p, ok := c.GetProfile(name)
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	c.PostalCode = p.PostalCode
	c.AddressID = p.AddressID
	c.Latitude = p.Latitude
	c.Longitude = p.Longitude
	c.ZoneID = p.ZoneID
	return nil
}

// UseProfile applies the named profile and marks it active. Does not save.
func (c *Config) UseProfile(name string) error {
	if err := c.ApplyProfile(name); err != nil {
		return err
	}
	c.ActiveProfile = name
	return nil
}
