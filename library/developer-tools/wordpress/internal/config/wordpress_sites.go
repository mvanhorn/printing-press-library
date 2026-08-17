package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/cliutil"
	"github.com/pelletier/go-toml/v2"
)

// Site is one WordPress installation in the local registry. AppPassword is
// deliberately excluded from JSON so no CLI output can leak it.
type Site struct {
	Name              string    `toml:"name" json:"name"`
	BaseURL           string    `toml:"base_url" json:"base_url"`
	SiteURL           string    `toml:"site_url" json:"site_url"`
	DisplayName       string    `toml:"display_name" json:"display_name"`
	User              string    `toml:"user" json:"user,omitempty"`
	AppPassword       string    `toml:"app_password" json:"-"`
	RestRouteFallback bool      `toml:"rest_route_fallback" json:"rest_route_fallback"`
	Namespaces        []string  `toml:"namespaces" json:"namespaces"`
	AuthorizeURL      string    `toml:"authorize_url" json:"authorize_url,omitempty"`
	AddedAt           time.Time `toml:"added_at" json:"added_at"`
}

type WordPressSite = Site

// SiteRegistry is stored in the main config document as active_site plus
// [sites.<name>] tables. document preserves config keys this accessor does not
// understand, including keys added by future generator versions.
type SiteRegistry struct {
	Active string          `toml:"active_site" json:"active_site"`
	Sites  map[string]Site `toml:"sites" json:"sites"`

	path     string
	document map[string]any
}

type WordPressSiteRegistry = SiteRegistry

// LoadSites reads the registry from the same TOML path used by Config.Load.
func LoadSites(configPath string) (*SiteRegistry, error) {
	path, explicit, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	document := make(map[string]any)
	var data []byte
	if explicit {
		data, err = os.ReadFile(filepath.Clean(path)) // #nosec G304 -- user-selected config path.
	} else {
		legacyPath, legacyErr := LegacyConfigPath()
		if legacyErr != nil {
			return nil, legacyErr
		}
		data, _, err = cliutil.ReadFileWithLegacyFallback(path, legacyPath)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading WordPress site registry: %w", err)
	}

	registry := &SiteRegistry{
		Sites:    make(map[string]Site),
		path:     path,
		document: document,
	}
	if len(data) == 0 {
		return registry, nil
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parsing WordPress site registry %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, registry); err != nil {
		return nil, fmt.Errorf("parsing WordPress sites in %s: %w", path, err)
	}
	if registry.Sites == nil {
		registry.Sites = make(map[string]Site)
	}
	registry.path = path
	registry.document = document
	return registry, nil
}

func LoadWordPressSites(configPath string) (*WordPressSiteRegistry, error) {
	return LoadSites(configPath)
}

// Save atomically writes the full config document with owner-only access.
func (r *SiteRegistry) Save() error {
	if r == nil {
		return fmt.Errorf("WordPress site registry is nil")
	}
	if r.path == "" {
		path, _, err := resolveConfigPath("")
		if err != nil {
			return err
		}
		r.path = path
	}
	if r.Sites == nil {
		r.Sites = make(map[string]Site)
	}
	if r.document == nil {
		r.document = make(map[string]any)
	}

	r.document["active_site"] = r.Active
	r.document["sites"] = r.Sites
	data, err := toml.Marshal(r.document)
	if err != nil {
		return fmt.Errorf("marshaling WordPress site registry: %w", err)
	}
	if err := cliutil.AtomicWritePrivateFile(r.path, data, 0o600, 0o700); err != nil {
		return fmt.Errorf("saving WordPress site registry: %w", err)
	}
	return nil
}

func SaveWordPressSites(registry *WordPressSiteRegistry) error {
	return registry.Save()
}

func (r *SiteRegistry) Get(name string) (Site, bool) {
	if r == nil {
		return Site{}, false
	}
	site, ok := r.Sites[name]
	return site, ok
}

func (r *SiteRegistry) ActiveSite() (Site, bool) {
	if r == nil || r.Active == "" {
		return Site{}, false
	}
	return r.Get(r.Active)
}

func (r *SiteRegistry) Current() (Site, bool) {
	return r.ActiveSite()
}

// ApplyActiveWordPressSite overlays the active site's endpoint and Basic-auth
// pair onto a generated Config without adding registry fields to Config.
// Explicit environment credentials retain their normal highest precedence.
func ApplyActiveWordPressSite(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	registry, err := LoadSites(cfg.Path)
	if err != nil {
		return err
	}
	site, ok := registry.ActiveSite()
	if !ok {
		return nil
	}

	if os.Getenv("WORDPRESS_BASE_URL") == "" {
		cfg.BaseURL = site.BaseURL
	}
	// Never silently apply a global credentials-file secret to an active site.
	// Direct environment overrides retain the precedence Config.Load assigned.
	cfg.AuthHeaderVal = ""
	cfg.AccessToken = ""
	cfg.RefreshToken = ""
	cfg.ClientID = ""
	cfg.ClientSecret = ""
	if !cfg.envOverrides["WordpressUser"] {
		cfg.WordpressUser = site.User
	}
	if !cfg.envOverrides["WordpressAppPassword"] {
		cfg.WordpressAppPassword = site.AppPassword
	}
	if cfg.WordpressUser != "" && cfg.WordpressAppPassword != "" && cfg.AuthSource == "" {
		cfg.AuthSource = "site registry"
		cfg.CredentialSource = "site registry"
	}
	return nil
}
