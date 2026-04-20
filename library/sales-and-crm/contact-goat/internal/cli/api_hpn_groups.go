// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// api_hpn_groups.go: `api hpn groups list` and `api hpn groups get`.
// Both are free probes (no credit cost), so no budget gate or cost
// preview applies.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

func newAPIHpnGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List or fetch Happenstance groups (free probe)",
		Long: `Calls /v1/groups and /v1/groups/{id}. Groups are the named cohorts
the user has access to (e.g. an alumni network or company list).
Member names from a singular group fetch can be passed back into
` + "`api hpn search`" + ` as @-mentions to scope the search.`,
	}
	cmd.AddCommand(newAPIHpnGroupsListCmd(flags))
	cmd.AddCommand(newAPIHpnGroupsGetCmd(flags))
	return cmd
}

func newAPIHpnGroupsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all Happenstance groups the caller can access (free)",
		Example: `  contact-goat-pp-cli api hpn groups list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			groups, err := c.Groups(cmd.Context())
			if err != nil {
				return classifyHpnError(err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"source": "api",
					"count":  len(groups),
					"groups": groups,
				})
			}
			if len(groups) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no groups available")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("ID")+"\t"+bold("NAME")+"\t"+bold("MEMBERS"))
			for _, g := range groups {
				fmt.Fprintf(tw, "%s\t%s\t%d\n", g.Id, g.Name, g.MemberCount)
			}
			return tw.Flush()
		},
	}
}

func newAPIHpnGroupsGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "get <group_id>",
		Short:   "Get a single Happenstance group by id, including members (free)",
		Example: `  contact-goat-pp-cli api hpn groups get grp_abc123`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID := strings.TrimSpace(args[0])
			if groupID == "" {
				return usageErr(fmt.Errorf("group_id is empty"))
			}
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			g, err := c.Group(cmd.Context(), groupID)
			if err != nil {
				return classifyHpnError(err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"source": "api",
					"group":  g,
					// Pre-baked @-mention strings for each member, so a
					// caller copying-and-pasting into `api hpn search` does
					// not have to know the FormatGroupMention quoting
					// rules. The api package owns the formatter; we just
					// surface its output.
					"member_mentions": formatGroupMembersForMention(g),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) - %d members\n\n", g.Name, g.Id, g.MemberCount)
			if len(g.Members) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no members on file")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("NAME")+"\t"+bold("MENTION"))
			for _, m := range g.Members {
				fmt.Fprintf(tw, "%s\t%s\n", m.Name, api.FormatGroupMention(m.Name))
			}
			return tw.Flush()
		},
	}
}

// formatGroupMembersForMention pre-computes the @-mention strings for
// every member of a group, so JSON consumers do not have to recompute
// the quoting/escaping rules client-side. Mirrors the table rendering
// path's column.
func formatGroupMembersForMention(g api.Group) []string {
	out := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		mention := api.FormatGroupMention(m.Name)
		if mention != "" {
			out = append(out, mention)
		}
	}
	return out
}
