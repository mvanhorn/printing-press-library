// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: find alternative listings that cover a model's missing
// file formats.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// trackedFormats is the format set formats gaps evaluates against, per spec.
var trackedFormats = []string{"stl", "3mf", "step", "gcode"}

func formatsPresent(files []modelFileInfo) []string {
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.Format] = true
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

type formatAlternative struct {
	Result        novelResult `json:"result"`
	HasFormats    []string    `json:"has_formats"`
	CoversMissing []string    `json:"covers_missing"`
}

// pp:data-source computed
func newNovelFormatsGapsCmd(flags *rootFlags) *cobra.Command {
	var limitPerSource int

	cmd := &cobra.Command{
		Use:         "gaps <source>:<id>",
		Short:       "Find alternatives when a model only ships in one file format and you need another.",
		Example:     "  printgoat-pp-cli formats gaps printables:3161 --agent",
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
					"error":     "model not found (delisted or deleted)",
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			have := map[string]bool{}
			for _, f := range seed.Files {
				have[f.Format] = true
			}
			var missing []string
			for _, fmtName := range trackedFormats {
				if !have[fmtName] {
					missing = append(missing, fmtName)
				}
			}

			out := map[string]any{
				"source":          source,
				"model_id":        id,
				"model_key":       modelKey(source, id),
				"name":            seed.Name,
				"has_formats":     formatsPresent(seed.Files),
				"missing_formats": missing,
			}

			if len(missing) == 0 {
				out["message"] = "this model already covers every tracked format (stl, 3mf, step, gcode)"
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if seed.Name == "" {
				out["message"] = "model has no name to use as a search seed; cannot look for alternatives"
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			results, statuses := searchAllSourcesLive(ctx, c, seed.Name, limitPerSource)
			out["sources"] = statuses

			// Exclude the seed model itself, then cap to the top 5 candidates
			// before the (expensive) per-candidate file-listing fetch.
			candidates := make([]novelResult, 0, len(results))
			for _, r := range results {
				if r.Source == source && r.ID == id {
					continue
				}
				candidates = append(candidates, r)
			}
			if len(candidates) > 5 {
				candidates = candidates[:5]
			}

			var alternatives []formatAlternative
			for _, cand := range candidates {
				detail, derr := fetchModelDetail(ctx, c, cand.Source, cand.ID)
				if derr != nil || detail == nil || !detail.Found {
					continue // best-effort: skip candidates we can't inspect
				}
				candHave := map[string]bool{}
				for _, f := range detail.Files {
					candHave[f.Format] = true
				}
				var covers []string
				for _, m := range missing {
					if candHave[m] {
						covers = append(covers, m)
					}
				}
				if len(covers) == 0 {
					continue
				}
				alternatives = append(alternatives, formatAlternative{
					Result:        cand,
					HasFormats:    formatsPresent(detail.Files),
					CoversMissing: covers,
				})
			}
			out["alternatives"] = alternatives
			out["alternatives_checked"] = len(candidates)

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limitPerSource, "limit", 20, "Maximum results to fetch per source before candidate filtering")
	return cmd
}
