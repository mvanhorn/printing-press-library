// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/yamato"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCountersCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "counters"}
	var query string
	var limit, offset int
	cmd = luggageCmd(cmd, "Inspect live airport pickup/send/floor evidence and supported counter hours.", "--query Narita --agent", "live", f)
	cmd.Long = "Read the full first-party airport directory. Map links identify listed pickup/send counters. Most hours are image-only and remain explicit unknowns; Narita Terminal2 sending hours are visually transcribed and served only while live image bytes match the verified fingerprint. Business closing hours are not dispatch deadlines. Use the local-office source handoff for other dropoff locations."
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "counters")
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
		rows, total, e := c.Counters(ctx, query, limit, offset)
		if e != nil {
			return luggageError(e)
		}
		return luggageOutput(cmd, f, c, map[string]any{"items": rows, "total": total, "offset": offset, "limit": limit, "next_offset": min(offset+len(rows), total), "has_more": offset+len(rows) < total, "local_office_handoff": yamato.Main + "/ytc/files/redirect/search_office.html", "acceptance": "listed service evidence only; verify parcel, current operating calendar and dispatch deadline"})
	}
	cmd.Flags().StringVar(&query, "query", "", "Literal airport, prefecture, service name or counter map ID")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum counter records returned; bounded1..50, default10")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip matched records for the next bounded result window")
	return cmd
}
