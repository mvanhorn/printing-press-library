package insurance

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// embeddedProviders is the seed registry compiled into the binary. The on-disk
// providers.json (next to this file) is the editable source of truth; users can
// also override it at runtime without recompiling (see LoadRegistry).
//
//go:embed providers.json
var embeddedProviders []byte

// EmbeddedRegistry returns the registry baked into the binary. It never fails
// for a correctly-built binary (the embed is validated by the tests).
func EmbeddedRegistry() (Registry, error) {
	reg, err := parseRegistry(embeddedProviders)
	if err != nil {
		return Registry{}, fmt.Errorf("embedded registry is invalid: %w", err)
	}
	reg.Source = "embedded"
	return reg, nil
}

// LoadRegistry resolves the provider registry from the first available source,
// so the registry is trivially editable without recompiling:
//
//  1. an explicit path (the --providers flag), if non-empty
//  2. $INSURANCE_FINDER_PROVIDERS, if set
//  3. ./providers.json in the working directory, if present
//  4. the embedded seed registry (always available)
//
// The chosen source is recorded in Registry.Source.
func LoadRegistry(explicitPath string) (Registry, error) {
	if explicitPath != "" {
		return loadRegistryFile(explicitPath)
	}
	if env := os.Getenv("INSURANCE_FINDER_PROVIDERS"); env != "" {
		return loadRegistryFile(env)
	}
	if _, err := os.Stat("providers.json"); err == nil {
		return loadRegistryFile("providers.json")
	}
	return EmbeddedRegistry()
}

func loadRegistryFile(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, fmt.Errorf("read registry %q: %w", path, err)
	}
	reg, err := parseRegistry(data)
	if err != nil {
		return Registry{}, fmt.Errorf("parse registry %q: %w", path, err)
	}
	reg.Source = path
	return reg, nil
}

// parseRegistry decodes and validates a registry document.
func parseRegistry(data []byte) (Registry, error) {
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return Registry{}, err
	}
	if len(reg.Providers) == 0 {
		return Registry{}, fmt.Errorf("registry contains no providers")
	}
	seen := map[string]bool{}
	for i, p := range reg.Providers {
		if p.ID == "" {
			return Registry{}, fmt.Errorf("provider %d has no id", i)
		}
		if p.Name == "" {
			return Registry{}, fmt.Errorf("provider %q has no name", p.ID)
		}
		if seen[p.ID] {
			return Registry{}, fmt.Errorf("duplicate provider id %q", p.ID)
		}
		seen[p.ID] = true
	}
	return reg, nil
}

// Get returns the provider with the given id, or false if not found.
func (r Registry) Get(id string) (Provider, bool) {
	for _, p := range r.Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// SortedByName returns the providers sorted by display name (stable, for list output).
func (r Registry) SortedByName() []Provider {
	out := make([]Provider, len(r.Providers))
	copy(out, r.Providers)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// StartURL returns the best URL to begin a quote: the quote_url if set, else url.
func (p Provider) StartURL() string {
	if p.QuoteURL != "" {
		return p.QuoteURL
	}
	return p.URL
}
