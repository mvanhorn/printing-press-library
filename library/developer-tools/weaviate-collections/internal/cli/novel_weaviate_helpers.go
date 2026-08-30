// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written novel commands (schema snapshot/diff/export,
// collections lint, tenants audit). Kept in its own file so regen never touches it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/weaviate-collections/internal/store"
)

const schemaSnapshotResourceType = "schema_snapshot"

// openNovelStore opens the local SQLite store used by the novel commands.
func openNovelStore(ctx context.Context, flags *rootFlags) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("weaviate-collections-pp-cli"))
}

// schemaSnapshotRecord is what gets persisted per snapshot label.
type schemaSnapshotRecord struct {
	Label    string           `json:"label"`
	TakenAt  string           `json:"taken_at"`
	Classes  []map[string]any `json:"classes"`
	NumClass int              `json:"num_classes"`
}

func fetchAllClasses(ctx context.Context, flags *rootFlags) ([]map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/schema", nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var wrapped struct {
		Classes []map[string]any `json:"classes"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("parsing /schema response: %w", err)
	}
	return wrapped.Classes, nil
}

func fetchOneClass(ctx context.Context, flags *rootFlags, className string) (map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, replacePathParam("/schema/{className}", "className", className), nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var cls map[string]any
	if err := json.Unmarshal(data, &cls); err != nil {
		return nil, fmt.Errorf("parsing collection config: %w", err)
	}
	return cls, nil
}

func classNameOf(cls map[string]any) string {
	if v, ok := cls["class"].(string); ok {
		return v
	}
	return ""
}

// diffEntry describes one leaf-level difference between two JSON documents.
type diffEntry struct {
	Path   string `json:"path"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
	Kind   string `json:"kind"` // added, removed, changed
}

// diffJSON walks two decoded JSON values and reports leaf-level differences.
// Object keys are compared recursively; arrays are compared by JSON-equality
// as a whole (order-sensitive config arrays like `properties` are rarely
// worth diffing element-by-element for a config-drift check).
func diffJSON(before, after any, path string) []diffEntry {
	var out []diffEntry

	bm, bok := before.(map[string]any)
	am, aok := after.(map[string]any)
	if bok && aok {
		keys := map[string]struct{}{}
		for k := range bm {
			keys[k] = struct{}{}
		}
		for k := range am {
			keys[k] = struct{}{}
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			bv, bhas := bm[k]
			av, ahas := am[k]
			switch {
			case bhas && !ahas:
				out = append(out, diffEntry{Path: childPath, Before: bv, Kind: "removed"})
			case !bhas && ahas:
				out = append(out, diffEntry{Path: childPath, After: av, Kind: "added"})
			default:
				out = append(out, diffJSON(bv, av, childPath)...)
			}
		}
		return out
	}

	bj, _ := json.Marshal(before)
	aj, _ := json.Marshal(after)
	if string(bj) != string(aj) {
		out = append(out, diffEntry{Path: path, Before: before, After: after, Kind: "changed"})
	}
	return out
}
