// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelTripInspectCmd(flags *rootFlags) *cobra.Command {
	var reference string
	cmd := &cobra.Command{
		Use: "inspect [reference]", Short: "Read one Iko-yo Trip reference’s age, facility, fee and booking facts; missing evidence stays unknown",
		Long: "Inspect one canonical spots/ID or events/ID, or its Iko-yo Trip URL. Returns bounded factual excerpts and source links. Missing amenities remain unknown; published age, dates and capacity do not guarantee admission, operation or seats. Auto tries live first, then saved detail only on a network failure. Use --data-source local for an offline observation. " + trip.ScopeNote,
		Example: strings.Trim(`
  iko-yo-pp-cli trip inspect spots/8220 --agent
  iko-yo-pp-cli trip inspect --ref events/8412 --data-source local --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "reference=spots/8220"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trip inspect")
			}
			if len(args) > 1 || len(args) == 1 && reference != "" {
				return usageErr(fmt.Errorf("provide exactly one Trip reference as a positional or --ref"))
			}
			ref := reference
			if len(args) == 1 {
				ref = args[0]
			}
			kind, id, err := trip.ParseReference(ref)
			if err != nil {
				return usageErr(err)
			}
			if flags.dataSource == "local" && flags.noCache {
				return usageErr(fmt.Errorf("--no-cache conflicts with --data-source local"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			r, err := tripRead(ctx, cmd, flags, trip.New(flags.rateLimit), kind+"/"+id)
			if err != nil {
				return tripError(err)
			}
			r = trip.RefreshStatus(r, tripToday())
			tripSource(cmd, flags, r.DataSource)
			return tripPrintRecord(cmd, flags, r)
		},
	}
	cmd.Flags().StringVar(&reference, "ref", "", "Canonical Trip reference, for example spots/8220 or events/8412")
	return cmd
}
