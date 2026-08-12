// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: see .printing-press-patches/ for context. Hand-authored, not
// generator output — regen-merge preserves this file.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/spf13/cobra"
)

type portAuditPort struct {
	Idx          int    `json:"idx"`
	State        string `json:"state"`
	Connector    string `json:"connector"`
	MaxSpeedMbps int    `json:"maxSpeedMbps"`
	SpeedMbps    int    `json:"speedMbps,omitempty"`
	PoEEnabled   bool   `json:"poe_enabled,omitempty"`
	PoEState     string `json:"poe_state,omitempty"`
}

type portAuditDevice struct {
	DeviceID  string          `json:"device_id"`
	Name      string          `json:"name"`
	Model     string          `json:"model"`
	Ports     []portAuditPort `json:"ports"`
	PortsUp   int             `json:"ports_up"`
	PortsDown int             `json:"ports_down"`
	PoEActive int             `json:"poe_active"`
}

func newNovelPortAuditCmd(flags *rootFlags) *cobra.Command {
	var flagSite string

	cmd := &cobra.Command{
		Use:   "port-audit",
		Short: "Review port utilization and PoE status across every switch on a site in one table.",
		Long: "Lists per-port link state and PoE status for every switching- or " +
			"gateway-capable device on a site. Per-port interface data does not " +
			"appear in any list/sync response for this API — only a per-device " +
			"detail fetch returns it — so this command reads the device list from " +
			"the local mirror (for device ids) but fetches interfaces live, one " +
			"call per switching/gateway device. Run 'unifi-pp-cli sync' first so " +
			"the device list is available.",
		Example:     "  unifi-pp-cli port-audit --site default --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "port-audit")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("unifi-pp-cli")
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), []portAuditDevice{}, flags)
				}
				return nil
			}

			siteID, _, err := resolveSiteIDLocal(ctx, db.DB(), flagSite)
			if err != nil {
				_ = db.Close()
				if isNoLocalDataYet(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: unifi-pp-cli sync\n", err)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), []portAuditDevice{}, flags)
					}
					return nil
				}
				return err
			}
			deviceRows, err := resourceRows(ctx, db.DB(), "devices", siteID)
			_ = db.Close()
			if err != nil {
				return err
			}

			type deviceJSON struct {
				ID       string   `json:"id"`
				Name     string   `json:"name"`
				Model    string   `json:"model"`
				Features []string `json:"features"`
			}
			var candidates []deviceJSON
			for _, id := range sortedKeys(deviceRows) {
				var d deviceJSON
				if json.Unmarshal(deviceRows[id], &d) != nil {
					continue
				}
				for _, f := range d.Features {
					if f == "switching" || f == "gateway" {
						candidates = append(candidates, d)
						break
					}
				}
			}

			results := make([]portAuditDevice, 0, len(candidates))
			if cliutil.IsDogfoodEnv() && len(candidates) > 1 {
				candidates = candidates[:1]
			}
			if len(candidates) > 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				for _, d := range candidates {
					path := replacePathParam(replacePathParam("/v1/sites/{siteId}/devices/{deviceId}", "siteId", siteID), "deviceId", d.ID)
					data, err := c.Get(ctx, path, nil)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					var detail struct {
						Interfaces struct {
							Ports []struct {
								Idx          int    `json:"idx"`
								State        string `json:"state"`
								Connector    string `json:"connector"`
								MaxSpeedMbps int    `json:"maxSpeedMbps"`
								SpeedMbps    int    `json:"speedMbps"`
								PoE          *struct {
									Enabled bool   `json:"enabled"`
									State   string `json:"state"`
								} `json:"poe"`
							} `json:"ports"`
						} `json:"interfaces"`
					}
					if err := json.Unmarshal(data, &detail); err != nil {
						return fmt.Errorf("parsing device %s interfaces: %w", d.ID, err)
					}
					pd := portAuditDevice{DeviceID: d.ID, Name: d.Name, Model: d.Model, Ports: []portAuditPort{}}
					for _, p := range detail.Interfaces.Ports {
						port := portAuditPort{Idx: p.Idx, State: p.State, Connector: p.Connector, MaxSpeedMbps: p.MaxSpeedMbps, SpeedMbps: p.SpeedMbps}
						if p.PoE != nil {
							port.PoEEnabled = p.PoE.Enabled
							port.PoEState = p.PoE.State
							if p.PoE.State == "UP" {
								pd.PoEActive++
							}
						}
						if p.State == "UP" {
							pd.PortsUp++
						} else {
							pd.PortsDown++
						}
						pd.Ports = append(pd.Ports, port)
					}
					results = append(results, pd)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(w, "No switching- or gateway-capable devices found in the local mirror for this site.")
				return nil
			}
			for _, d := range results {
				fmt.Fprintf(w, "%s (%s): %d up / %d down, PoE active on %d port(s)\n", d.Name, d.Model, d.PortsUp, d.PortsDown, d.PoEActive)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "", "Site id, internalReference, or name (default: the only synced site)")
	return cmd
}
