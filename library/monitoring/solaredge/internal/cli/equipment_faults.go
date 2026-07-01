// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-written: surfaces only non-nominal equipment by
// joining inventory, battery telemetry, and current power flow status.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/internal/cliutil"
	"github.com/spf13/cobra"
)

type equipmentFault struct {
	Category     string `json:"category"`
	Name         string `json:"name,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	Issue        string `json:"issue"`
	Detail       string `json:"detail"`
}

type equipmentFaultsView struct {
	SiteID  string           `json:"site_id"`
	Faults  []equipmentFault `json:"faults"`
	Checked []string         `json:"checked"`
}

func newNovelEquipmentFaultsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "faults <siteId>",
		Short: "See only the inverters, batteries, or system elements in a non-nominal state.",
		Long: "Use this command for a filtered list of equipment in a non-nominal state. " +
			"Do NOT use it for full inventory; use 'equipment inventory' instead. " +
			"Do NOT use it for raw per-serial telemetry; use 'equipment inverter-data' instead.",
		Example:     "  solaredge-pp-cli equipment faults 1223050 --json",
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

			view := equipmentFaultsView{SiteID: siteID, Faults: []equipmentFault{}}
			calls := 0
			// Record via defer, not a single call after the last c.Get: quota
			// calls that already succeeded must still be counted even if a
			// later call in this sequence fails and the command returns early.
			defer func() { recordSolarEdgeCalls(ctx, siteID, calls) }()

			invRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/Inventory", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			calls++
			view.Checked = append(view.Checked, "inventory")

			var invObj map[string]json.RawMessage
			_ = json.Unmarshal(applyResponsePath(invRaw, "Inventory"), &invObj)

			view.Faults = append(view.Faults, checkInverterOptimizers(invObj)...)

			batterySerials := batterySerialNumbers(invObj)
			if len(batterySerials) > 0 {
				now := time.Now().UTC()
				start := now.Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
				end := now.Format("2006-01-02 15:04:05")
				storageRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/storageData", "siteId", siteID), map[string]string{
					"startTime": start,
					"endTime":   end,
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				calls++
				view.Checked = append(view.Checked, "storage-data")
				view.Faults = append(view.Faults, checkBatteryState(storageRaw)...)
			}

			flowRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/currentPowerFlow", "siteId", siteID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			calls++
			view.Checked = append(view.Checked, "current-power-flow")
			view.Faults = append(view.Faults, checkPowerFlowElements(flowRaw)...)

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func checkInverterOptimizers(invObj map[string]json.RawMessage) []equipmentFault {
	var faults []equipmentFault
	raw, ok := invObj["inverters"]
	if !ok {
		return faults
	}
	var inverters []map[string]json.RawMessage
	if json.Unmarshal(raw, &inverters) != nil {
		return faults
	}
	for _, inv := range inverters {
		name, _ := extractStringField(inv, "name")
		sn, _ := extractStringField(inv, "SN")
		if n, ok := cliutil.ExtractInt(inv, "connectedOptimizers"); ok && n == 0 {
			faults = append(faults, equipmentFault{
				Category:     "inverter",
				Name:         name,
				SerialNumber: sn,
				Issue:        "no_optimizers_reporting",
				Detail:       "inventory reports 0 connected optimizers — verify communication",
			})
		}
	}
	return faults
}

func batterySerialNumbers(invObj map[string]json.RawMessage) []string {
	var serials []string
	raw, ok := invObj["batteries"]
	if !ok {
		return serials
	}
	var batteries []map[string]json.RawMessage
	if json.Unmarshal(raw, &batteries) != nil {
		return serials
	}
	for _, b := range batteries {
		if sn, ok := extractStringField(b, "SN"); ok && sn != "" {
			serials = append(serials, sn)
		}
	}
	return serials
}

// checkBatteryState flags batteries whose most recent telemetry reports a
// non-nominal batteryState. Per the vendor's Storage Information docs, the
// field is one of: 0 Invalid, 1 Standby, 2 Thermal Mgmt., 3 Enabled, 4
// Fault. Only 0 and 4 are treated as faults here; 1/2/3 are normal
// operating states a battery cycles through.
func checkBatteryState(storageRaw json.RawMessage) []equipmentFault {
	var faults []equipmentFault
	var storageObj map[string]json.RawMessage
	if json.Unmarshal(applyResponsePath(storageRaw, "storageData"), &storageObj) != nil {
		return faults
	}
	raw, ok := storageObj["batteries"]
	if !ok {
		return faults
	}
	var batteries []map[string]json.RawMessage
	if json.Unmarshal(raw, &batteries) != nil {
		return faults
	}
	for _, b := range batteries {
		name, _ := extractStringField(b, "name")
		sn, _ := extractStringField(b, "serialNumber")
		telRaw, ok := b["telemetries"]
		if !ok {
			continue
		}
		var telemetries []map[string]json.RawMessage
		if json.Unmarshal(telRaw, &telemetries) != nil || len(telemetries) == 0 {
			continue
		}
		latest := latestTelemetryByTimestamp(telemetries)
		state, ok := cliutil.ExtractInt(latest, "batteryState")
		if !ok {
			continue
		}
		switch state {
		case 4:
			faults = append(faults, equipmentFault{Category: "battery", Name: name, SerialNumber: sn, Issue: "fault", Detail: "most recent telemetry reports batteryState=4 (Fault)"})
		case 0:
			faults = append(faults, equipmentFault{Category: "battery", Name: name, SerialNumber: sn, Issue: "invalid", Detail: "most recent telemetry reports batteryState=0 (Invalid)"})
		}
	}
	return faults
}

// latestTelemetryByTimestamp returns the telemetry entry with the
// lexicographically greatest "timeStamp" field (the API's timestamp format,
// yyyy-MM-dd HH:mm:ss, sorts correctly as a plain string). Falls back to the
// last array element if no entry has a parseable timestamp, since the API
// has historically returned telemetries in chronological order.
func latestTelemetryByTimestamp(telemetries []map[string]json.RawMessage) map[string]json.RawMessage {
	var latest map[string]json.RawMessage
	var latestTS string
	for _, t := range telemetries {
		ts, ok := extractStringField(t, "timeStamp")
		if !ok {
			continue
		}
		if latest == nil || ts > latestTS {
			latest = t
			latestTS = ts
		}
	}
	if latest != nil {
		return latest
	}
	return telemetries[len(telemetries)-1]
}

func checkPowerFlowElements(flowRaw json.RawMessage) []equipmentFault {
	var faults []equipmentFault
	var flowObj map[string]json.RawMessage
	if json.Unmarshal(applyResponsePath(flowRaw, "siteCurrentPowerFlow"), &flowObj) != nil || len(flowObj) == 0 {
		return faults
	}
	for _, key := range []string{"GRID", "PV", "STORAGE"} {
		if elementStatus(flowObj, key) == "Disabled" {
			faults = append(faults, equipmentFault{Category: "system", Name: key, Issue: "disabled", Detail: fmt.Sprintf("%s element reports status Disabled", key)})
		}
	}
	return faults
}
