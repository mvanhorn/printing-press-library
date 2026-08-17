// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// watch: poll one resort/date until it becomes bookable. Hand-authored.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

type watchView struct {
	ResortID int    `json:"resort_id"`
	Date     string `json:"date"`
	Status   string `json:"status"`
	Checks   int    `json:"checks"`
	Open     bool   `json:"open"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var maxChecks int
	var once bool

	cmd := &cobra.Command{
		Use:         "watch <resort_id> <YYYY-MM-DD>",
		Short:       "Poll a resort+date and alert the moment a currently-full day frees up.",
		Example:     "  ikon-pp-cli watch 14 2026-01-17 --interval 5m",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would watch resort/date %v\n", args)
				return nil
			}
			if len(args) != 2 {
				return usageErr(fmt.Errorf("resort_id and date are required"))
			}
			resortID, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid resort_id %q", args[0]))
			}
			if _, err := time.Parse(isoDate, args[1]); err != nil {
				return usageErr(fmt.Errorf("invalid date %q (want YYYY-MM-DD)", args[1]))
			}
			if interval <= 0 {
				return usageErr(fmt.Errorf("--interval must be positive"))
			}
			if once {
				maxChecks = 1
			}
			if (flags.noInput || flags.agent) && maxChecks == 0 {
				maxChecks = 1
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			checks := 0
			for {
				checks++
				availability, err := fetchAvailability(ctx, c, resortID)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				open := bookable(accessiblePasses(availability), args[1])
				view := watchView{
					ResortID: resortID,
					Date:     args[1],
					Status:   classifyAvailabilityDate(availability, args[1]),
					Checks:   checks,
					Open:     open,
				}
				if open || maxChecks > 0 && checks >= maxChecks {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				timer := time.NewTimer(interval)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Polling interval")
	cmd.Flags().IntVar(&maxChecks, "max-checks", 0, "Maximum checks before returning (0 watches until open)")
	cmd.Flags().BoolVar(&once, "once", false, "Check once and return")
	return cmd
}
