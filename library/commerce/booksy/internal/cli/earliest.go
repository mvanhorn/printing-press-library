// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: earliest — soonest open slot for a service (requires token).

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelEarliestCmd(flags *rootFlags) *cobra.Command {
	var flagServiceVariant string
	var flagStaffer int64
	var flagWithin string

	cmd := &cobra.Command{
		Use:   "earliest <business_id>",
		Short: "Find the single earliest open slot for a service across a date window in one call",
		Long: "Return the soonest open appointment slot for a service variant.\n" +
			"Get the --service-variant id from `booksy-pp-cli services <business_id>`.\n" +
			"--within accepts 7d/2w/etc (default 14d).\n" +
			"Do NOT use this for a full calendar — use `availability` for every open slot.",
		Example:     "  booksy-pp-cli earliest 297360 --service-variant 20193554 --within 14d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3", "pp:happy-args": "business_id=297360;--service-variant=20193554"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "earliest")
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
			within := 14 * 24 * time.Hour
			if flagWithin != "" {
				d, derr := cliutil.ParseDurationLoose(flagWithin)
				if derr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --within %q: %v", flagWithin, derr))
				}
				within = d
			}
			from := time.Now().Format("2006-01-02")
			to := time.Now().Add(within).Format("2006-01-02")

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
			var foundDate, foundTime string
			for _, d := range days {
				if len(d.Slots) > 0 {
					foundDate = d.Date
					foundTime = d.Slots[0].T
					break
				}
			}
			view := struct {
				BusinessID     string `json:"business_id"`
				ServiceVariant int64  `json:"service_variant_id"`
				Within         string `json:"within"`
				Found          bool   `json:"found"`
				Date           string `json:"date,omitempty"`
				Time           string `json:"time,omitempty"`
				Note           string `json:"note,omitempty"`
			}{BusinessID: args[0], ServiceVariant: variantID, Within: flagWithin, Found: foundDate != "", Date: foundDate, Time: foundTime}
			if foundDate == "" {
				view.Note = fmt.Sprintf("no open slots for variant %d in the next %s; widen --within", variantID, flagWithin)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if werr := printJSONFiltered(cmd.OutOrStdout(), view, flags); werr != nil {
					return werr
				}
			} else if foundDate == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "No open slot for variant %d within %s (from %s).\n", variantID, flagWithin, from)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Earliest: %s at %s (variant %d)\n", foundDate, foundTime, variantID)
			}
			if foundDate == "" {
				// graceful "no result" exit distinct from success
				return &cliError{code: 3, err: fmt.Errorf("no open slot within %s", flagWithin)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagServiceVariant, "service-variant", "", "Service variant id (from `services`)")
	cmd.Flags().Int64Var(&flagStaffer, "staffer", -1, "Staffer id, or -1 for any staffer")
	cmd.Flags().StringVar(&flagWithin, "within", "14d", "Search window, e.g. 7d, 2w, 30d")
	return cmd
}
