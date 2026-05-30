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
	"gopkg.in/yaml.v3"
)

type remapManifest struct {
	Connections map[int64]int64 `yaml:"connections,omitempty"`
	Hooks       map[int64]int64 `yaml:"hooks,omitempty"`
	DataStores  map[int64]int64 `yaml:"data_stores,omitempty"`
	Folders     map[int64]int64 `yaml:"folders,omitempty"`
}

func newNovelBlueprintPromoteCmd(flags *rootFlags) *cobra.Command {
	var flagFromTeam string
	var flagToTeam string
	var flagScenario string
	var flagAutoSuggest bool
	var flagMap string
	var flagName string

	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a scenario from one team to another, rewriting connectionId/hookId/dataStoreId/folderId",
		Example: strings.Trim(`
  make-pp-cli blueprint promote --from-team 588013 --to-team 999999 --scenario 3454192 --auto-suggest --dry-run
  make-pp-cli blueprint promote --from-team 588013 --to-team 999999 --scenario 3454192 --map ./remap.yaml --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagFromTeam == "" || flagToTeam == "" || flagScenario == "" {
				return usageErr(fmt.Errorf("--from-team, --to-team, and --scenario are required"))
			}
			fromTeam, err := strconv.ParseInt(flagFromTeam, 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --from-team: %w", err))
			}
			toTeam, err := strconv.ParseInt(flagToTeam, 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --to-team: %w", err))
			}
			scenarioID, err := strconv.ParseInt(flagScenario, 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --scenario: %w", err))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			bp, err := getBlueprint(ctx, c, scenarioID)
			if err != nil {
				return fmt.Errorf("fetch source blueprint: %w", err)
			}

			rm := remapManifest{}
			todos := map[string][]int64{}
			if flagMap != "" {
				raw, err := os.ReadFile(flagMap)
				if err != nil {
					return fmt.Errorf("read --map %q: %w", flagMap, err)
				}
				if err := yaml.Unmarshal(raw, &rm); err != nil {
					return fmt.Errorf("parse --map %q: %w", flagMap, err)
				}
			}
			if flagAutoSuggest {
				suggested, suggestedTodos, err := buildAutoRemap(ctx, c, fromTeam, toTeam, bp)
				if err != nil {
					return err
				}
				mergeRemap(&rm, suggested)
				for k, v := range suggestedTodos {
					todos[k] = append(todos[k], v...)
				}
			}

			rewritten, rewriteSummary := applyRemap(bp, rm)
			result := map[string]any{
				"sourceScenario": scenarioID,
				"fromTeam":       fromTeam,
				"toTeam":         toTeam,
				"remap":          rm,
				"rewrites":       rewriteSummary,
				"todos":          todos,
			}

			if flags.dryRun {
				result["dryRun"] = true
				b, _ := json.Marshal(result)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}

			// POST to /scenarios with the new blueprint in the target team.
			body := map[string]any{
				"blueprint": string(rewritten),
			}
			if flagName != "" {
				body["name"] = flagName
			}
			postPath := "/scenarios"
			params := map[string]string{"teamId": strconv.FormatInt(toTeam, 10)}
			respRaw, status, err := c.PostWithParams(ctx, postPath, params, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("create scenario in target team returned HTTP %d: %s", status, truncate(string(respRaw), 200)))
			}
			var created any
			_ = json.Unmarshal(respRaw, &created)
			result["createdScenario"] = created
			b, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagFromTeam, "from-team", "", "Source team ID")
	cmd.Flags().StringVar(&flagToTeam, "to-team", "", "Target team ID")
	cmd.Flags().StringVar(&flagScenario, "scenario", "", "Source scenario ID to promote")
	cmd.Flags().BoolVar(&flagAutoSuggest, "auto-suggest", false, "Auto-build a remap by name-matching connections, hooks, and data-stores in the target team")
	cmd.Flags().StringVar(&flagMap, "map", "", "YAML file with explicit ID remapping (overrides auto-suggest entries when both are present)")
	cmd.Flags().StringVar(&flagName, "name", "", "Optional name for the cloned scenario in the target team")
	return cmd
}

// buildAutoRemap fetches connections/hooks/data-stores in both teams and
// proposes a remap by name match. Returns the proposed remap and a TODO map
// of source-IDs that couldn't be matched (so the user can fill them in).
func buildAutoRemap(ctx context.Context, c *client.Client, fromTeam, toTeam int64, bp json.RawMessage) (remapManifest, map[string][]int64, error) {
	out := remapManifest{
		Connections: map[int64]int64{},
		Hooks:       map[int64]int64{},
		DataStores:  map[int64]int64{},
		Folders:     map[int64]int64{},
	}
	todos := map[string][]int64{}

	fromConns, err := listConnections(ctx, c, fromTeam)
	if err != nil {
		return out, todos, fmt.Errorf("list source connections: %w", err)
	}
	toConns, err := listConnections(ctx, c, toTeam)
	if err != nil {
		return out, todos, fmt.Errorf("list target connections: %w", err)
	}
	connByName := byName(toConns)
	for _, src := range fromConns {
		sid := int64(asFloat(src["id"]))
		name := stringOf(src["name"])
		if dst, ok := connByName[strings.ToLower(name)]; ok {
			did := int64(asFloat(dst["id"]))
			if did != 0 {
				out.Connections[sid] = did
				continue
			}
		}
		// only flag connections that the blueprint actually references
		if sliceContainsInt64(walkBlueprintConnectionRefs(bp), sid) {
			todos["connections"] = append(todos["connections"], sid)
		}
	}

	fromHooks, err := listHooks(ctx, c, fromTeam)
	if err != nil {
		return out, todos, fmt.Errorf("list source hooks: %w", err)
	}
	toHooks, err := listHooks(ctx, c, toTeam)
	if err != nil {
		return out, todos, fmt.Errorf("list target hooks: %w", err)
	}
	hookByName := byName(toHooks)
	for _, src := range fromHooks {
		sid := int64(asFloat(src["id"]))
		name := stringOf(src["name"])
		if dst, ok := hookByName[strings.ToLower(name)]; ok {
			did := int64(asFloat(dst["id"]))
			if did != 0 {
				out.Hooks[sid] = did
				continue
			}
		}
		if sliceContainsInt64(walkBlueprintWebhookRefs(bp), sid) {
			todos["hooks"] = append(todos["hooks"], sid)
		}
	}

	fromDS, err := listDataStores(ctx, c, fromTeam)
	if err == nil {
		toDS, err := listDataStores(ctx, c, toTeam)
		if err == nil {
			dsByName := byName(toDS)
			for _, src := range fromDS {
				sid := int64(asFloat(src["id"]))
				name := stringOf(src["name"])
				if dst, ok := dsByName[strings.ToLower(name)]; ok {
					did := int64(asFloat(dst["id"]))
					if did != 0 {
						out.DataStores[sid] = did
						continue
					}
				}
				if sliceContainsInt64(walkBlueprintDataStoreRefs(bp), sid) {
					todos["data_stores"] = append(todos["data_stores"], sid)
				}
			}
		}
	}

	return out, todos, nil
}

func byName(items []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, it := range items {
		name := stringOf(it["name"])
		if name == "" {
			continue
		}
		out[strings.ToLower(name)] = it
	}
	return out
}

func mergeRemap(dst *remapManifest, src remapManifest) {
	for k, v := range src.Connections {
		if dst.Connections == nil {
			dst.Connections = map[int64]int64{}
		}
		if _, ok := dst.Connections[k]; !ok {
			dst.Connections[k] = v
		}
	}
	for k, v := range src.Hooks {
		if dst.Hooks == nil {
			dst.Hooks = map[int64]int64{}
		}
		if _, ok := dst.Hooks[k]; !ok {
			dst.Hooks[k] = v
		}
	}
	for k, v := range src.DataStores {
		if dst.DataStores == nil {
			dst.DataStores = map[int64]int64{}
		}
		if _, ok := dst.DataStores[k]; !ok {
			dst.DataStores[k] = v
		}
	}
	for k, v := range src.Folders {
		if dst.Folders == nil {
			dst.Folders = map[int64]int64{}
		}
		if _, ok := dst.Folders[k]; !ok {
			dst.Folders[k] = v
		}
	}
}

// applyRemap rewrites in-place: walks the blueprint flow, swapping
// connectionId/hookId/dataStoreId references per the remap manifest.
// Returns the rewritten blueprint JSON plus a summary of substitutions made.
func applyRemap(bp json.RawMessage, rm remapManifest) ([]byte, map[string]int) {
	summary := map[string]int{}
	var top map[string]any
	_ = json.Unmarshal(bp, &top)
	root := top
	if inner, ok := top["blueprint"].(map[string]any); ok {
		root = inner
	}
	walk := func(m map[string]any) {
		params, _ := m["parameters"].(map[string]any)
		moduleName, _ := m["module"].(string)
		// Connection refs
		if v, ok := params["__IMTCONN__"]; ok {
			if newID, ok := rm.Connections[numberLike(v)]; ok && newID != 0 {
				params["__IMTCONN__"] = newID
				summary["connections"]++
			}
		}
		// Hook refs
		if strings.HasPrefix(moduleName, "gateway:") {
			if v, ok := params["hook"]; ok {
				if newID, ok := rm.Hooks[numberLike(v)]; ok && newID != 0 {
					params["hook"] = newID
					summary["hooks"]++
				}
			}
		}
		// Data store refs
		for k, v := range params {
			if strings.EqualFold(k, "datastore") || strings.HasSuffix(k, "DataStoreId") {
				if newID, ok := rm.DataStores[numberLike(v)]; ok && newID != 0 {
					params[k] = newID
					summary["data_stores"]++
				}
			}
		}
	}
	visitFlowMapsInPlace(root["flow"], walk)
	out, _ := json.MarshalIndent(top, "", "  ")
	return out, summary
}

// visitFlowMapsInPlace is the mutating variant of visitFlowModules — required
// because applyRemap mutates the maps during the walk.
func visitFlowMapsInPlace(flow any, fn func(map[string]any)) {
	arr, ok := flow.([]any)
	if !ok {
		return
	}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn(m)
		if routes, ok := m["routes"].([]any); ok {
			for _, r := range routes {
				rm, ok := r.(map[string]any)
				if !ok {
					continue
				}
				visitFlowMapsInPlace(rm["flow"], fn)
			}
		}
		visitFlowMapsInPlace(m["flow"], fn)
	}
}

func sliceContainsInt64(s []int64, v int64) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
