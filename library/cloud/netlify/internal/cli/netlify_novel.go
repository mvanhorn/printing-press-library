// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written cross-site novel commands
// (overview, env-drift, dns-audit, since, deploy-diff, submissions search).
// Kept in a dedicated file so `generate --force` preserves it.

package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/mvanhorn/printing-press-library/library/cloud/netlify/internal/store"

	"github.com/spf13/cobra"
)

// novelDBPath resolves the local SQLite mirror path for this CLI.
func novelDBPath() string { return defaultDBPath("netlify-pp-cli") }

// mirrorMissing reports whether the local mirror has not been synced yet.
func mirrorMissing(dbPath string) bool {
	_, err := os.Stat(dbPath)
	return os.IsNotExist(err)
}

// noMirror emits the standard "run sync first" response. Callers provide the
// type-appropriate empty value so machine output keeps the command's schema.
func noMirror(cmd *cobra.Command, flags *rootFlags, dbPath string, resources string, emptyResult any) error {
	cmd.PrintErrf("no local mirror at %s\nrun: netlify-pp-cli sync --resources %s --db %s\n", dbPath, resources, dbPath)
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), emptyResult, flags)
	}
	return nil
}

func timestampAfter(a, b string) bool {
	ta, oka := parseTimeLoose(a)
	tb, okb := parseTimeLoose(b)
	if oka && okb {
		return ta.After(tb)
	}
	return a > b
}

// openMirror opens the local mirror read-only.
func openMirror(ctx context.Context, dbPath string) (*store.Store, error) {
	return store.OpenReadOnlyContext(ctx, dbPath)
}

// loadTyped loads every stored resource of the given types as decoded maps.
// Unknown/empty types are skipped silently so callers can pass a broad set
// covering flat and hierarchical naming variants.
func loadTyped(st *store.Store, types ...string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, t := range types {
		rows, err := st.List(t, 0)
		if err != nil {
			continue
		}
		for _, raw := range rows {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err == nil {
				out = append(out, m)
			}
		}
	}
	return out
}

// str returns the string value at key, or "" when absent or not a string.
func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// firstStr returns the first non-empty string value across the given keys.
func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := str(m, k); v != "" {
			return v
		}
	}
	return ""
}

// nestedStr walks nested maps by key path and returns the leaf string value.
func nestedStr(m map[string]any, path ...string) string {
	cur := m
	for i, k := range path {
		if cur == nil {
			return ""
		}
		if i == len(path)-1 {
			return str(cur, k)
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}
