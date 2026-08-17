// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// plan: fan out reservation availability across reservable resorts and show
// where the requested date window has openings. Hand-authored.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type planResortView struct {
	ResortID   int      `json:"resort_id"`
	ResortName string   `json:"resort_name"`
	OpenDates  []string `json:"open_dates"`
	OpenCount  int      `json:"open_count"`
	Error      string   `json:"error,omitempty"`
}

type planView struct {
	From    string           `json:"from"`
	To      string           `json:"to"`
	Days    int              `json:"days"`
	Resorts []planResortView `json:"resorts"`
}

func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:   "plan --from YYYY-MM-DD --to YYYY-MM-DD",
		Short: "Across every reservation-required resort, show which have your target dates open.",
		Long: "Fan out across Ikon's reservation-required resorts and report which dates\n" +
			"in the requested window are bookable for at least one pass on your account.",
		Example:     "  ikon-pp-cli plan --from 2026-01-10 --to 2026-01-20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would check reservable Ikon resorts from %s to %s\n", flagFrom, flagTo)
				return nil
			}
			if flagFrom == "" || flagTo == "" {
				return usageErr(fmt.Errorf("--from and --to are required"))
			}
			dates, err := enumerateDates(flagFrom, flagTo)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resorts, err := fetchResorts(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := planView{From: flagFrom, To: flagTo, Days: len(dates)}
			for _, r := range reservableResorts(resorts) {
				item := planResortView{ResortID: r.ID, ResortName: r.Name}
				availability, err := fetchAvailability(ctx, c, r.ID)
				if err != nil {
					item.Error = err.Error()
					view.Resorts = append(view.Resorts, item)
					continue
				}
				windows := accessiblePasses(availability)
				for _, d := range dates {
					if bookable(windows, d) {
						item.OpenDates = append(item.OpenDates, d)
					}
				}
				item.OpenCount = len(item.OpenDates)
				view.Resorts = append(view.Resorts, item)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start date, inclusive (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "End date, inclusive (YYYY-MM-DD)")
	return cmd
}
