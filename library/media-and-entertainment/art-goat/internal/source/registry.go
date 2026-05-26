// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package source

import (
	"fmt"
	"sort"
)

// Registry holds the configured art-goat sources keyed by name. v1 MVP
// ships AIC + APOD; additional sources register themselves in their
// package init() via Register().
var defaultRegistry = newRegistry()

type registry struct {
	sources map[string]Source
}

func newRegistry() *registry {
	return &registry{sources: make(map[string]Source)}
}

// Register adds a source to the default registry. Called from each
// source package's init().
func Register(s Source) {
	defaultRegistry.sources[s.Name()] = s
}

// All returns every registered source, sorted by name for deterministic
// output across `sync`, `sources`, and `doctor`.
func All() []Source {
	names := make([]string, 0, len(defaultRegistry.sources))
	for name := range defaultRegistry.sources {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Source, 0, len(names))
	for _, n := range names {
		out = append(out, defaultRegistry.sources[n])
	}
	return out
}

// Lookup returns the named source, or an error if no source is registered
// under that slug. The error names the available sources to make typos
// easy to fix.
func Lookup(name string) (Source, error) {
	if s, ok := defaultRegistry.sources[name]; ok {
		return s, nil
	}
	known := make([]string, 0, len(defaultRegistry.sources))
	for n := range defaultRegistry.sources {
		known = append(known, n)
	}
	sort.Strings(known)
	return nil, fmt.Errorf("unknown source %q (known: %v)", name, known)
}
