// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/make/internal/client"

	"github.com/spf13/cobra"
)

func newNovelBlueprintDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagKeepMetadata bool

	cmd := &cobra.Command{
		Use:   "diff <scenarioId>",
		Short: "Diff two blueprint snapshots (file paths or 'current'); strips metadata.expect/restore noise by default",
		Example: strings.Trim(`
  make-pp-cli blueprint diff 3041366 --from ./repo/team-588013/3041366-buzzsprout.blueprint.json --to current --json
  make-pp-cli blueprint diff 3041366 --from ./old.blueprint.json --to ./new.blueprint.json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			scenarioID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("scenarioId must be an integer: %q", args[0]))
			}
			ctx := cmd.Context()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			fromBP, err := loadBlueprintSource(ctx, c, scenarioID, flagFrom)
			if err != nil {
				return fmt.Errorf("loading --from: %w", err)
			}
			toBP, err := loadBlueprintSource(ctx, c, scenarioID, flagTo)
			if err != nil {
				return fmt.Errorf("loading --to: %w", err)
			}

			fromCanon, err := canonicalBlueprintJSON(fromBP, flagKeepMetadata)
			if err != nil {
				return err
			}
			toCanon, err := canonicalBlueprintJSON(toBP, flagKeepMetadata)
			if err != nil {
				return err
			}

			diff := computeBlueprintDiff(fromBP, toBP)
			result := map[string]any{
				"scenarioId":   scenarioID,
				"from":         flagFrom,
				"to":           flagTo,
				"keepMetadata": flagKeepMetadata,
				"identical":    string(fromCanon) == string(toCanon),
				"diff":         diff,
			}
			b, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Source snapshot: a file path, or 'current' to fetch from the live API")
	cmd.Flags().StringVar(&flagTo, "to", "current", "Target snapshot: a file path, or 'current' to fetch from the live API (default)")
	cmd.Flags().BoolVar(&flagKeepMetadata, "keep-metadata", false, "Include metadata.expect/restore/designer in the diff (default strips them)")
	return cmd
}

func loadBlueprintSource(ctx context.Context, c *client.Client, scenarioID int64, src string) (json.RawMessage, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("source is empty")
	}
	if src == "current" {
		return getBlueprint(ctx, c, scenarioID)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// computeBlueprintDiff compares two blueprints at the module level: returns
// added module IDs (in target but not source), removed (in source but not
// target), and changed (same ID but mapper/parameters differ).
func computeBlueprintDiff(from, to json.RawMessage) map[string]any {
	fromMods := indexModules(from)
	toMods := indexModules(to)
	var added, removed, changed []map[string]any
	for id, m := range toMods {
		if _, ok := fromMods[id]; !ok {
			added = append(added, map[string]any{
				"id":     id,
				"module": stringOf(m["module"]),
			})
		}
	}
	for id, m := range fromMods {
		if _, ok := toMods[id]; !ok {
			removed = append(removed, map[string]any{
				"id":     id,
				"module": stringOf(m["module"]),
			})
		}
	}
	for id, sm := range fromMods {
		tm, ok := toMods[id]
		if !ok {
			continue
		}
		smJSON, _ := json.Marshal(sm["parameters"])
		tmJSON, _ := json.Marshal(tm["parameters"])
		smMap, _ := json.Marshal(sm["mapper"])
		tmMap, _ := json.Marshal(tm["mapper"])
		if string(smJSON) != string(tmJSON) || string(smMap) != string(tmMap) {
			changed = append(changed, map[string]any{
				"id":             id,
				"module":         stringOf(sm["module"]),
				"parametersDiff": string(smJSON) != string(tmJSON),
				"mapperDiff":     string(smMap) != string(tmMap),
			})
		}
	}
	return map[string]any{
		"added":   added,
		"removed": removed,
		"changed": changed,
	}
}

func indexModules(bp json.RawMessage) map[int64]map[string]any {
	out := map[int64]map[string]any{}
	visit := func(m map[string]any) {
		id := int64(asFloat(m["id"]))
		if id != 0 {
			out[id] = m
		}
	}
	visitFlowModules(bp, visit)
	return out
}
