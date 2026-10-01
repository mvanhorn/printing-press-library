// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type routeRow struct {
	Hostname string `json:"hostname"`
	NodeID   string `json:"node_id"`
	OS       string `json:"os,omitempty"`
	Online   bool   `json:"online"`
	tsadmin.RouteState
}

type routeListView struct {
	Items          []routeRow `json:"items"`
	ScannedDevices int        `json:"scanned_devices"`
	PendingCount   int        `json:"devices_with_pending"`
}

func newNovelRoutesOverviewCmd(flags *rootFlags) *cobra.Command {
	var tailnet, deviceSel string
	var pendingOnly, exitOnly, all bool
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "See approved, pending, and stale routes and exit-node state for every device in one table.",
		Long: strings.TrimSpace(`
Use this command to see advertised, approved, pending, and stale routes and
exit-node state across devices. Do NOT use this command to change approvals; use
'routes approve' or 'routes unapprove' instead.

pending = advertised by the device but not approved. stale = approved but no
longer advertised. exit_node is one of none, approved, pending, partial (only
one of 0.0.0.0/0 and ::/0), or approved-not-advertised. By default only devices
with any advertised or approved route are shown; --all shows every device.`),
		Example: strings.Trim(`
  tailscale-pp-cli routes overview
  tailscale-pp-cli routes overview --pending
  tailscale-pp-cli routes overview --exit-nodes --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "routes overview")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			devs, err := fetchDevicesLive(ctx, c, tailnet, true)
			if err != nil {
				return err
			}
			if deviceSel != "" {
				d, err := resolveDevice(ctx, devs, deviceSel)
				if err != nil {
					return err
				}
				devs = []tsadmin.Device{d}
			}
			view := routeListView{Items: make([]routeRow, 0), ScannedDevices: len(devs)}
			for _, d := range devs {
				st := tsadmin.ComputeRouteState(d.AdvertisedRoutes, d.EnabledRoutes)
				hasRoutes := len(st.Advertised) > 0 || len(st.Enabled) > 0
				if !all && deviceSel == "" && !hasRoutes {
					continue
				}
				if pendingOnly && len(st.Pending) == 0 {
					continue
				}
				if exitOnly && st.ExitNode == tsadmin.ExitNone {
					continue
				}
				if len(st.Pending) > 0 {
					view.PendingCount++
				}
				view.Items = append(view.Items, routeRow{Hostname: d.Label(), NodeID: d.PreferredID(), OS: d.OS, Online: d.ConnectedToControl, RouteState: st})
			}
			sort.SliceStable(view.Items, func(i, j int) bool { return view.Items[i].Hostname < view.Items[j].Hostname })
			if ok, err := emitMachine(cmd, flags, view); ok {
				return err
			}
			if len(view.Items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matching devices (%d scanned).\n", view.ScannedDevices)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "HOST\tEXIT NODE\tAPPROVED\tPENDING\tSTALE")
			for _, r := range view.Items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Hostname, r.ExitNode, joinOrDash(r.Enabled), joinOrDash(r.Pending), joinOrDash(r.Stale))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&pendingOnly, "pending", false, "Only devices with advertised routes awaiting approval")
	cmd.Flags().BoolVar(&exitOnly, "exit-nodes", false, "Only devices that advertise or have approved exit-node routes")
	cmd.Flags().BoolVar(&all, "all", false, "Include devices with no routes")
	cmd.Flags().StringVar(&deviceSel, "device", "", "Show one device (self, hostname, MagicDNS name, Tailscale IP, or nodeId)")
	addTailnetFlag(cmd, &tailnet)
	return cmd
}
