// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
)

func newWheelogSaveCmd(flags *rootFlags) *cobra.Command {
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "save [spot-id...]", Short: "Save bounded normalized public spot observations to the local shortlist.",
		Example: "  wheelog-pp-cli shortlist save 166345 166344 --agent", Annotations: map[string]string{"mcp:read-only": "true", "mcp:local-write": "true", "pp:data-source": "live", "pp:happy-args": "spot-id=166345", "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "save selected public WheeLog observations locally")
			}
			if len(args) < 1 || len(args) > 5 {
				return usageErr(fmt.Errorf("shortlist save requires 1..5 public spot IDs"))
			}
			maxAge, err := validateWheelogOptions(options)
			if err != nil {
				return err
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			if mode == "local" {
				return usageErr(fmt.Errorf("shortlist save has no local data source; it captures actual source observations"))
			}
			parsed := make([]int64, 0, len(args))
			seen := map[int64]bool{}
			for _, arg := range args {
				id, err := wheelogID(arg)
				if err != nil {
					return err
				}
				if seen[id] {
					return usageErr(fmt.Errorf("duplicate spot ID %d", id))
				}
				seen[id] = true
				parsed = append(parsed, id)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newWheelogClient(flags)
			if err != nil {
				return err
			}
			spots := make([]wheelog.Spot, 0, len(parsed))
			for _, id := range parsed {
				spot, err := fetchWheelogSpot(ctx, c, id)
				if err != nil {
					return wheelogError(err)
				}
				spots = append(spots, spot)
			}
			db, err := openWheelogStore(ctx, options.DB)
			if err != nil {
				return err
			}
			defer db.Close()
			saved, err := db.WheelogList(ctx)
			if err != nil {
				return err
			}
			newCount := 0
			existing := map[int64]bool{}
			for _, item := range saved {
				existing[item.Latest.ID] = true
			}
			for _, spot := range spots {
				if !existing[spot.ID] {
					newCount++
				}
			}
			if len(saved)+newCount > wheelog.MaxSpots {
				return usageErr(fmt.Errorf("shortlist is limited to 50 selected public spots; remove an entry before adding more"))
			}
			for _, spot := range spots {
				if err := db.ObserveWheelog(ctx, spot); err != nil {
					return configErr(err)
				}
			}
			views := make([]wheelogSpotView, 0, len(spots))
			for _, spot := range spots {
				views = append(views, viewWheelog(spot, options.Questions, maxAge))
			}
			return emitWheelog(cmd, flags, map[string]any{"results": views, "saved": len(spots), "scope": "selected_public_spots", "note": "Only allowlisted public-place facts and at most two observations per selected ID are saved."}, "live")
		}}
	addWheelogReadFlags(cmd, &options)
	return cmd
}
