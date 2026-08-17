// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: see .printing-press-patches/ for context. Hand-authored, not
// generator output — regen-merge preserves this file.

// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type topologyClient struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Type string `json:"type,omitempty"`
}

type topologyDevice struct {
	ID       string           `json:"id"`
	Name     string           `json:"name,omitempty"`
	Model    string           `json:"model,omitempty"`
	State    string           `json:"state,omitempty"`
	Features []string         `json:"features,omitempty"`
	Clients  []topologyClient `json:"clients"`
}

type topologyView struct {
	Site              string           `json:"site"`
	Devices           []topologyDevice `json:"devices"`
	UnattachedClients []topologyClient `json:"unattached_clients"`
	Note              string           `json:"note,omitempty"`
}

func newNovelTopologyCmd(flags *rootFlags) *cobra.Command {
	var flagSite string

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "See the physical device tree (gateway to switches to APs) built entirely from local mirror data, no live crawl needed.",
		Long: "Groups every synced client under the device it is attached to " +
			"(via each client's uplinkDeviceId), giving a two-level device→client " +
			"tree entirely from the local mirror. The API's device-list response " +
			"does not include device-to-device uplink chaining (that only appears " +
			"on a per-device detail fetch), so a switch-behind-switch or " +
			"AP-behind-switch relationship is not shown — every device is listed " +
			"at the top level. Run 'unifi-pp-cli sync' first to populate the mirror.",
		Example:     "  unifi-pp-cli topology --site default --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "topology")
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
					return printJSONFiltered(cmd.OutOrStdout(), topologyView{Devices: []topologyDevice{}, UnattachedClients: []topologyClient{}}, flags)
				}
				return nil
			}
			defer db.Close()

			siteID, siteName, err := resolveSiteIDLocal(ctx, db.DB(), flagSite)
			if err != nil {
				if isNoLocalDataYet(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: unifi-pp-cli sync\n", err)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), topologyView{Devices: []topologyDevice{}, UnattachedClients: []topologyClient{}}, flags)
					}
					return nil
				}
				return err
			}

			deviceRows, err := resourceRows(ctx, db.DB(), "devices", siteID)
			if err != nil {
				return err
			}
			clientRows, err := resourceRows(ctx, db.DB(), "clients", siteID)
			if err != nil {
				return err
			}

			type deviceJSON struct {
				ID       string   `json:"id"`
				Name     string   `json:"name"`
				Model    string   `json:"model"`
				State    string   `json:"state"`
				Features []string `json:"features"`
			}
			type clientJSON struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				MACAddress     string `json:"macAddress"`
				IPAddress      string `json:"ipAddress"`
				Type           string `json:"type"`
				UplinkDeviceID string `json:"uplinkDeviceId"`
			}

			view := topologyView{Site: siteName, Devices: []topologyDevice{}, UnattachedClients: []topologyClient{}}
			byDeviceID := make(map[string]int, len(deviceRows))
			for _, id := range sortedKeys(deviceRows) {
				var d deviceJSON
				if err := json.Unmarshal(deviceRows[id], &d); err != nil {
					continue
				}
				byDeviceID[id] = len(view.Devices)
				view.Devices = append(view.Devices, topologyDevice{
					ID: d.ID, Name: d.Name, Model: d.Model, State: d.State,
					Features: d.Features, Clients: []topologyClient{},
				})
			}
			for _, id := range sortedKeys(clientRows) {
				var c clientJSON
				if err := json.Unmarshal(clientRows[id], &c); err != nil {
					continue
				}
				tc := topologyClient{ID: c.ID, Name: c.Name, MAC: c.MACAddress, IP: c.IPAddress, Type: c.Type}
				if idx, ok := byDeviceID[c.UplinkDeviceID]; ok {
					view.Devices[idx].Clients = append(view.Devices[idx].Clients, tc)
				} else {
					view.UnattachedClients = append(view.UnattachedClients, tc)
				}
			}
			if len(view.Devices) == 0 {
				view.Note = "no devices in the local mirror for this site; run 'unifi-pp-cli sync' to populate it"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Site: %s\n", view.Site)
			for _, d := range view.Devices {
				fmt.Fprintf(w, "%s  %s  [%s]\n", d.Name, d.Model, d.State)
				for _, c := range d.Clients {
					fmt.Fprintf(w, "  └─ %s (%s)\n", c.Name, c.IP)
				}
			}
			if len(view.UnattachedClients) > 0 {
				names := make([]string, 0, len(view.UnattachedClients))
				for _, c := range view.UnattachedClients {
					names = append(names, c.Name)
				}
				sort.Strings(names)
				fmt.Fprintf(w, "Unattached clients (%d): %v\n", len(names), names)
			}
			if view.Note != "" {
				fmt.Fprintln(w, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "", "Site id, internalReference, or name (default: the only synced site)")
	return cmd
}
