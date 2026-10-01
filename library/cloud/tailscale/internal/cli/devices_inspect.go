// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type inspectSection struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

type inspectChange struct {
	EventTime string          `json:"event_time"`
	Action    string          `json:"action"`
	Actor     string          `json:"actor,omitempty"`
	Property  string          `json:"property,omitempty"`
	Old       json.RawMessage `json:"old,omitempty"`
	New       json.RawMessage `json:"new,omitempty"`
}

type inspectView struct {
	Device struct {
		deviceRef
		ID              string   `json:"legacy_id,omitempty"`
		Addresses       []string `json:"addresses"`
		User            string   `json:"user,omitempty"`
		OS              string   `json:"os,omitempty"`
		ClientVersion   string   `json:"client_version,omitempty"`
		UpdateAvailable bool     `json:"update_available"`
		Tags            []string `json:"tags,omitempty"`
		Authorized      bool     `json:"authorized"`
		Online          bool     `json:"online"`
		LastSeen        string   `json:"last_seen,omitempty"`
		IsExternal      bool     `json:"is_external"`
		IsSelf          bool     `json:"is_self"`
	} `json:"device"`
	Routes tsadmin.RouteState `json:"routes"`
	Expiry struct {
		Expires           string `json:"expires,omitempty"`
		DaysLeft          *int   `json:"days_left"`
		KeyExpiryDisabled bool   `json:"key_expiry_disabled"`
		Status            string `json:"status"`
	} `json:"expiry"`
	Shares struct {
		inspectSection
		Items []shareRow `json:"items"`
	} `json:"shares"`
	RecentChanges struct {
		inspectSection
		Since string          `json:"since"`
		Items []inspectChange `json:"items"`
	} `json:"recent_changes"`
}

func newNovelDevicesInspectCmd(flags *rootFlags) *cobra.Command {
	var tailnet, since string
	cmd := &cobra.Command{
		Use:   "inspect <device>",
		Short: "Get one device's identity, routes, key expiry, shares, and recent changes in a single record.",
		Long: strings.TrimSpace(`
Use this command to get one device's identity, routes, expiry, shares, and
recent changes in a single record before acting on it. Do NOT use this command
for fleet-wide lists; use 'devices expiry', 'routes overview', or 'shares audit'
instead.

<device> accepts self (this machine, via the local tailscale command, whose
status ID equals the API nodeId), a hostname, MagicDNS name, Tailscale IP, or
nodeId. A section the token cannot read (for example the audit log) is marked
unavailable instead of failing the command. The audit-log window defaults to
24h because the API gets slower as the window grows (about 7s for 7d).`),
		Example: strings.Trim(`
  tailscale-pp-cli devices inspect self
  tailscale-pp-cli devices inspect home-mac --since 7d --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "device=self"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "devices inspect")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("<device> is required (self, hostname, MagicDNS name, Tailscale IP, or nodeId)"))
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil || window <= 0 {
				return usageErr(fmt.Errorf("--since must be a duration like 7d, 24h, or 2w"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			devs, err := fetchDevicesLive(ctx, c, tailnet, false)
			if err != nil {
				return err
			}
			ref, err := resolveDevice(ctx, devs, args[0])
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			start := now.Add(-window)
			// The audit log is the slow call (about 2s for 24h), so start it
			// as soon as the device is known and overlap it with the rest.
			audit := make(chan auditResult, 1)
			go func() {
				audit <- fetchRecentChanges(ctx, c, tailnet, ref, start, now)
			}()
			d, err := fetchDeviceLive(ctx, c, ref.PreferredID())
			if err != nil {
				return err
			}
			var v inspectView
			v.Device.deviceRef = refOf(d)
			v.Device.ID = d.ID
			v.Device.Addresses = d.Addresses
			v.Device.User = d.User
			v.Device.OS = d.OS
			v.Device.ClientVersion = d.ClientVersion
			v.Device.UpdateAvailable = d.UpdateAvailable
			v.Device.Tags = d.Tags
			v.Device.Authorized = d.Authorized
			v.Device.Online = d.ConnectedToControl
			v.Device.LastSeen = d.LastSeen
			v.Device.IsExternal = d.IsExternal
			v.Device.IsSelf = strings.EqualFold(strings.TrimSpace(args[0]), "self") || localSelfID(ctx) == d.NodeID
			v.Routes = tsadmin.ComputeRouteState(d.AdvertisedRoutes, d.EnabledRoutes)
			status, days, _ := tsadmin.ClassifyExpiry(d.Expires, d.KeyExpiryDisabled, 0, now)
			v.Expiry.Expires, v.Expiry.DaysLeft, v.Expiry.KeyExpiryDisabled, v.Expiry.Status = d.Expires, days, d.KeyExpiryDisabled, status

			v.Shares.Items = make([]shareRow, 0)
			if d.IsExternal {
				v.Shares.Error = "device is shared into this tailnet; its share invites belong to the owner"
			} else if invites, err := fetchDeviceInvites(ctx, c, d.PreferredID()); err != nil {
				v.Shares.Error = err.Error()
			} else {
				v.Shares.Available = true
				for _, inv := range invites {
					v.Shares.Items = append(v.Shares.Items, shareRowOf(inv, d.Label(), d.PreferredID(), false))
				}
			}

			res := <-audit
			v.RecentChanges.Since = start.Format(time.RFC3339)
			v.RecentChanges.Items = res.items
			v.RecentChanges.Available = res.err == ""
			v.RecentChanges.Error = res.err

			if ok, err := emitMachine(cmd, flags, v); ok {
				return err
			}
			renderInspect(cmd, v)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Audit-log window for recent changes (e.g. 24h, 7d); wider windows are slower")
	addTailnetFlag(cmd, &tailnet)
	return cmd
}

type auditResult struct {
	items []inspectChange
	err   string
}

// fetchRecentChanges reads configuration audit-log entries that target dev.
// Errors are returned as text because the section degrades instead of
// failing the whole record.
func fetchRecentChanges(ctx context.Context, c *client.Client, tailnet string, dev tsadmin.Device, start, end time.Time) auditResult {
	res := auditResult{items: make([]inspectChange, 0)}
	logParams := map[string]string{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339), "target": dev.NodeID}
	resp, err := getLive[struct {
		Logs []struct {
			EventTime string `json:"eventTime"`
			Action    string `json:"action"`
			Actor     struct {
				LoginName   string `json:"loginName"`
				DisplayName string `json:"displayName"`
			} `json:"actor"`
			Target struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Property string `json:"property"`
			} `json:"target"`
			Old json.RawMessage `json:"old"`
			New json.RawMessage `json:"new"`
		} `json:"logs"`
	}](ctx, c, tailnetPath(tailnet, "/logging/configuration"), logParams, "audit log")
	if err != nil {
		res.err = err.Error()
		return res
	}
	for _, l := range resp.Logs {
		if l.Target.ID != dev.NodeID && l.Target.ID != dev.ID && !strings.EqualFold(l.Target.Name, dev.Name) {
			continue
		}
		actor := l.Actor.LoginName
		if actor == "" {
			actor = l.Actor.DisplayName
		}
		res.items = append(res.items, inspectChange{EventTime: l.EventTime, Action: l.Action, Actor: actor, Property: l.Target.Property, Old: nonNullRaw(l.Old), New: nonNullRaw(l.New)})
	}
	return res
}

func nonNullRaw(r json.RawMessage) json.RawMessage {
	if len(r) == 0 || rawJSONNull(r) {
		return nil
	}
	return r
}

func renderInspect(cmd *cobra.Command, v inspectView) {
	w := cmd.OutOrStdout()
	self := ""
	if v.Device.IsSelf {
		self = " (this machine)"
	}
	fmt.Fprintf(w, "%s%s  %s\n", v.Device.Hostname, self, v.Device.NodeID)
	fmt.Fprintf(w, "  addresses: %s\n", joinOrDash(v.Device.Addresses))
	fmt.Fprintf(w, "  os: %s  client: %s  online: %t  last seen: %s\n", v.Device.OS, v.Device.ClientVersion, v.Device.Online, v.Device.LastSeen)
	fmt.Fprintf(w, "  user: %s  tags: %s\n", v.Device.User, joinOrDash(v.Device.Tags))
	days := "-"
	if v.Expiry.DaysLeft != nil {
		days = fmt.Sprintf("%d days", *v.Expiry.DaysLeft)
	}
	fmt.Fprintf(w, "  key expiry: %s (%s)\n", v.Expiry.Status, days)
	fmt.Fprintf(w, "  routes: exit node %s; approved %s; pending %s\n", v.Routes.ExitNode, joinOrDash(v.Routes.Enabled), joinOrDash(v.Routes.Pending))
	if v.Shares.Available {
		fmt.Fprintf(w, "  shares: %d invite(s)\n", len(v.Shares.Items))
		for _, s := range v.Shares.Items {
			fmt.Fprintf(w, "    %s accepted=%t by=%s redeemable=%t\n", s.InviteID, s.Accepted, s.AcceptedBy, s.Redeemable)
		}
	} else {
		fmt.Fprintf(w, "  shares: unavailable (%s)\n", v.Shares.Error)
	}
	if v.RecentChanges.Available {
		fmt.Fprintf(w, "  changes since %s: %d\n", v.RecentChanges.Since, len(v.RecentChanges.Items))
		for _, ch := range v.RecentChanges.Items {
			fmt.Fprintf(w, "    %s %s %s by %s\n", wholeSecondTime(ch.EventTime), ch.Action, ch.Property, ch.Actor)
		}
	} else {
		fmt.Fprintf(w, "  recent changes: unavailable (%s)\n", v.RecentChanges.Error)
	}
}

// wholeSecondTime trims an audit-log timestamp to whole-second RFC 3339 for
// the human table, matching every other time field. JSON keeps the raw value.
func wholeSecondTime(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339)
}
