// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type routeChangeView struct {
	Action       string            `json:"action"`
	DryRun       bool              `json:"dry_run,omitempty"`
	Selector     string            `json:"selector"`
	Device       deviceRef         `json:"device"`
	Mode         string            `json:"mode"`
	Applied      bool              `json:"applied"`
	Plan         tsadmin.RoutePlan `json:"plan"`
	EnabledAfter []string          `json:"enabled_after_apply,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
	Next         string            `json:"next,omitempty"`
}

type routeChangeOpts struct {
	action            string // "approve" or "unapprove"
	tailnet           string
	exitNode          bool
	allAdvertised     bool
	allowUnadvertised bool
}

func newNovelRoutesApproveCmd(flags *rootFlags) *cobra.Command {
	opts := routeChangeOpts{action: "approve"}
	cmd := &cobra.Command{
		Use:   "approve <device> [cidr...]",
		Short: "Approve a subnet route or exit node on one device without dropping the routes it already has.",
		Long: strings.TrimSpace(`
Use this command to approve subnet routes or an exit node on one device while
keeping its other approved routes. Do NOT use this command to find which routes
are waiting for approval across the tailnet; use 'routes overview' instead.

The Tailscale API replaces a device's whole enabled-route set on every write.
This command reads the current set, adds the requested routes, re-reads right
before writing and aborts if the set changed, then reads the set again after
the write to verify it. The routes endpoint has no conditional write, so a
change someone else makes between that last read and the write can still be
replaced; the window is one request round trip.
--exit-node means the pair 0.0.0.0/0 and ::/0. Routes the device does not
advertise are refused unless --allow-unadvertised is set.

Without --yes it prints the plan and changes nothing. --dry-run always plans.
As an MCP tool it is plan-only (--yes is not accepted there).
<device> accepts self, a hostname, MagicDNS name, Tailscale IP, or nodeId.`),
		Example: strings.Trim(`
  tailscale-pp-cli routes approve self --all-advertised
  tailscale-pp-cli routes approve home-mac --exit-node
  tailscale-pp-cli routes approve home-mac 192.168.1.0/24 --yes
  tailscale-pp-cli routes approve nas --all-advertised --agent`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			// Over MCP these tools are plan-only: --yes is blocked, so the
			// read-only hint is accurate. Writes need the CLI with --yes.
			"mcp:read-only":   "true",
			"mcp:write-flags": "yes",
			"pp:happy-args":   "device=self",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRouteChange(cmd, flags, opts, args)
		},
	}
	cmd.Flags().BoolVar(&opts.exitNode, "exit-node", false, "Approve the exit-node route pair (0.0.0.0/0 and ::/0)")
	cmd.Flags().BoolVar(&opts.allAdvertised, "all-advertised", false, "Approve every route the device advertises that is not yet enabled")
	cmd.Flags().BoolVar(&opts.allowUnadvertised, "allow-unadvertised", false, "Approve routes the device does not advertise yet (they stay inactive until advertised)")
	addTailnetFlag(cmd, &opts.tailnet)
	return cmd
}

func runRouteChange(cmd *cobra.Command, flags *rootFlags, opts routeChangeOpts, args []string) error {
	if len(args) == 0 && cmd.Flags().NFlag() == 0 {
		return cmd.Help()
	}
	if dryRunOK(flags) && (len(args) == 0 || cliutil.IsVerifyEnv()) {
		return writeDryRun(cmd.OutOrStdout(), flags, "routes "+opts.action)
	}
	if len(args) == 0 {
		_ = cmd.Usage()
		return usageErr(errors.New("<device> is required (self, hostname, MagicDNS name, Tailscale IP, or nodeId)"))
	}
	requested, err := tsadmin.NormalizeRoutes(args[1:])
	if err != nil {
		return usageErr(err)
	}
	if opts.exitNode {
		requested = append(requested, tsadmin.ExitRoutes()...)
	}
	if len(requested) == 0 && !(opts.action == "approve" && opts.allAdvertised) {
		_ = cmd.Usage()
		if opts.action == "approve" {
			return usageErr(errors.New("name at least one CIDR, --exit-node, or --all-advertised"))
		}
		return usageErr(errors.New("name at least one CIDR or --exit-node"))
	}

	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := tsLiveClient(ctx, flags)
	if err != nil {
		return err
	}
	devs, err := fetchDevicesLive(ctx, c, opts.tailnet, false)
	if err != nil {
		return err
	}
	dev, err := resolveDevice(ctx, devs, args[0])
	if err != nil {
		return err
	}
	id := dev.PreferredID()
	current, err := fetchDeviceRoutes(ctx, c, id)
	if err != nil {
		return err
	}
	if opts.allAdvertised {
		requested = append(requested, current.AdvertisedRoutes...)
	}

	var plan tsadmin.RoutePlan
	var verifyErr error
	view := routeChangeView{Action: opts.action, DryRun: flags.dryRun, Selector: args[0], Device: refOf(dev)}
	if opts.action == "approve" {
		plan = tsadmin.PlanApprove(current.AdvertisedRoutes, current.EnabledRoutes, requested)
		if len(plan.Unadvertised) > 0 {
			if !opts.allowUnadvertised {
				return usageErr(fmt.Errorf("%s does not advertise %s; advertise it on the device first (tailscale set --advertise-routes=...) or pass --allow-unadvertised", dev.Label(), strings.Join(plan.Unadvertised, ", ")))
			}
			view.Warnings = append(view.Warnings, fmt.Sprintf("not advertised yet, will stay inactive until advertised: %s", strings.Join(plan.Unadvertised, ", ")))
		}
		if opts.exitNode && plan.AddedExitRoutes() == 1 {
			view.Warnings = append(view.Warnings, "only one half of the exit-node pair was missing; the device was a partial exit node before this change")
		}
		if plan.NoChange {
			if len(requested) == 0 {
				view.Warnings = append(view.Warnings, "the device advertises no routes, so --all-advertised has nothing to approve")
			} else {
				view.Warnings = append(view.Warnings, fmt.Sprintf("already enabled: %s", strings.Join(uniqueStrings(requested), ", ")))
			}
		}
	} else {
		plan = tsadmin.PlanUnapprove(current.AdvertisedRoutes, current.EnabledRoutes, requested)
		if len(plan.NotEnabled) > 0 {
			view.Warnings = append(view.Warnings, fmt.Sprintf("already not enabled: %s", strings.Join(plan.NotEnabled, ", ")))
		}
	}
	view.Plan = plan

	switch {
	case plan.NoChange:
		view.Mode = "no-change"
	case tsPlanOnly(flags):
		view.Mode = "plan"
		view.Next = tsApplyHint
	default:
		fresh, err := fetchDeviceRoutes(ctx, c, id)
		if err != nil {
			return err
		}
		if !tsadmin.SameRouteSet(fresh.EnabledRoutes, current.EnabledRoutes) {
			return apiErr(fmt.Errorf("enabled routes on %s changed while planning (now %s); nothing was written, re-run to plan against the current state", dev.Label(), joinOrDash(fresh.EnabledRoutes)))
		}
		if err := setDeviceRoutes(ctx, c, id, plan.After); err != nil {
			return err
		}
		view.Mode = "applied"
		view.Applied = true
		// Verify against a fresh read, not the POST response, so the report
		// reflects what the device's routes actually are now.
		after, err := fetchDeviceRoutes(ctx, c, id)
		switch {
		case err != nil:
			msg := fmt.Sprintf("the write was sent, but reading the routes back failed (%v); check the device before retrying", err)
			view.Warnings = append(view.Warnings, msg)
			verifyErr = apiErr(errors.New(msg))
		case !tsadmin.SameRouteSet(after.EnabledRoutes, plan.After):
			view.EnabledAfter = after.EnabledRoutes
			msg := fmt.Sprintf("the write was sent, but the device now reports enabled routes %s instead of %s; check the device before retrying", joinOrDash(after.EnabledRoutes), joinOrDash(plan.After))
			view.Warnings = append(view.Warnings, msg)
			verifyErr = apiErr(errors.New(msg))
		default:
			view.EnabledAfter = after.EnabledRoutes
		}
	}
	return emitOrRender(cmd, flags, view, func() { renderRouteChange(cmd, view) }, verifyErr)
}

// uniqueStrings drops repeats while keeping first-occurrence order.
func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func setDeviceRoutes(ctx context.Context, c *client.Client, id string, routes []string) error {
	if routes == nil {
		routes = []string{}
	}
	if _, _, err := c.Post(ctx, devicePath(id, "/routes"), map[string]any{"routes": routes}); err != nil {
		return classifyAPIErrorOnly(err)
	}
	return nil
}

func renderRouteChange(cmd *cobra.Command, v routeChangeView) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "routes %s on %s (%s): %s\n", v.Action, v.Device.Hostname, v.Device.NodeID, v.Mode)
	fmt.Fprintf(w, "  enabled before: %s\n", joinOrDash(v.Plan.Before))
	fmt.Fprintf(w, "  enabled after:  %s\n", joinOrDash(v.Plan.After))
	if len(v.Plan.Added) > 0 {
		fmt.Fprintf(w, "  + %s\n", strings.Join(v.Plan.Added, ", "))
	}
	if len(v.Plan.Removed) > 0 {
		fmt.Fprintf(w, "  - %s\n", strings.Join(v.Plan.Removed, ", "))
	}
	for _, warn := range v.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
	if v.Next != "" {
		fmt.Fprintf(w, "  %s\n", v.Next)
	}
}
