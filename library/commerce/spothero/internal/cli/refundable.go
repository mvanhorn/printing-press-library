// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newNovelRefundableCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "refundable",
		Short: "See refund status across upcoming reservations at once.",
		Long: "Use this command for refund status across upcoming reservations. It only reads; it shows current eligibility, not a deadline.\n" +
			"Do NOT use this command to see the next reservation; use 'next' instead.",
		Example:     "  spothero-pp-cli refundable --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "refundable")
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			list, err := c.ListAllReservations(5)
			if err != nil {
				return mapConsumerErr(err)
			}
			now := time.Now()
			rows := []map[string]any{}
			for _, r := range upcoming(list, now) {
				if start, err := time.Parse(time.RFC3339, r.Starts); err == nil && days > 0 && start.After(now.AddDate(0, 0, days)) {
					continue
				}
				row := map[string]any{"reservation_id": r.ID, "facility": r.FacilityTitle, "starts": r.Starts, "price_cents": r.PriceCents}
				if e, err := c.GetRefundEligibility(r.ID); err != nil {
					row["error"] = err.Error()
				} else {
					row["refund_to_original_payment"] = e.OriginalSource
					row["refund_as_credit"] = e.CreditOnly
				}
				rows = append(rows, row)
				// One call per reservation against an unofficial API: stay gentle.
				time.Sleep(150 * time.Millisecond)
			}
			return printJSONFilteredKeep(cmd.OutOrStdout(), map[string]any{"count": len(rows), "reservations": rows}, flags, refundableCompactKeep...)
		},
	}
	cmd.Flags().IntVar(&days, "days", 0, "Only reservations starting within this many days (0 for all)")
	return cmd
}

// refundableCompactKeep survives --agent compaction: without "error", a failed
// eligibility lookup reads as a row that simply has no refund flags.
var refundableCompactKeep = []string{"error", "refund_to_original_payment", "refund_as_credit"}
