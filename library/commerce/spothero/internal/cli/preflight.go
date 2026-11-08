// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
)

type preflightCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

func newNovelPreflightCmd(flags *rootFlags) *cobra.Command {
	var in bookInputs
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Check session, vehicle, card, overlaps and spend cap before you try to book.",
		Long: "Use this command to check whether a chosen garage and window are ready to book. It only reads.\n" +
			"Do NOT use this command to find a garage; use 'best' instead.",
		Example:     "  spothero-pp-cli preflight --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --max-total 40 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--facility-id=2175;--starts=2027-01-15T09:00;--ends=2027-01-15T12:00;--max-total=500"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "preflight")
			}
			if in.facilityID == 0 || in.starts == "" || in.ends == "" {
				return usageErr(fmt.Errorf("--facility-id, --starts and --ends are required"))
			}
			capCents, err := parseDollars(in.maxTotal)
			if err != nil {
				return usageErr(err)
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			var checks []preflightCheck
			add := func(name string, ok bool, detail string) { checks = append(checks, preflightCheck{name, ok, detail}) }

			preview, err := in.quote(c)
			if err != nil {
				add("quote", false, err.Error())
			} else {
				add("quote", preview.Available, preview.Price)
				if capCents > 0 {
					add("max_total", preview.PriceCents <= capCents, fmt.Sprintf("quote %d cents, cap %d cents", preview.PriceCents, capCents))
				}
			}
			me, err := c.GetMe()
			if err != nil {
				add("session", false, err.Error())
			} else {
				add("session", true, "")
				add("phone", me.PhoneNumber != "", "")
				if me, err = c.EnrichCreditCards(me); err != nil {
					add("card", false, err.Error())
				} else {
					add("card", len(me.CreditCards) > 0, fmt.Sprintf("%d saved", len(me.CreditCards)))
				}
				vehicles, verr := c.ListVehicles(me.ID)
				switch {
				case verr != nil:
					add("vehicle", !preview.LicensePlateRequired, verr.Error())
				default:
					add("vehicle", len(vehicles) > 0 || !preview.LicensePlateRequired, fmt.Sprintf("%d saved, plate required: %v", len(vehicles), preview.LicensePlateRequired))
				}
				if list, lerr := c.ListAllReservations(5); lerr != nil {
					add("no_overlap", false, lerr.Error())
				} else {
					n := countOverlaps(list, preview.PeriodsUTC, time.Now())
					add("no_overlap", n == 0, fmt.Sprintf("%d overlapping upcoming reservation(s)", n))
				}
			}
			ready := true
			for _, ch := range checks {
				ready = ready && ch.OK
			}
			return printJSONFilteredKeep(cmd.OutOrStdout(), map[string]any{"ready": ready, "checks": checks, "placed": false}, flags, preflightCompactKeep...)
		},
	}
	cmd.Flags().IntVar(&in.facilityID, "facility-id", 0, "Facility id from find results")
	cmd.Flags().StringVar(&in.starts, "starts", "", "Parking start (local or RFC3339)")
	cmd.Flags().StringVar(&in.ends, "ends", "", "Parking end (local or RFC3339)")
	cmd.Flags().StringVar(&in.citySlug, "city-slug", "", "City slug for naive datetimes (from find output params.city_slug)")
	cmd.Flags().StringVar(&in.rateID, "rate-id", "", "A specific rate id from book preview")
	cmd.Flags().StringVar(&in.maxTotal, "max-total", "", "Refuse if the fresh quote total exceeds this many dollars (for example 30.00)")
	return cmd
}

func countOverlaps(list []consumer.Reservation, periods []consumer.SearchPeriod, now time.Time) int {
	if len(periods) == 0 {
		return 0
	}
	start, err1 := time.Parse(time.RFC3339, periods[0].Starts)
	end, err2 := time.Parse(time.RFC3339, periods[0].Ends)
	if err1 != nil || err2 != nil {
		return 0
	}
	n := 0
	for _, r := range upcoming(list, now) {
		rs, e1 := time.Parse(time.RFC3339, r.Starts)
		re, e2 := time.Parse(time.RFC3339, r.Ends)
		if e1 == nil && e2 == nil && rs.Before(end) && re.After(start) {
			n++
		}
	}
	return n
}

// preflightCompactKeep survives --agent compaction: a failed check is useless to an
// agent without the detail that says what the quote and cap were.
var preflightCompactKeep = []string{"ok", "detail"}
