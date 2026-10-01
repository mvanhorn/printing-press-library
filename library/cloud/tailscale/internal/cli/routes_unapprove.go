// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call runRouteChange in routes_approve.go performs the GET/POST /device/{id}/routes calls

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelRoutesUnapproveCmd(flags *rootFlags) *cobra.Command {
	opts := routeChangeOpts{action: "unapprove"}
	cmd := &cobra.Command{
		Use:   "unapprove <device> [cidr...]",
		Short: "Remove approval for one route or the exit-node pair while leaving every other approved route in place.",
		Long: strings.TrimSpace(`
Use this command to remove approval for specific routes or the exit node on one
device while keeping the rest. Do NOT use this command to find pending routes;
use 'routes overview' instead.

The Tailscale API has no remove-one-route call: it replaces the whole enabled
set. This command reads the current set, sends it minus the requested routes,
re-reads right before writing and aborts if the set changed, then verifies.
--exit-node removes both 0.0.0.0/0 and ::/0.

Without --yes it prints the plan and changes nothing. --dry-run always plans.
As an MCP tool it is plan-only (--yes is not accepted there).`),
		Example: strings.Trim(`
  tailscale-pp-cli routes unapprove self --exit-node
  tailscale-pp-cli routes unapprove home-mac 192.168.1.0/24 --yes`, "\n"),
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
	cmd.Flags().BoolVar(&opts.exitNode, "exit-node", false, "Remove the exit-node route pair (0.0.0.0/0 and ::/0)")
	addTailnetFlag(cmd, &opts.tailnet)
	return cmd
}
