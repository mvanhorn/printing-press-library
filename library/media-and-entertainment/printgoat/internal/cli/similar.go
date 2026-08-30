// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: re-search for alternatives after a bad print, excluding
// the model and designer that failed.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelSimilarCmd(flags *rootFlags) *cobra.Command {
	var limitPerSource int

	cmd := &cobra.Command{
		Use:         "similar <source>:<id>",
		Short:       "Re-search for alternatives after a bad print, automatically excluding the model and designer that failed you.",
		Example:     "  printgoat-pp-cli similar printables:3161 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing required argument <model-key>\nUsage: %s <model-key>", cmd.CommandPath()))
			}
			source, id, perr := parseModelRef(args[0])
			if perr != nil {
				return usageErr(perr)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			seed, ferr := fetchModelDetail(ctx, c, source, id)
			if ferr != nil {
				return classifyAPIError(ferr, flags)
			}
			if !seed.Found {
				out := map[string]any{
					"source":    source,
					"model_id":  id,
					"model_key": modelKey(source, id),
					"error":     "model not found (delisted or deleted); cannot derive a search seed",
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if seed.Name == "" {
				out := map[string]any{
					"source":    source,
					"model_id":  id,
					"model_key": modelKey(source, id),
					"error":     "model has no name to use as a search seed",
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			results, statuses := searchAllSourcesLive(ctx, c, seed.Name, limitPerSource)

			included := make([]novelResult, 0, len(results))
			excluded := make([]map[string]any, 0)
			for _, r := range results {
				sameModel := r.Source == source && r.ID == id
				sameDesigner := seed.Designer != "" && r.Designer != "" && strings.EqualFold(strings.TrimSpace(r.Designer), strings.TrimSpace(seed.Designer))
				switch {
				case sameModel:
					excluded = append(excluded, map[string]any{"result": r, "reason": "same model"})
				case sameDesigner:
					excluded = append(excluded, map[string]any{"result": r, "reason": "same designer as the model that failed"})
				default:
					included = append(included, r)
				}
			}

			out := map[string]any{
				"seed": map[string]any{
					"source":    source,
					"model_id":  id,
					"model_key": modelKey(source, id),
					"name":      seed.Name,
					"designer":  seed.Designer,
				},
				"sources":  statuses,
				"results":  included,
				"excluded": excluded,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limitPerSource, "limit", 20, "Maximum results to fetch per source for the re-search")
	return cmd
}
