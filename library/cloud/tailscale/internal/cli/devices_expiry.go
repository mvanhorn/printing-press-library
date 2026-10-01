// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type expiryView struct {
	Items        []tsadmin.ExpiryItem `json:"items"`
	WithinDays   int                  `json:"within_days"`
	FlaggedCount int                  `json:"flagged_count"`
	DeviceCount  int                  `json:"device_count"`
	KeyCount     int                  `json:"key_count,omitempty"`
	GeneratedAt  string               `json:"generated_at"`
	Warnings     []string             `json:"warnings,omitempty"`
}

type apiKey struct {
	ID          string `json:"id"`
	KeyType     string `json:"keyType"`
	Description string `json:"description"`
	Expires     string `json:"expires"`
	Revoked     string `json:"revoked"`
	Invalid     bool   `json:"invalid"`
	UserID      string `json:"userId"`
}

func newNovelDevicesExpiryCmd(flags *rootFlags) *cobra.Command {
	var tailnet string
	var within int
	var includeKeys, flaggedOnly, failOnFlagged bool
	cmd := &cobra.Command{
		Use:   "expiry",
		Short: "List every device by days until its key expires and flag the ones inside a window.",
		Long: strings.TrimSpace(`
Use this command for a fleet-wide key-expiry report or a cron/CI expiry gate.
Do NOT use this command for the full state of one device; use 'devices inspect'
instead.

Reads every device from the admin API (not only peers visible from this
machine) and reports days left, soonest first. status is expired, expiring
(inside --within days), ok, never (key expiry disabled), or unknown.
--include-keys adds auth keys, API access tokens, and OAuth clients; if the
keys cannot be read the report is still printed and the command exits non-zero.
--fail-on-flagged exits 1 when any item is flagged, for cron and CI.`),
		Example: strings.Trim(`
  tailscale-pp-cli devices expiry --within 14
  tailscale-pp-cli devices expiry --include-keys --flagged-only
  tailscale-pp-cli devices expiry --within 30 --agent --select items.machine,items.days_left,items.flagged`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "devices expiry")
			}
			if within < 0 {
				return usageErr(fmt.Errorf("--within must be zero or more days"))
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
			selfID := localSelfID(ctx)
			now := time.Now().UTC()
			var keysErr error
			view := expiryView{Items: make([]tsadmin.ExpiryItem, 0, len(devs)), WithinDays: within, DeviceCount: len(devs), GeneratedAt: now.Format(time.RFC3339)}
			for _, d := range devs {
				status, days, flagged := tsadmin.ClassifyExpiry(d.Expires, d.KeyExpiryDisabled, within, now)
				expires := d.Expires
				if d.KeyExpiryDisabled {
					expires = "" // the stored date is stale once expiry is disabled
				}
				view.Items = append(view.Items, tsadmin.ExpiryItem{
					Kind: "device", ID: d.PreferredID(), Machine: d.Label(), Hostname: d.Hostname, Name: d.Name, User: d.User, OS: d.OS, Tags: d.Tags,
					Expires: expires, DaysLeft: days, KeyExpiryDisabled: d.KeyExpiryDisabled, LastSeen: d.LastSeen,
					Status: status, Flagged: flagged, Self: selfID != "" && selfID == d.NodeID,
				})
			}
			if includeKeys {
				data, err := c.GetNoCache(ctx, tailnetPath(tailnet, "/keys"), map[string]string{"all": "true"})
				if err != nil {
					keysErr = classifyAPIErrorOnly(err)
					view.Warnings = append(view.Warnings, fmt.Sprintf("keys could not be read, so they are missing from this report: %v", keysErr))
				} else {
					var resp struct {
						Keys []apiKey `json:"keys"`
					}
					if err := json.Unmarshal(data, &resp); err != nil {
						return apiErr(fmt.Errorf("decoding keys: %w", err))
					}
					for _, k := range resp.Keys {
						if k.Invalid || k.Revoked != "" {
							continue
						}
						// Keys without an expires field (e.g. OAuth clients) do not expire.
						status, days, flagged := tsadmin.ClassifyExpiry(k.Expires, strings.TrimSpace(k.Expires) == "", within, now)
						view.Items = append(view.Items, tsadmin.ExpiryItem{
							Kind: "key", ID: k.ID, KeyType: k.KeyType, Description: k.Description, Expires: k.Expires,
							DaysLeft: days, Status: status, Flagged: flagged,
						})
						view.KeyCount++
					}
				}
			}
			for _, it := range view.Items {
				if it.Flagged {
					view.FlaggedCount++
				}
			}
			if flaggedOnly {
				kept := make([]tsadmin.ExpiryItem, 0)
				for _, it := range view.Items {
					if it.Flagged {
						kept = append(kept, it)
					}
				}
				view.Items = kept
			}
			tsadmin.SortExpiry(view.Items)
			var gateErr error
			switch {
			case keysErr != nil:
				gateErr = keysErr
			case failOnFlagged && view.FlaggedCount > 0:
				gateErr = fmt.Errorf("%d item(s) expire within %d days", view.FlaggedCount, within)
			}
			return emitOrRender(cmd, flags, view, func() { renderExpiry(cmd, view) }, gateErr)
		},
	}
	cmd.Flags().IntVar(&within, "within", 30, "Flag items expiring within this many days")
	cmd.Flags().BoolVar(&includeKeys, "include-keys", false, "Also report auth keys, API access tokens, and OAuth clients")
	cmd.Flags().BoolVar(&flaggedOnly, "flagged-only", false, "Only show expired or expiring items")
	cmd.Flags().BoolVar(&failOnFlagged, "fail-on-flagged", false, "Exit 1 when any item is expired or expiring (for cron/CI)")
	addTailnetFlag(cmd, &tailnet)
	return cmd
}

func renderExpiry(cmd *cobra.Command, v expiryView) {
	w := cmd.OutOrStdout()
	if len(v.Items) == 0 {
		fmt.Fprintf(w, "Nothing to report (%d devices checked).\n", v.DeviceCount)
		for _, warn := range v.Warnings {
			fmt.Fprintf(w, "warning: %s\n", warn)
		}
		return
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "KIND\tNAME\tSTATUS\tDAYS LEFT\tEXPIRES")
	for _, it := range v.Items {
		name := it.Machine
		if name == "" {
			name = it.Hostname
		}
		if it.Kind == "key" {
			name = strings.TrimSpace(it.KeyType + " " + it.Description)
		}
		if it.Self {
			name += " (this machine)"
		}
		days := "-"
		if it.DaysLeft != nil {
			days = fmt.Sprintf("%d", *it.DaysLeft)
		}
		marker := ""
		if it.Flagged {
			marker = " !"
		}
		expires := it.Expires
		if it.KeyExpiryDisabled {
			expires = "disabled"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s%s\t%s\t%s\n", it.Kind, name, it.Status, marker, days, expires)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\n%d of %d item(s) expire within %d days.\n", v.FlaggedCount, v.DeviceCount+v.KeyCount, v.WithinDays)
	for _, warn := range v.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
}
