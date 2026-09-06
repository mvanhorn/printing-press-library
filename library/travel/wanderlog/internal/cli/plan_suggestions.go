// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:client-call

import (
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
)

func newNovelPlanSuggestionsCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var limit int
	cmd := &cobra.Command{Use: "suggestions", Short: "List saved server suggestions for a section, excluding planned places", Args: cobra.NoArgs,
		Example:     "  wanderlog-pp-cli plan suggestions --target-key naertjcoixqrgrfc --day 1 --limit 5 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > 100 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 100"))
			}
			trip, key, err := readTravelPlan(cmd, flags, opts)
			if err != nil {
				return err
			}
			sec, err := resolveSection(trip, opts.day, opts.sectionIndex, opts.sectionID)
			if err != nil {
				return usageErr(err)
			}
			items := travelSuggestions(trip, sec, limit)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan suggestions", "target_key": key, "section": sec.Report, "source": "saved_server_suggestions", "freshness": "unknown", "suggestions": items}, flags)
		}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum saved suggestions (1-100)")
	return cmd
}

func travelSuggestions(trip map[string]any, sec resolvedSection, limit int) []map[string]any {
	seen := map[string]bool{}
	for _, raw := range sections(trip) {
		s, _ := raw.(map[string]any)
		blocks, _ := s["blocks"].([]any)
		for _, b := range blocks {
			block, _ := b.(map[string]any)
			id := travelStopID(block)
			if id != "" {
				seen[id] = true
			}
		}
	}
	recommendations, _ := planResource(trip, "sectionRecommendations").(map[string]any)
	rows, _ := recommendations[strconv.Itoa(intAny(sec.Raw["id"]))].([]any)
	out := []map[string]any{}
	for i, raw := range rows {
		r, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := stringField(r, "placeId")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		item := map[string]any{"place_id": id, "name": r["name"], "server_position": i + 1}
		for _, k := range []string{"latitude", "longitude"} {
			if v, ok := r[k]; ok {
				item[k] = v
			}
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}
