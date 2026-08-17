// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// allowance-burn: granted-vs-consumed allowance utilization per club. Live
// GraphQL fetch of mealClubsAs (allowance settings) + myDeliveries (spend
// grouped by mealClubId). Hand-authored; preserved across generate --force.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const clubsAllowanceQuery = `query { mealClubsAs (roles: ["member","organizer","accountant","assistant"]) { id name copay copayAllowance allowanceType allowanceMealLimit dailyAllowances } }`

const deliveriesSpendByClubQuery = `query { myDeliveries (from: "2000-01-01") { id forDeliveryAt mealClubId userReceipt { subtotal feesTotal due copayAmount } } }`

type clubAllowance struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Copay          bool      `json:"copay"`
	CopayAllowance flexFloat `json:"copayAllowance"`
	AllowanceType  string    `json:"allowanceType"`
	// Forkable returns allowanceMealLimit as a number when a limit is set
	// and as boolean false when there is none, so decode it leniently.
	MealLimit flexFloat `json:"allowanceMealLimit"`
}

// flexFloat decodes a JSON value that may be a number, a boolean (false =>
// 0, true => 0), a numeric string, or null into a float64. Forkable's
// GraphQL API uses `false` as a "no value" sentinel on numeric fields.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "" || s == "null" || s == "false" || s == "true":
		*f = 0
		return nil
	}
	// Strip surrounding quotes for numeric strings.
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil // fail-open: treat unparseable as 0 rather than erroring the whole command
	}
	*f = flexFloat(v)
	return nil
}

type clubSpendDelivery struct {
	ForDeliveryAt string `json:"forDeliveryAt"`
	MealClubID    int64  `json:"mealClubId"`
	UserReceipt   struct {
		Subtotal  float64 `json:"subtotal"`
		FeesTotal float64 `json:"feesTotal"`
		Due       float64 `json:"due"`
		CopayAmt  float64 `json:"copayAmount"`
	} `json:"userReceipt"`
}

type allowanceRow struct {
	ClubID         int64   `json:"club_id"`
	Club           string  `json:"club"`
	AllowanceType  string  `json:"allowance_type,omitempty"`
	MealLimit      float64 `json:"meal_limit,omitempty"`
	Deliveries     int     `json:"deliveries"`
	Consumed       float64 `json:"consumed_subtotal"`
	MealLimitTotal float64 `json:"granted_estimate,omitempty"`
	UtilizationPct float64 `json:"utilization_pct,omitempty"`
}

type allowanceBurnView struct {
	Clubs []allowanceRow `json:"clubs"`
	Count int            `json:"count"`
	Since string         `json:"since,omitempty"`
}

func newNovelAllowanceBurnCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagBy string

	cmd := &cobra.Command{
		Use:         "allowance-burn",
		Short:       "Show granted-vs-consumed allowance utilization per club, including multi-club comparison.",
		Long:        "For each meal club you belong to or manage, compares the per-meal allowance limit against the meal spend actually consumed over the --since window. Use this for allowance utilization; for a raw time series of spend, use 'spend-trend'.",
		Example:     "  forkable-pp-cli allowance-burn --by club --csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "compute allowance utilization per club")
				return nil
			}
			if flagBy != "club" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be 'club'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// 1) clubs (allowance settings)
			clubData, err := fetchGraphQL(ctx, flags, clubsAllowanceQuery)
			if err != nil {
				return err
			}
			var clubWrap struct {
				MealClubsAs []clubAllowance `json:"mealClubsAs"`
			}
			if err := json.Unmarshal(clubData, &clubWrap); err != nil {
				return fmt.Errorf("parsing clubs: %w", err)
			}
			clubByID := map[int64]clubAllowance{}
			for _, c := range clubWrap.MealClubsAs {
				clubByID[c.ID] = c
			}

			// 2) deliveries (spend grouped by mealClubId)
			delData, err := fetchGraphQL(ctx, flags, deliveriesSpendByClubQuery)
			if err != nil {
				return err
			}
			var delWrap struct {
				MyDeliveries []clubSpendDelivery `json:"myDeliveries"`
			}
			if err := json.Unmarshal(delData, &delWrap); err != nil {
				return fmt.Errorf("parsing deliveries: %w", err)
			}

			cutoff := sinceCutoffISO(flagSince)
			type acc struct {
				deliveries int
				consumed   float64
			}
			byClub := map[int64]*acc{}
			for _, d := range delWrap.MyDeliveries {
				if !dateOnOrAfter(d.ForDeliveryAt, cutoff) {
					continue
				}
				a := byClub[d.MealClubID]
				if a == nil {
					a = &acc{}
					byClub[d.MealClubID] = a
				}
				a.deliveries++
				a.consumed += d.UserReceipt.Subtotal
			}

			rows := make([]allowanceRow, 0, len(byClub))
			for cid, a := range byClub {
				c := clubByID[cid]
				name := c.Name
				if name == "" {
					name = fmt.Sprintf("club %d", cid)
				}
				mealLimit := float64(c.MealLimit)
				row := allowanceRow{
					ClubID:        cid,
					Club:          name,
					AllowanceType: c.AllowanceType,
					MealLimit:     mealLimit,
					Deliveries:    a.deliveries,
					Consumed:      a.consumed,
				}
				if mealLimit > 0 {
					row.MealLimitTotal = mealLimit * float64(a.deliveries)
					if row.MealLimitTotal > 0 {
						row.UtilizationPct = 100 * a.consumed / row.MealLimitTotal
					}
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Consumed > rows[j].Consumed })
			view := allowanceBurnView{Clubs: rows, Count: len(rows), Since: cutoff}
			if flags.asJSON || flags.agent || flags.csv || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No club spend found for the given window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %8s %12s %12s %8s\n", "club", "n", "consumed", "granted~", "util%")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %8d %12.2f %12.2f %7.1f%%\n", r.Club, r.Deliveries, r.Consumed, r.MealLimitTotal, r.UtilizationPct)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include deliveries on or after this window (e.g. 3mo)")
	cmd.Flags().StringVar(&flagBy, "by", "club", "Grouping dimension (only 'club' supported)")
	return cmd
}
