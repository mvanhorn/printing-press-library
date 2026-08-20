// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: availability — open time slots for a service (requires token).

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAvailabilityCmd(flags *rootFlags) *cobra.Command {
	var flagServiceVariant string
	var flagStaffer int64
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:   "availability <business_id>",
		Short: "List open time slots for a service at a business over a date range, grouped by day",
		Long: "List open appointment slots for a service variant at a business, grouped by day.\n" +
			"Get the --service-variant id from `booksy-pp-cli services <business_id>`.\n" +
			"--from/--to default to today and today+14d.",
		Example:     "  booksy-pp-cli availability 297360 --service-variant 20193554 --from 2026-08-19 --to 2026-08-31",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "business_id=297360;--service-variant=20193554"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "availability")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("business_id is required"))
			}
			variantID, err := strconv.ParseInt(flagServiceVariant, 10, 64)
			if err != nil || variantID == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--service-variant is required (see `booksy-pp-cli services %s`)", args[0]))
			}
			from := flagFrom
			to := flagTo
			if from == "" {
				from = time.Now().Format("2006-01-02")
			}
			if to == "" {
				to = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			days, err := fetchTimeSlots(ctx, c, args[0], variantID, flagStaffer, from, to)
			if err != nil {
				return err
			}
			// Keep only days that actually have slots.
			open := make([]bkDaySlots, 0, len(days))
			total := 0
			for _, d := range days {
				if len(d.Slots) > 0 {
					open = append(open, d)
					total += len(d.Slots)
				}
			}
			view := struct {
				BusinessID     string       `json:"business_id"`
				ServiceVariant int64        `json:"service_variant_id"`
				From           string       `json:"from"`
				To             string       `json:"to"`
				TotalSlots     int          `json:"total_slots"`
				Days           []bkDaySlots `json:"days"`
			}{BusinessID: args[0], ServiceVariant: variantID, From: from, To: to, TotalSlots: total, Days: open}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if total == 0 {
				fmt.Fprintf(out, "No open slots for service variant %d between %s and %s.\n", variantID, from, to)
				return nil
			}
			fmt.Fprintf(out, "%d open slots (variant %d, %s → %s):\n", total, variantID, from, to)
			for _, d := range open {
				times := make([]string, 0, len(d.Slots))
				for _, s := range d.Slots {
					times = append(times, s.T)
				}
				fmt.Fprintf(out, "  %s: %s\n", d.Date, joinTimes(times))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagServiceVariant, "service-variant", "", "Service variant id to check (from `services`)")
	cmd.Flags().Int64Var(&flagStaffer, "staffer", -1, "Staffer id, or -1 for any staffer")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "End date YYYY-MM-DD (default: today+14d)")
	return cmd
}

func joinTimes(times []string) string {
	if len(times) == 0 {
		return "(none)"
	}
	out := times[0]
	for _, t := range times[1:] {
		out += ", " + t
	}
	return out
}
