// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Envelope is the {meta, results} shape every query command returns. Matches
// the sec-edgar convention so airframe composes cleanly with other CLIs.
type Envelope struct {
	Meta    Meta `json:"meta"`
	Results any  `json:"results,omitempty"`
}

// Meta carries provenance for the result set: where the data came from,
// when the store was last synced, and the originating query parameters.
type Meta struct {
	Source    string         `json:"source"`              // "local" — airframe has no live mode in v1
	SyncedAt  string         `json:"synced_at,omitempty"` // sync_meta.last_synced_at for the relevant source(s)
	DBPath    string         `json:"db_path,omitempty"`
	Query     map[string]any `json:"query,omitempty"`
	RowCount  *int           `json:"row_count,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
}

// emitEnvelope writes the envelope to stdout. When flagJSON is set, it is
// emitted as compact JSON. Otherwise the command's text renderer is used;
// emitEnvelope is JSON-only here.
func emitEnvelope(env Envelope) error {
	out := os.Stdout
	if flagSelect != "" {
		sub, err := applySelect(env, flagSelect)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(sub)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// applySelect resolves a dotted path against the envelope and returns the
// subtree. Example: `results.aircraft.owner_name` returns just that field.
// Paths that miss return a typed error; the caller surfaces it.
func applySelect(env Envelope, sel string) (any, error) {
	// Round-trip through JSON so we walk a uniform map[string]any tree
	// without reflection on concrete struct fields.
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope for --select: %w", err)
	}
	var cur any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return nil, err
	}
	for _, part := range strings.Split(sel, ".") {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("--select: key %q not found", part)
			}
			cur = next
		case []any:
			return nil, fmt.Errorf("--select: cannot index array with key %q (use a sibling array filter command instead)", part)
		default:
			return nil, fmt.Errorf("--select: cannot descend into %q on %T", part, cur)
		}
	}
	return cur, nil
}

// nowRFC3339 returns the current UTC time in RFC3339 — used for meta
// stamping when sync_meta has no row yet.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
