// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-written: combines four live SolarEdge endpoints into
// one go/no-go status. generate --force preserves this implementation.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/internal/cliutil"
	"github.com/spf13/cobra"
)

type siteHealthView struct {
	SiteID         string   `json:"site_id"`
	SiteName       string   `json:"site_name,omitempty"`
	Status         string   `json:"status"`
	CurrentPowerW  float64  `json:"current_power_w"`
	TodayEnergyWh  float64  `json:"today_energy_wh"`
	GridStatus     string   `json:"grid_status,omitempty"`
	PVStatus       string   `json:"pv_status,omitempty"`
	StorageStatus  string   `json:"storage_status,omitempty"`
	EquipmentCount int      `json:"equipment_count"`
	Notes          []string `json:"notes,omitempty"`
}

func newNovelSiteHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health <siteId>",
		Short: "See one combined go/no-go status for a site instead of cross-referencing several separate calls.",
		Long: "Use this command to get one combined go/no-go status for a site. " +
			"Do NOT use it for raw live power numbers; use 'site current-power-flow' instead, " +
			"or for raw summary stats use 'site overview'.",
		Example:     "  solaredge-pp-cli site health 1223050 --json",
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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			detailsRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/details", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			overviewRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/overview", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			flowRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/currentPowerFlow", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			invRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/Inventory", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			recordSolarEdgeCalls(ctx, siteID, 4)

			view := buildSiteHealthView(siteID, detailsRaw, overviewRaw, flowRaw, invRaw)
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func buildSiteHealthView(siteID string, detailsRaw, overviewRaw, flowRaw, invRaw json.RawMessage) siteHealthView {
	view := siteHealthView{SiteID: siteID, Status: "unknown"}

	var detailsObj map[string]json.RawMessage
	if json.Unmarshal(applyResponsePath(detailsRaw, "details"), &detailsObj) == nil {
		if name, ok := extractStringField(detailsObj, "name"); ok {
			view.SiteName = name
		}
	}

	var overviewObj map[string]json.RawMessage
	if json.Unmarshal(applyResponsePath(overviewRaw, "overview"), &overviewObj) == nil {
		if cp, ok := overviewObj["currentPower"]; ok {
			var cpObj map[string]json.RawMessage
			if json.Unmarshal(cp, &cpObj) == nil {
				if v, ok := cliutil.ExtractNumber(cpObj, "power"); ok {
					view.CurrentPowerW = v
				}
			}
		}
		if ld, ok := overviewObj["lastDayData"]; ok {
			var ldObj map[string]json.RawMessage
			if json.Unmarshal(ld, &ldObj) == nil {
				if v, ok := cliutil.ExtractNumber(ldObj, "energy"); ok {
					view.TodayEnergyWh = v
				}
			}
		}
	}

	var flowObj map[string]json.RawMessage
	flowParsed := json.Unmarshal(applyResponsePath(flowRaw, "siteCurrentPowerFlow"), &flowObj) == nil && len(flowObj) > 0
	if flowParsed {
		view.GridStatus = elementStatus(flowObj, "GRID")
		view.PVStatus = elementStatus(flowObj, "PV")
		view.StorageStatus = elementStatus(flowObj, "STORAGE")
	} else {
		view.Notes = append(view.Notes, "site does not report a current power flow (empty response from the API)")
	}

	var invObj map[string]json.RawMessage
	if json.Unmarshal(applyResponsePath(invRaw, "Inventory"), &invObj) == nil {
		view.EquipmentCount = countInventoryEquipment(invObj)
	}

	switch {
	case view.PVStatus == "Disabled":
		view.Status = "degraded"
		view.Notes = append(view.Notes, "PV element reports Disabled")
	case view.GridStatus == "Disabled":
		view.Status = "degraded"
		view.Notes = append(view.Notes, "GRID element reports Disabled")
	case view.EquipmentCount == 0:
		view.Status = "unknown"
		view.Notes = append(view.Notes, "no equipment found in inventory")
	case flowParsed:
		view.Status = "healthy"
	}

	return view
}

func elementStatus(flowObj map[string]json.RawMessage, key string) string {
	raw, ok := flowObj[key]
	if !ok {
		return ""
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	s, _ := extractStringField(obj, "status")
	return s
}
