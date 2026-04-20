// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// api_hpn.go nests the Happenstance public-API command tree under the
// existing top-level `api` parent. The full path is:
//
//   contact-goat-pp-cli api hpn <subcommand>
//
// Subcommands live in sibling files (api_hpn_search.go, api_hpn_research.go,
// api_hpn_groups.go, api_hpn_usage.go). This file is the registration seam
// only — no per-endpoint behavior lives here.
//
// Why nested under `api` rather than top-level: the parent `api` command
// already exists as the generated discovery shim; making `hpn` a child of
// it keeps the Happenstance public-REST surface namespaced and avoids
// colliding with the cookie-surface commands at the root level (`hp`,
// `groups`, `user`, `dossier`, etc.). Users and agents discover the
// surface via `contact-goat-pp-cli api hpn --help`.

package cli

import (
	"github.com/spf13/cobra"
)

// newAPIHpnCmd returns the `api hpn` parent command. Behavior beyond
// grouping subcommands is intentionally absent: invoking `api hpn` alone
// prints the subcommand index via cobra's default help.
func newAPIHpnCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hpn",
		Short: "Happenstance public REST API (bearer-auth, credit-priced)",
		Long: `Call the Happenstance public REST API (https://api.happenstance.ai/v1)
from the terminal. Bearer-auth via HAPPENSTANCE_API_KEY; provision and
rotate keys at https://happenstance.ai/settings/api-keys.

Costs: search costs 2 credits, research costs 1 credit on completion,
find-more costs 2 credits. Groups, usage, and user are free probes.

Every credit-spending subcommand prints a cost preview and refuses to
spend when --budget is set and would be exceeded. Pass --yes to skip
the confirmation gate (required for agents).

This surface is the bearer-auth peer of the cookie-sniff surface that
backs ` + "`hp people`" + `, ` + "`coverage`" + `, etc. The two surfaces coexist; the
auto router prefers cookie (free quota) and falls back to bearer (paid
credits) when cookie quota is exhausted. Use ` + "`--source api`" + ` on the
search-using commands to opt into the bearer surface explicitly.`,
		Example: `  # Free probes
  contact-goat-pp-cli api hpn user
  contact-goat-pp-cli api hpn usage
  contact-goat-pp-cli api hpn groups list

  # Search (costs 2 credits)
  contact-goat-pp-cli api hpn search "VPs at NBA" --yes

  # Deep research (costs 1 credit on completion)
  contact-goat-pp-cli api hpn research "Brian Chesky, CEO at Airbnb" --yes`,
	}

	cmd.AddCommand(newAPIHpnSearchCmd(flags))
	cmd.AddCommand(newAPIHpnResearchCmd(flags))
	cmd.AddCommand(newAPIHpnGroupsCmd(flags))
	cmd.AddCommand(newAPIHpnUsageCmd(flags))
	cmd.AddCommand(newAPIHpnUserCmd(flags))

	return cmd
}
