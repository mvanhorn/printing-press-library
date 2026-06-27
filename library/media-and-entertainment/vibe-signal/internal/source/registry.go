// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored aggregator source layer (not generator-emitted).

package source

import "sort"

var registry = map[string]Source{}

// Register adds a source to the registry. Call from a source package's init().
func Register(s Source) {
	registry[s.Name()] = s
}

// All returns every registered source, sorted by name.
func All() []Source {
	out := make([]Source, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Lookup returns the named source.
func Lookup(name string) (Source, bool) {
	s, ok := registry[name]
	return s, ok
}
