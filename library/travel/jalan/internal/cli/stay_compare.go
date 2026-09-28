// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newNovelStayCompareCmd(flags *rootFlags) *cobra.Command {
	var f stayFlags
	var dateList, planList string
	cmd := &cobra.Command{
		Use: "compare <property-id>", Short: "Compare up to five dates or exact plans under equal party conditions",
		Long:        "Choose one mode: --dates YYYY-MM-DD,... compares bounded observed offers, or --plans PLAN:ROOM,... with --check-in compares exact pairs. Maximum five alternatives; coverage is not exhaustive and no global cheapest claim is made. Unknown or differently based prices are not treated as zero. Select fields inside each comparison cell with --select check_in,results.property_id,results.plan_id,results.room_id,results.price. Selection keeps each alternative's identity, query, status, pagination and source observations. When all results arrays are empty, nested field names cannot be verified. Failed alternatives are retained in fetch_failures; partial coverage exits 8 with successful results on stdout.",
		Example:     "  jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 3\n  jalan-pp-cli stay compare 385995 --check-in 2026-11-10 --plans 03912759:0576806,03806855:0546600 --adults 2",
		Annotations: stayAnnotations("live", "<property-id>=385995;--dates=2026-11-10,2026-11-11;--adults=2;--limit=2;--max-age=5m"), SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, f.query)
			}
			id, err := stayPropertyID(args)
			if err != nil {
				return stayReportError(cmd, err)
			}
			dates, plans, err := stayCompareAlternatives(dateList, planList, f.query.CheckIn)
			if err != nil {
				return stayReportError(cmd, err)
			}
			q := f.query
			if len(dates) > 0 {
				q.CheckIn = dates[0]
			}
			q, err = stayValidateQuery(cmd, q, false)
			if err != nil {
				return stayReportError(cmd, err)
			}
			if len(dates) > 0 {
				_, err = jalan.ValidateOffersQuery(q)
			} else {
				_, err = jalan.ValidatePlanQuery(q)
			}
			if err != nil {
				return stayReportError(cmd, err)
			}
			f.query = q
			for _, date := range dates {
				dated := q
				dated.CheckIn = date
				if _, err := jalan.ValidateQuery(dated, false); err != nil {
					return stayReportError(cmd, err)
				}
			}
			return stayCall(cmd, flags, &f, func(ctx context.Context, c stayService) (jalan.Response, error) {
				return c.Compare(ctx, id, q, dates, plans)
			})
		},
	}
	cmd.Flags().StringVar(&dateList, "dates", "", "Up to five comma-separated check-in dates; use instead of --check-in/--plans")
	cmd.Flags().StringVar(&planList, "plans", "", "Up to five comma-separated PLAN_ID:ROOM_ID pairs; requires --check-in")
	addStayPartyFlags(cmd, &f)
	addStayOfferFilterFlags(cmd, &f)
	addStayPageFlags(cmd, &f)
	addStayCacheFlags(cmd, &f)
	return cmd
}

func stayCompareAlternatives(dateList, planList, checkIn string) ([]string, []jalan.PlanRef, error) {
	fail := func(message string) ([]string, []jalan.PlanRef, error) {
		return nil, nil, &jalan.Error{Code: "usage", Message: message, Hint: "Use --dates 2026-11-10,2026-11-11 OR --check-in 2026-11-10 --plans 03912759:0576806."}
	}
	if (dateList == "") == (planList == "") {
		return fail("choose exactly one of --dates or --plans")
	}
	if dateList != "" {
		if checkIn != "" {
			return fail("--dates cannot be combined with --check-in")
		}
		dates := strings.Split(dateList, ",")
		if len(dates) > 5 {
			return fail("compare accepts at most five dates")
		}
		seen := map[string]bool{}
		for i, date := range dates {
			date = strings.TrimSpace(date)
			if date == "" || seen[date] {
				return fail("comparison dates must be nonempty and distinct")
			}
			dates[i] = date
			seen[date] = true
		}
		return dates, nil, nil
	}
	if checkIn == "" {
		return fail("--plans requires --check-in")
	}
	pairs := strings.Split(planList, ",")
	if len(pairs) > 5 {
		return fail("compare accepts at most five exact plan/room pairs")
	}
	plans := make([]jalan.PlanRef, 0, len(pairs))
	seen := map[string]bool{}
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.Split(pair, ":")
		if len(parts) != 2 || seen[pair] {
			return fail("plans must be distinct PLAN_ID:ROOM_ID pairs")
		}
		if err := stayValidatePlanRef(parts[0], parts[1]); err != nil {
			return nil, nil, err
		}
		plans = append(plans, jalan.PlanRef{PlanID: parts[0], RoomID: parts[1]})
		seen[pair] = true
	}
	return nil, plans, nil
}
