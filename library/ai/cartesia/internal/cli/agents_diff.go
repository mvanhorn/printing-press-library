// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// diffResult is the JSON shape returned by `agents diff`.
type diffResult struct {
	AgentID string                   `json:"agent_id"`
	From    string                   `json:"from"`
	To      string                   `json:"to"`
	Added   map[string]any           `json:"added"`
	Removed map[string]any           `json:"removed"`
	Changed map[string]map[string]any `json:"changed"`
}

// newAgentsDiffCmd registers `agents diff <agent-id> --from <id> --to <id>`.
// Loads both deployment JSON blobs and walks them recursively, producing a
// flat key.path keyed diff.
func newAgentsDiffCmd(flags *rootFlags) *cobra.Command {
	var from, to, dbPath string
	cmd := &cobra.Command{
		Use:         "diff [agent-id]",
		Short:       "Diff configuration between two deployments of an agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cartesia-pp-cli agents diff agent_abc --from dep_old --to dep_new --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if from == "" || to == "" {
				return fmt.Errorf("both --from and --to are required")
			}
			agentID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			fromObj, err := loadDeployment(db, from)
			if err != nil {
				return fmt.Errorf("loading --from deployment %q: %w", from, err)
			}
			toObj, err := loadDeployment(db, to)
			if err != nil {
				return fmt.Errorf("loading --to deployment %q: %w", to, err)
			}

			result := diffResult{
				AgentID: agentID,
				From:    from,
				To:      to,
				Added:   map[string]any{},
				Removed: map[string]any{},
				Changed: map[string]map[string]any{},
			}
			diffMaps("", fromObj, toObj, &result)
			return flags.printJSON(cmd, result)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source deployment ID")
	cmd.Flags().StringVar(&to, "to", "", "Target deployment ID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// loadDeployment fetches a deployment record by ID and returns its parsed
// JSON object. Looks first in the typed deployments table; falls back to
// the generic resources catch-all so external sync tooling can write
// deployments under whatever scope label it prefers.
func loadDeployment(db *store.Store, id string) (map[string]any, error) {
	var data string
	err := db.DB().QueryRow(`SELECT data FROM "deployments" WHERE id = ?`, id).Scan(&data)
	if err != nil {
		raw, err2 := db.Get("deployments", id)
		if err2 != nil {
			return nil, fmt.Errorf("deployment not found: %w", err)
		}
		data = string(raw)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, fmt.Errorf("parsing deployment json: %w", err)
	}
	return obj, nil
}

func diffMaps(prefix string, a, b map[string]any, out *diffResult) {
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	for _, k := range ordered {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		av, aok := a[k]
		bv, bok := b[k]
		if aok && !bok {
			out.Removed[path] = av
			continue
		}
		if !aok && bok {
			out.Added[path] = bv
			continue
		}
		amap, aIsMap := av.(map[string]any)
		bmap, bIsMap := bv.(map[string]any)
		if aIsMap && bIsMap {
			diffMaps(path, amap, bmap, out)
			continue
		}
		if !reflect.DeepEqual(av, bv) {
			out.Changed[path] = map[string]any{"from": av, "to": bv}
		}
	}
}
