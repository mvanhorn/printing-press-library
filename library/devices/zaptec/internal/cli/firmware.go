// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// newFirmwareCmd groups firmware operations: list, upgrade, and drift detection.
func newFirmwareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "firmware",
		Short:       "Inspect and manage charger firmware across an installation",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newFirmwareListCmd(flags))
	cmd.AddCommand(newFirmwareDriftCmd(flags))
	cmd.AddCommand(newFirmwareUpgradeCmd(flags))
	return cmd
}

type fwRow struct {
	ChargerID      string `json:"charger_id"`
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	Available      string `json:"available_version"`
	UpToDate       bool   `json:"up_to_date"`
	Online         bool   `json:"online"`
	Behind         bool   `json:"behind_fleet,omitempty"`
}

func fetchFirmware(flags *rootFlags, installationID string) ([]fwRow, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := replacePathParam("/api/chargerFirmware/installation/{installationId}", "installationId", installationID)
	data, err := c.Get(path, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		var wrapped struct {
			Data []map[string]json.RawMessage `json:"Data"`
		}
		if json.Unmarshal(data, &wrapped) == nil {
			raw = wrapped.Data
		} else {
			return nil, fmt.Errorf("parsing firmware response: %w", err)
		}
	}
	rows := make([]fwRow, 0, len(raw))
	for _, item := range raw {
		var r fwRow
		_ = json.Unmarshal(item["ChargerId"], &r.ChargerID)
		_ = json.Unmarshal(item["DeviceName"], &r.Name)
		if r.Name == "" {
			_ = json.Unmarshal(item["ChargerName"], &r.Name)
		}
		_ = json.Unmarshal(item["CurrentVersion"], &r.CurrentVersion)
		_ = json.Unmarshal(item["AvailableVersion"], &r.Available)
		_ = json.Unmarshal(item["IsUpToDate"], &r.UpToDate)
		_ = json.Unmarshal(item["IsOnline"], &r.Online)
		rows = append(rows, r)
	}
	return rows, nil
}

func newFirmwareListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list <installation-id>",
		Short:       "List firmware versions for every charger in an installation",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  zaptec-pp-cli firmware list " + exampleChargerID,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			rows, err := fetchFirmware(flags, args[0])
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CHARGER\tCURRENT\tAVAILABLE\tUP-TO-DATE\tONLINE")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%v\n", r.Name, r.CurrentVersion, r.Available, r.UpToDate, r.Online)
			}
			return tw.Flush()
		},
	}
}

// newFirmwareDriftCmd groups an installation's chargers by current firmware
// version, determines the fleet's modal version, and flags chargers behind it.
func newFirmwareDriftCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "drift <installation-id>",
		Short:       "Flag chargers behind the fleet's majority firmware version",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  zaptec-pp-cli firmware drift " + exampleChargerID + " --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			rows, err := fetchFirmware(flags, args[0])
			if err != nil {
				return err
			}

			counts := map[string]int{}
			for _, r := range rows {
				if r.CurrentVersion != "" {
					counts[r.CurrentVersion]++
				}
			}
			modal := ""
			best := -1
			versions := make([]string, 0, len(counts))
			for v := range counts {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			for _, v := range versions {
				if counts[v] > best {
					best = counts[v]
					modal = v
				}
			}

			var behind []fwRow
			for _, r := range rows {
				if (modal != "" && r.CurrentVersion != modal) || !r.UpToDate {
					r.Behind = true
					behind = append(behind, r)
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"fleet_version": modal,
					"total":         len(rows),
					"behind":        behind,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fleet majority version: %s (%d/%d chargers)\n", modal, best, len(rows))
			if len(behind) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No firmware drift — all chargers are aligned and up to date.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CHARGER\tCURRENT\tAVAILABLE\tUP-TO-DATE")
			for _, r := range behind {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n", r.Name, r.CurrentVersion, r.Available, r.UpToDate)
			}
			return tw.Flush()
		},
	}
}

func newFirmwareUpgradeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "upgrade <charger-id>",
		Short:   "Trigger a firmware upgrade on a charger",
		Example: "  zaptec-pp-cli firmware upgrade " + exampleChargerID + " --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdUpgradeFirmware, "upgrade-firmware")
		},
	}
}
