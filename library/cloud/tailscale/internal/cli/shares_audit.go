// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type shareRow struct {
	InviteID        string `json:"invite_id"`
	Device          string `json:"device"`
	NodeID          string `json:"node_id"`
	Created         string `json:"created"`
	Email           string `json:"email,omitempty"`
	Accepted        bool   `json:"accepted"`
	AcceptedBy      string `json:"accepted_by,omitempty"`
	MultiUse        bool   `json:"multi_use"`
	AllowExitNode   bool   `json:"allow_exit_node"`
	Redeemable      bool   `json:"redeemable"`
	LastEmailSentAt string `json:"last_email_sent_at,omitempty"`
	InviteURL       string `json:"invite_url,omitempty"`
}

// shareRowOf builds the output row for one invite. The invite link is
// redacted unless showURL is set, because anyone holding it can accept.
func shareRowOf(inv tsadmin.Invite, device, nodeID string, showURL bool) shareRow {
	row := shareRow{
		InviteID: inv.ID, Device: device, NodeID: nodeID, Created: inv.Created, Email: inv.Email,
		Accepted: inv.Accepted, AcceptedBy: inv.AcceptedByLogin(), MultiUse: inv.MultiUse, AllowExitNode: inv.AllowExitNode,
		Redeemable: inv.Redeemable(), LastEmailSentAt: inv.LastEmailSentAt, InviteURL: tsadmin.RedactInviteURL(inv.InviteURL),
	}
	if showURL {
		row.InviteURL = inv.InviteURL
	}
	return row
}

type fetchFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

type shareListView struct {
	Items          []shareRow     `json:"items"`
	ScannedDevices int            `json:"scanned_devices"`
	RedeemableOpen int            `json:"redeemable_count"`
	FetchFailures  []fetchFailure `json:"fetch_failures,omitempty"`
}

type shareQuery struct {
	tailnet   string
	deviceSel string
	filter    tsadmin.ShareFilter
	showURLs  bool
}

// collectShares lists share invites across every owned device (or one device)
// and applies the filter. Per-device failures are reported, not dropped.
func collectShares(ctx context.Context, c *client.Client, q shareQuery) (shareListView, error) {
	devs, err := fetchDevicesLive(ctx, c, q.tailnet, false)
	if err != nil {
		return shareListView{}, err
	}
	if q.deviceSel != "" {
		d, err := resolveDevice(ctx, devs, q.deviceSel)
		if err != nil {
			return shareListView{}, err
		}
		devs = []tsadmin.Device{d}
	}
	owned := make([]tsadmin.Device, 0, len(devs))
	for _, d := range devs {
		if !d.IsExternal {
			owned = append(owned, d)
		}
	}
	results, errs := cliutil.FanoutRun(ctx, owned,
		func(d tsadmin.Device) string { return d.PreferredID() },
		func(ctx context.Context, d tsadmin.Device) ([]tsadmin.Invite, error) {
			return fetchDeviceInvites(ctx, c, d.PreferredID())
		},
		cliutil.WithConcurrency(6),
	)
	byID := map[string]tsadmin.Device{}
	for _, d := range owned {
		byID[d.PreferredID()] = d
	}
	view := shareListView{Items: make([]shareRow, 0), ScannedDevices: len(owned)}
	for _, r := range results {
		d := byID[r.Source]
		for _, inv := range r.Value {
			if !q.filter.Match(inv) {
				continue
			}
			row := shareRowOf(inv, d.Label(), d.PreferredID(), q.showURLs)
			if row.Redeemable {
				view.RedeemableOpen++
			}
			view.Items = append(view.Items, row)
		}
	}
	for _, e := range errs {
		label := e.Source
		if d, ok := byID[e.Source]; ok {
			label = d.Label() + " (" + e.Source + ")"
		}
		view.FetchFailures = append(view.FetchFailures, fetchFailure{Source: label, Error: e.Err.Error()})
	}
	sort.SliceStable(view.Items, func(i, j int) bool {
		if view.Items[i].Device != view.Items[j].Device {
			return view.Items[i].Device < view.Items[j].Device
		}
		return view.Items[i].Created < view.Items[j].Created
	})
	return view, nil
}

func newNovelSharesAuditCmd(flags *rootFlags) *cobra.Command {
	var q shareQuery
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List every machine-share invite across the tailnet with who accepted it and which invites are still redeemable.",
		Long: strings.TrimSpace(`
Use this command to list machine-share invites across the tailnet, filtered by
device, state, or acceptor. Do NOT use this command for the full state of one
device; use 'devices inspect' instead.

The API lists share invites one device at a time; this checks every device you
own and joins the results. redeemable means anyone holding the invite link can
still accept it (unaccepted invites, and multi-use invites even after one use).
Invite links are redacted unless --show-urls is set.`),
		Example: strings.Trim(`
  tailscale-pp-cli shares audit
  tailscale-pp-cli shares audit --pending
  tailscale-pp-cli shares audit --device nas --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "shares audit")
			}
			if q.filter.Pending && q.filter.Accepted {
				return usageErr(fmt.Errorf("--pending and --accepted are mutually exclusive"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			view, err := collectShares(ctx, c, q)
			if err != nil {
				return err
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: share invites for %d of %d devices could not be read; results are partial\n", len(view.FetchFailures), view.ScannedDevices)
			}
			if ok, err := emitMachine(cmd, flags, view); ok {
				return err
			}
			if len(view.Items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matching share invites (%d devices checked).\n", view.ScannedDevices)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "INVITE\tDEVICE\tACCEPTED BY\tREDEEMABLE\tEXIT NODE\tCREATED")
			for _, r := range view.Items {
				by := r.AcceptedBy
				if !r.Accepted {
					by = "(pending)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%t\t%t\t%s\n", r.InviteID, r.Device, by, r.Redeemable, r.AllowExitNode, r.Created)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&q.deviceSel, "device", "", "Only this device (self, hostname, MagicDNS name, Tailscale IP, or nodeId)")
	cmd.Flags().BoolVar(&q.filter.Pending, "pending", false, "Only invites nobody has accepted yet")
	cmd.Flags().BoolVar(&q.filter.Accepted, "accepted", false, "Only accepted invites")
	cmd.Flags().StringVar(&q.filter.AcceptedBy, "accepted-by", "", "Only invites accepted by this login name")
	cmd.Flags().BoolVar(&q.showURLs, "show-urls", false, "Include full invite links (anyone with a link can accept it)")
	addTailnetFlag(cmd, &q.tailnet)
	return cmd
}
