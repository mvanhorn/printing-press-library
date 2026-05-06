// Package cartstore provides on-disk persistence for the active cart and
// for named templates. Files are stored as TOML under
// ~/.config/dominos-pp-cli/. Consumers should treat ErrNotFound as the
// "no active cart" / "no such template" signal, and surface honest
// messaging rather than raw filesystem errors.
package cartstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ErrNotFound is returned when an active cart or named template is missing.
var ErrNotFound = errors.New("cartstore: not found")

// CartItem is a single line on a cart. Toppings are encoded as
// "code:placement:weight" strings; consumers are responsible for parsing
// when shipping to the API.
type CartItem struct {
	Code     string            `toml:"code"`
	Qty      int               `toml:"qty"`
	Size     string            `toml:"size,omitempty"`
	Toppings []string          `toml:"toppings,omitempty"`
	Options  map[string]string `toml:"options,omitempty"`
}

// Cart is the active cart on disk. CreatedAt is set on cart new and is
// preserved across cart add / cart remove edits.
type Cart struct {
	StoreID   string     `toml:"store_id"`
	Service   string     `toml:"service"`
	Address   string     `toml:"address"`
	Items     []CartItem `toml:"items,omitempty"`
	CreatedAt time.Time  `toml:"created_at"`
}

// Template is a named cart that can be replayed via `template order`.
type Template struct {
	Name      string     `toml:"name"`
	StoreID   string     `toml:"store_id"`
	Service   string     `toml:"service"`
	Address   string     `toml:"address"`
	Items     []CartItem `toml:"items,omitempty"`
	CreatedAt time.Time  `toml:"created_at"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "dominos-pp-cli"), nil
}

func cartPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cart.toml"), nil
}

func templatesDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "templates"), nil
}

// LoadActive returns the active cart, or ErrNotFound if none exists.
func LoadActive() (*Cart, error) {
	p, err := cartPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var c Cart
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing active cart: %w", err)
	}
	return &c, nil
}

// SaveActive overwrites any existing active cart with the supplied one.
func SaveActive(c *Cart) error {
	if c == nil {
		return errors.New("cartstore: nil cart")
	}
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	p, err := cartPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// ClearActive deletes the active cart. Returns nil if no cart exists.
func ClearActive() error {
	p, err := cartPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// validateTemplateName rejects names that would escape the templates
// directory or produce surprising filenames.
func validateTemplateName(name string) error {
	if name == "" {
		return errors.New("template name is required")
	}
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid template name %q: must not contain path separators or start with '.'", name)
	}
	return nil
}

func templatePath(name string) (string, error) {
	if err := validateTemplateName(name); err != nil {
		return "", err
	}
	dir, err := templatesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".toml"), nil
}

// SaveTemplate writes a named template, overwriting any existing one.
func SaveTemplate(name string, t *Template) error {
	if t == nil {
		return errors.New("cartstore: nil template")
	}
	t.Name = name
	dir, err := templatesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(t)
	if err != nil {
		return err
	}
	p, err := templatePath(name)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// LoadTemplate reads a named template, returning ErrNotFound if missing.
func LoadTemplate(name string) (*Template, error) {
	p, err := templatePath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var t Template
	if err := toml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", name, err)
	}
	if t.Name == "" {
		t.Name = name
	}
	return &t, nil
}

// ListTemplates returns all template names in lexical order. Returns
// an empty slice if the templates directory does not exist.
func ListTemplates() ([]string, error) {
	dir, err := templatesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".toml") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".toml"))
	}
	sort.Strings(names)
	return names, nil
}

// DeleteTemplate removes a named template. Returns ErrNotFound if missing.
func DeleteTemplate(name string) error {
	p, err := templatePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
