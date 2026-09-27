// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newNovelStayPlanCmd(flags *rootFlags) *cobra.Command {
	var f stayFlags
	var planID, roomID string
	cmd := &cobra.Command{
		Use: "plan <property-id>", Short: "Inspect an exact dated room/plan quote, baths and restrictions",
		Long:        "Fetch one exact public room/plan page under the requested party. Preserve quoted cash price and its basis separately from conditional discounts, points and extra fees. Room bath evidence stays distinct from property amenities; unknown final payable totals are not calculated. One source page, at most one transient retry.",
		Example:     "  jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2",
		Annotations: stayAnnotations("live", "<property-id>=385995;--plan-id=03912759;--room-id=0576806;--check-in=2026-11-10;--adults=2;--max-age=5m"), SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, f.query)
			}
			id, err := stayPropertyID(args)
			if err != nil {
				return stayReportError(cmd, err)
			}
			if err := stayValidatePlanRef(planID, roomID); err != nil {
				return stayReportError(cmd, err)
			}
			q, err := stayValidateQuery(cmd, f.query, false)
			if err != nil {
				return stayReportError(cmd, err)
			}
			if _, err := jalan.ValidatePlanQuery(q); err != nil {
				return stayReportError(cmd, err)
			}
			f.query = q
			return stayCall(cmd, flags, &f, func(ctx context.Context, c stayService) (jalan.Response, error) {
				return c.Plan(ctx, id, planID, roomID, q)
			})
		},
	}
	cmd.Flags().StringVar(&planID, "plan-id", "", "Exact source plan ID from stay offers (eight digits)")
	cmd.Flags().StringVar(&roomID, "room-id", "", "Exact source room ID paired with the plan (seven digits)")
	addStayPartyFlags(cmd, &f)
	addStayCacheFlags(cmd, &f)
	return cmd
}

func stayValidatePlanRef(planID, roomID string) error {
	if len(planID) != 8 || strings.Trim(planID, "0123456789") != "" || len(roomID) != 7 || strings.Trim(roomID, "0123456789") != "" {
		return &jalan.Error{Code: "usage", Message: "--plan-id (eight digits) and --room-id (seven digits) are required", Hint: "Use an exact plan/room pair from stay offers, for example --plan-id 03912759 --room-id 0576806."}
	}
	return nil
}
