package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL = "https://mcp-bff.gojinko.com"
	EnvAPIKey      = "JINKO_API_KEY"
	EnvBaseURL     = "JINKO_API_BASE"
)

type AuthMethod string

const (
	MethodAPIKey AuthMethod = "api_key"
	MethodOAuth  AuthMethod = "oauth"
)

// ResolvedAuth is the credential the CLI will send to the BFF.
type ResolvedAuth struct {
	Method AuthMethod
	Token  string
}

// ConfigFile mirrors ~/.jinko/config.yaml as written by `jinko auth login`.
//
// We keep the same shape as @gojinko/cli so users can switch between the
// TS and Go CLIs without re-authing.
type ConfigFile struct {
	APIKey string       `yaml:"api_key,omitempty"`
	OAuth  *OAuthConfig `yaml:"oauth,omitempty"`
}

type OAuthConfig struct {
	AccessToken  string `yaml:"access_token"`
	RefreshToken string `yaml:"refresh_token"`
	// ExpiresAt accepts int, float, or YAML .nan (Node's Number.NaN) — the TS
	// CLI emits .nan when device-flow auth hasn't returned an expiry yet.
	// We keep the field as any and surface 0 when the value isn't a real number.
	ExpiresAt any `yaml:"expires_at,omitempty"`
}

// ExpiresAtUnix returns the access_token expiry as Unix seconds, or 0 when
// the stored value is .nan, null, or otherwise non-numeric.
func (o *OAuthConfig) ExpiresAtUnix() int64 {
	switch v := o.ExpiresAt.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		if v == v { // false when NaN
			return int64(v)
		}
	}
	return 0
}

// ConfigPath returns the absolute path to ~/.jinko/config.yaml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".jinko", "config.yaml"), nil
}

// ResolveAuth applies the precedence: flag > env > config file (api_key) > config file (oauth).
// Returns a *AuthError when nothing is configured.
func ResolveAuth(paramToken string) (*ResolvedAuth, error) {
	if explicit := paramToken; explicit != "" {
		return resolved(explicit), nil
	}
	if env := os.Getenv(EnvAPIKey); env != "" {
		return resolved(env), nil
	}

	cfg, err := ReadConfigFile()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, &AuthError{}
	}
	if cfg.APIKey != "" {
		return &ResolvedAuth{Method: MethodAPIKey, Token: cfg.APIKey}, nil
	}
	if cfg.OAuth != nil && cfg.OAuth.AccessToken != "" {
		// OAuth refresh is deferred to v0.2 — the TS CLI handles it. For now
		// we send whatever access_token is on disk; the BFF will 401 if stale.
		return &ResolvedAuth{Method: MethodOAuth, Token: cfg.OAuth.AccessToken}, nil
	}
	return nil, &AuthError{}
}

func resolved(token string) *ResolvedAuth {
	if strings.HasPrefix(token, "jnk_") {
		return &ResolvedAuth{Method: MethodAPIKey, Token: token}
	}
	return &ResolvedAuth{Method: MethodOAuth, Token: token}
}

// ReadConfigFile returns nil, nil when the file is absent (not an error).
func ReadConfigFile() (*ConfigFile, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// WriteConfigFile merges updates into the existing file (api_key + oauth
// can coexist; the resolver picks api_key first). Creates ~/.jinko/ on demand.
func WriteConfigFile(updates ConfigFile) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	existing, err := ReadConfigFile()
	if err != nil {
		return err
	}
	merged := ConfigFile{}
	if existing != nil {
		merged = *existing
	}
	if updates.APIKey != "" {
		merged.APIKey = updates.APIKey
	}
	if updates.OAuth != nil {
		merged.OAuth = updates.OAuth
	}

	out, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ClearAuthConfig removes both api_key and oauth blocks from the file.
func ClearAuthConfig() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := ReadConfigFile()
	if err != nil || cfg == nil {
		return err
	}
	cfg.APIKey = ""
	cfg.OAuth = nil
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// ResolveBaseURL returns JINKO_API_BASE or the prod default.
func ResolveBaseURL() string {
	if v := os.Getenv(EnvBaseURL); v != "" {
		return v
	}
	return DefaultBaseURL
}
