// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type revokeResult struct {
	InviteID string `json:"invite_id"`
	Device   string `json:"device,omitempty"`
	Revoked  bool   `json:"revoked"`
	Error    string `json:"error,omitempty"`
}

type revokeView struct {
	Action  string         `json:"action"`
	Mode    string         `json:"mode"`
	DryRun  bool           `json:"dry_run,omitempty"`
	Applied bool           `json:"applied"`
	Targets []shareRow     `json:"targets"`
	Results []revokeResult `json:"results,omitempty"`
	Next    string         `json:"next,omitempty"`
}

func newNovelSharesRevokeCmd(flags *rootFlags) *cobra.Command {
	var q shareQuery
	var all bool
	cmd := &cobra.Command{
		Use:   "revoke [invite-id...]",
		Short: "Revoke machine-share invites by ID or by device and acceptor, with a plan before any delete.",
		Long: strings.TrimSpace(`
Use this command to revoke machine-share invites by ID or by device/acceptor
filter. Do NOT use this command to audit shares; use 'shares audit' instead.

Pass invite IDs, or select invites on one --device with --pending,
--accepted-by, or --all (filters always need --device). Without --yes it prints which invites would be
deleted and changes nothing (as an MCP tool it is always plan-only). With --yes it calls DELETE /device-invites/{id}
for each one and reports per-invite results.

Note: the Tailscale API documents this call as deleting the invite. It does
not say whether deleting an already-accepted invite also removes the accepted
share, so confirm in the admin console after revoking an accepted invite.`),
		Example: strings.Trim(`
  tailscale-pp-cli shares revoke --device self --pending
  tailscale-pp-cli shares revoke --device nas --accepted-by contractor@github
  tailscale-pp-cli shares revoke 12345 --yes`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			// Over MCP these tools are plan-only: --yes is blocked, so the
			// read-only hint is accurate. Writes need the CLI with --yes.
			"mcp:read-only":   "true",
			"mcp:write-flags": "yes",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && cliutil.IsVerifyEnv() {
				return writeDryRun(cmd.OutOrStdout(), flags, "shares revoke")
			}
			byFilter := q.deviceSel != "" || q.filter.Pending || q.filter.AcceptedBy != "" || all
			if len(args) == 0 && !byFilter {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "shares revoke")
				}
				_ = cmd.Usage()
				return usageErr(errors.New("pass invite IDs, or --device with --pending, --accepted-by, or --all"))
			}
			if len(args) > 0 && byFilter {
				return usageErr(errors.New("pass invite IDs or filters, not both"))
			}
			if byFilter && q.deviceSel == "" {
				return usageErr(errors.New("filters need --device; revoking invites across the whole tailnet in one call is not supported (pass invite IDs instead)"))
			}
			if byFilter && !q.filter.Pending && q.filter.AcceptedBy == "" && !all {
				return usageErr(errors.New("--device alone would revoke every invite on the device; add --pending, --accepted-by, or --all"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			view := revokeView{Action: "revoke", Targets: make([]shareRow, 0), DryRun: flags.dryRun}
			var revokeErr error
			if byFilter {
				listed, err := collectShares(ctx, c, q)
				if err != nil {
					return err
				}
				if len(listed.FetchFailures) > 0 {
					return apiErr(fmt.Errorf("could not read share invites for %s: %s; nothing was revoked", listed.FetchFailures[0].Source, listed.FetchFailures[0].Error))
				}
				view.Targets = listed.Items
			} else {
				for _, id := range args {
					inv, err := getLive[tsadmin.Invite](ctx, c, "/device-invites/"+cliutil.EscapePathParam(id), nil, "invite "+id)
					if err != nil {
						return err
					}
					view.Targets = append(view.Targets, shareRowOf(inv, strconv.FormatInt(inv.DeviceID, 10), "", false))
				}
			}
			switch {
			case len(view.Targets) == 0:
				view.Mode = "no-change"
			case tsPlanOnly(flags):
				view.Mode = "plan"
				view.Next = tsApplyHint
			default:
				view.Mode = "applied"
				failed := 0
				for _, t := range view.Targets {
					res := revokeResult{InviteID: t.InviteID, Device: t.Device}
					if _, _, err := c.Delete(ctx, "/device-invites/"+cliutil.EscapePathParam(t.InviteID)); err != nil {
						res.Error = classifyAPIErrorOnly(err).Error()
						failed++
					} else {
						res.Revoked = true
					}
					view.Results = append(view.Results, res)
				}
				view.Applied = failed < len(view.Targets)
				if failed > 0 {
					revokeErr = partialFailureErr(fmt.Errorf("%d of %d revocations failed; see results", failed, len(view.Targets)))
				}
			}
			return emitOrRender(cmd, flags, view, func() {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "shares revoke: %s (%d invite(s))\n", view.Mode, len(view.Targets))
				for _, t := range view.Targets {
					fmt.Fprintf(w, "  %s %s accepted=%t by=%s\n", t.InviteID, t.Device, t.Accepted, t.AcceptedBy)
				}
				for _, r := range view.Results {
					if r.Error != "" {
						fmt.Fprintf(w, "  failed %s: %s\n", r.InviteID, r.Error)
					}
				}
				if view.Next != "" {
					fmt.Fprintf(w, "  %s\n", view.Next)
				}
			}, revokeErr)
		},
	}
	cmd.Flags().StringVar(&q.deviceSel, "device", "", "Select invites on this device (self, hostname, MagicDNS name, Tailscale IP, or nodeId)")
	cmd.Flags().BoolVar(&q.filter.Pending, "pending", false, "Select invites nobody has accepted yet")
	cmd.Flags().StringVar(&q.filter.AcceptedBy, "accepted-by", "", "Select invites accepted by this login name")
	cmd.Flags().BoolVar(&all, "all", false, "Select every invite on the --device")
	addTailnetFlag(cmd, &q.tailnet)
	return cmd
}
