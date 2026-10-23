package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var accountNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

func ValidateAccount(name string) error {
	if !accountNamePattern.MatchString(name) || strings.Contains(name, "..") || name == "default-account" || name == "config.toml" {
		return fmt.Errorf("invalid account name: use 1-63 lowercase letters, digits, dots, underscores or hyphens")
	}
	return nil
}

func accountDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "google-search-console-pp-cli"), nil
}

func AccountPath(name string) (string, error) {
	if err := ValidateAccount(name); err != nil {
		return "", err
	}
	dir, err := accountDir()
	if err != nil {
		return "", err
	}
	if name == "default" {
		return filepath.Join(dir, "config.toml"), nil
	}
	return filepath.Join(dir, name, "config.toml"), nil
}

func DefaultAccount() (string, error) {
	dir, err := accountDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "default-account"))
	if os.IsNotExist(err) {
		return "default", nil
	}
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if err := ValidateAccount(name); err != nil {
		return "", err
	}
	return name, nil
}

// ResolveAccount keeps explicit legacy config paths independent of saved defaults.
func ResolveAccount(configPath, name string) (string, string, error) {
	if name == "" {
		name = os.Getenv("GOOGLE_SEARCH_CONSOLE_ACCOUNT")
	}
	if configPath == "" {
		configPath = os.Getenv("GOOGLE_SEARCH_CONSOLE_CONFIG")
	}
	if configPath != "" {
		if name != "" {
			return "", "", fmt.Errorf("--account/GOOGLE_SEARCH_CONSOLE_ACCOUNT cannot be combined with --config/GOOGLE_SEARCH_CONSOLE_CONFIG")
		}
		return configPath, "default", nil
	}
	if name == "" {
		var err error
		name, err = DefaultAccount()
		if err != nil {
			return "", "", err
		}
	}
	p, err := AccountPath(name)
	return p, name, err
}

func ListAccounts() ([]string, error) {
	dir, err := accountDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if e.Name() == "config.toml" && !e.IsDir() {
			names = append(names, "default")
			continue
		}
		if !e.IsDir() || ValidateAccount(e.Name()) != nil || e.Name() == "default" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, e.Name(), "config.toml")); err == nil && info.Mode().IsRegular() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func SetDefaultAccount(name string) error {
	p, err := AccountPath(name)
	if err != nil {
		return err
	}
	if name != "default" {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("account is not saved: %w", err)
		}
	}
	dir, err := accountDir()
	if err != nil {
		return err
	}
	return writePrivate(filepath.Join(dir, "default-account"), []byte(name+"\n"), false)
}

func RemoveAccount(name string) error {
	p, err := AccountPath(name)
	if err != nil {
		return err
	}
	current, err := DefaultAccount()
	if err != nil {
		return err
	}
	if current == name && name != "default" {
		return fmt.Errorf("select another default with 'auth set-default' before removing this account")
	}
	return os.Remove(p)
}

// Atomic replacement also fixes permissions on an existing permissive file.
func writePrivate(p string, data []byte, exclusive bool) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if exclusive {
		return os.Link(f.Name(), p)
	}
	return os.Rename(f.Name(), p)
}

func ImportAccount(name string, data []byte) error {
	p, err := AccountPath(name)
	if err != nil {
		return err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid OAuth config format")
	}
	if cfg.AccessToken == "" {
		return fmt.Errorf("OAuth config requires a saved access token; use auth login to create refreshable credentials")
	}
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		return fmt.Errorf("account already exists or cannot be accessed")
	}
	return writePrivate(p, data, true)
}

type accountContextKey struct{}

func WithAccount(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, accountContextKey{}, name)
}

func DataPath(ctx context.Context) string {
	name, _ := ctx.Value(accountContextKey{}).(string)
	cache, err := os.UserCacheDir()
	if err != nil {
		if name == "" || name == "default" {
			return filepath.Join(os.TempDir(), "google-search-console-pp-cli-store.db")
		}
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "google-search-console-pp-cli")
	if name != "" && name != "default" {
		dir = filepath.Join(dir, "accounts", name)
	}
	return filepath.Join(dir, "store.db")
}
