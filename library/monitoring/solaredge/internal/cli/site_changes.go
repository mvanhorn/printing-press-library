// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-written: diffs the current period's energy against
// the immediately prior period of equal length, computed from one
// /site/{siteId}/energy call spanning both windows, plus a current
// equipment-count snapshot from inventory.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/internal/cliutil"
	"github.com/spf13/cobra"
)

type periodSummary struct {
	Start    string  `json:"start"`
	End      string  `json:"end"`
	EnergyWh float64 `json:"energy_wh"`
}

type siteChangesView struct {
	SiteID                string        `json:"site_id"`
	Since                 string        `json:"since"`
	CurrentPeriod         periodSummary `json:"current_period"`
	PriorPeriod           periodSummary `json:"prior_period"`
	EnergyDeltaWh         float64       `json:"energy_delta_wh"`
	EnergyDeltaPercent    float64       `json:"energy_delta_percent,omitempty"`
	CurrentEquipmentCount int           `json:"current_equipment_count"`
	Note                  string        `json:"note,omitempty"`
}

func newNovelSiteChangesCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "changes <siteId>",
		Short: "Get a short digest of the energy delta vs the prior period, plus a current equipment snapshot.",
		Long: "Use this command for a short-term delta digest. " +
			"Do NOT use it for statistical underperformance analysis; use 'site underperformance' instead.",
		Example:     "  solaredge-pp-cli site changes 1223050 --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("siteId is required"))
			}
			siteID := args[0]

			since := flagSince
			if since == "" {
				since = "7d"
			}
			sinceDur, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			windowDays := int(sinceDur.Hours() / 24)
			if windowDays < 1 {
				windowDays = 1
			}
			if windowDays > solarEdgeMaxChangesSinceDays {
				windowDays = solarEdgeMaxChangesSinceDays
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			startDate := now.AddDate(0, 0, -2*windowDays).Format("2006-01-02")
			endDate := now.Format("2006-01-02")

			energyRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/energy", "siteId", siteID), map[string]string{
				"startDate": startDate,
				"endDate":   endDate,
				"timeUnit":  "DAY",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			invRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/Inventory", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			recordSolarEdgeCalls(ctx, siteID, 2)

			view, err := buildSiteChangesView(siteID, since, windowDays, energyRaw, invRaw)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window to compare against the immediately prior period of equal length, e.g. 7d, 14d")
	return cmd
}

func buildSiteChangesView(siteID, since string, windowDays int, energyRaw, invRaw json.RawMessage) (siteChangesView, error) {
	view := siteChangesView{SiteID: siteID, Since: since}

	points, err := parseEnergySeriesValues(energyRaw)
	if err != nil {
		return view, err
	}
	if points == nil {
		view.Note = "no energy values returned for this site"
	} else {
		if len(points) < 2*windowDays {
			view.Note = "not enough history to fill both comparison periods"
		} else {
			// Take only the most recent 2*windowDays points and split them in
			// half, so "prior" is the immediately preceding window of equal
			// length — not "everything before current," which would silently
			// widen the comparison if the API returns more points than
			// requested (inclusive date-range rounding, timezone edge days).
			window := points[len(points)-2*windowDays:]
			prior := window[:windowDays]
			current := window[windowDays:]
			view.PriorPeriod = summarizePeriod(prior)
			view.CurrentPeriod = summarizePeriod(current)
			view.EnergyDeltaWh = view.CurrentPeriod.EnergyWh - view.PriorPeriod.EnergyWh
			if view.PriorPeriod.EnergyWh != 0 {
				view.EnergyDeltaPercent = (view.EnergyDeltaWh / view.PriorPeriod.EnergyWh) * 100
			}
		}
	}

	var invObj map[string]json.RawMessage
	if json.Unmarshal(applyResponsePath(invRaw, "Inventory"), &invObj) == nil {
		view.CurrentEquipmentCount = countInventoryEquipment(invObj)
	}
	equipmentNote := "current_equipment_count is a current snapshot, not a delta — the API exposes no equipment history"
	if view.Note == "" {
		view.Note = equipmentNote
	} else {
		view.Note += "; " + equipmentNote
	}

	return view, nil
}

func summarizePeriod(points []energyDayPoint) periodSummary {
	if len(points) == 0 {
		return periodSummary{}
	}
	sum, _ := sumEnergyPoints(points)
	return periodSummary{Start: points[0].Date, End: points[len(points)-1].Date, EnergyWh: sum}
}
