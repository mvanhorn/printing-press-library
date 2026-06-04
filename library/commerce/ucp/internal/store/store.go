package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
)

func validateCartID(id string) error {
	if id == "" {
		return fmt.Errorf("cart id is empty")
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return fmt.Errorf("invalid cart id %q: must not contain path separators or '..'", id)
	}
	return nil
}

func cartsDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("cannot determine config dir: %w", err)
		}
		base = filepath.Join(home, ".ucp-pp-cli")
	} else {
		base = filepath.Join(base, "ucp-pp-cli")
	}
	dir := filepath.Join(base, "carts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create carts dir: %w", err)
	}
	return dir, nil
}

func cartPath(id string) (string, error) {
	dir, err := cartsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".json"), nil
}

// New creates a new Cart with a UUID id, timestamps, and status "incomplete".
func New(merchant string) *ucp.Cart {
	now := time.Now().UTC().Format(time.RFC3339)
	return &ucp.Cart{
		ID:        uuid.New().String(),
		Merchant:  merchant,
		Currency:  "USD",
		LineItems: []ucp.LineItem{},
		Status:    "incomplete",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Save writes a cart to disk.
func Save(cart *ucp.Cart) error {
	if err := validateCartID(cart.ID); err != nil {
		return err
	}
	cart.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	path, err := cartPath(cart.ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cart, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cart: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Load reads a cart from disk by ID.
func Load(id string) (*ucp.Cart, error) {
	if err := validateCartID(id); err != nil {
		return nil, err
	}
	path, err := cartPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cart %q not found", id)
		}
		return nil, fmt.Errorf("read cart: %w", err)
	}
	var cart ucp.Cart
	if err := json.Unmarshal(data, &cart); err != nil {
		return nil, fmt.Errorf("parse cart: %w", err)
	}
	return &cart, nil
}

// List returns all carts from the store.
func List() ([]*ucp.Cart, error) {
	dir, err := cartsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read carts dir: %w", err)
	}
	var carts []*ucp.Cart
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		cart, err := Load(id)
		if err != nil {
			continue
		}
		carts = append(carts, cart)
	}
	return carts, nil
}

// Delete removes a cart file from disk.
func Delete(id string) error {
	if err := validateCartID(id); err != nil {
		return err
	}
	path, err := cartPath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cart %q not found", id)
		}
		return fmt.Errorf("delete cart: %w", err)
	}
	return nil
}
