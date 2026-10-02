// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/yamato"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelAirportsCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "airports"}
	var query string
	var limit, offset int
	cmd = luggageCmd(cmd, "List live airport terminal IDs and Japanese names for an airport quote.", "--query Narita --agent --select results", "live", f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "airports")
		}
		if e := validateDataSourceStrategy(f, cmd.Annotations["pp:data-source"]); e != nil {
			return usageErr(e)
		}
		if cmd.Annotations["pp:data-source"] == "computed" && f.dataSource == "live" {
			return usageErr(fmt.Errorf("parcel uses verified local rules; choose --data-source auto or local"))
		}
		if e := noLuggageArgs(args); e != nil {
			return e
		}
		if e := luggageLimit(limit, offset); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		c := luggageClient(f)
		aa, e := c.Airports(ctx)
		if e != nil {
			return luggageError(e)
		}
		out := []yamato.Airport{}
		for _, a := range aa {
			values := append([]string{a.ID, a.NameJP}, a.Aliases...)
			if yamato.Matches(query, values...) {
				out = append(out, a)
			}
		}
		total := len(out)
		start := min(offset, total)
		end := min(start+limit, total)
		return luggageOutput(cmd, f, c, map[string]any{"items": out[start:end], "total": total, "offset": offset, "limit": limit, "next_offset": end, "has_more": end < total})
	}
	cmd.Flags().StringVar(&query, "query", "", "Literal airport/terminal name, Japanese text, IATA alias or source ID")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum airport records returned; bounded1..50, default10")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip matched records to read the next source list window")
	return cmd
}
