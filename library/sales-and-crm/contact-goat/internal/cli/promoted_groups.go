// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/spf13/cobra"
)

// newGroupsCmd wires `contact-goat-pp-cli groups` and the sub-`get` command.
// The endpoints aren't in the sniffed OpenAPI spec — they're added here for
// parity with the hpn web CLI. The assumption is that /api/groups mirrors
// the /api/friends/list pattern. If the server returns 404 we surface a
// stub message so the user can ignore the command without noise.
func newGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List your Happenstance groups (hpn-CLI parity, not in sniffed spec)",
		Long: `List your Happenstance groups.

Note: this endpoint is not in the sniffed OpenAPI spec — it is a best-effort
mirror of the hpn web CLI. The path /api/groups is inferred from the
/api/friends/list pattern. If the Happenstance API returns HTTP 404, the
command will report that the endpoint is still a stub.`,
		Example: `  contact-goat-pp-cli groups
  contact-goat-pp-cli groups --json
  contact-goat-pp-cli groups get grp_abc123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClientRequireCookies("happenstance")
			if err != nil {
				return err
			}
			return runGroupsList(c, cmd, flags)
		},
	}
	cmd.AddCommand(newGroupsGetCmd(flags))
	return cmd
}

func newGroupsGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get details for a single Happenstance group by id",
		Example: `  contact-goat-pp-cli groups get grp_abc123
  contact-goat-pp-cli groups get grp_abc123 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClientRequireCookies("happenstance")
			if err != nil {
				return err
			}
			path := "/api/groups/" + args[0]
			data, prov, err := resolveRead(c, flags, "groups", false, path, nil)
			if err != nil {
				return mapGroupsAPIError(err)
			}
			data = extractResponseData(data)
			printProvenance(cmd, 1, prov)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				wrapped, werr := wrapWithProvenance(data, prov)
				if werr != nil {
					return werr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func runGroupsList(c *client.Client, cmd *cobra.Command, flags *rootFlags) error {
	path := "/api/groups"
	data, prov, err := resolveRead(c, flags, "groups", true, path, map[string]string{})
	if err != nil {
		return mapGroupsAPIError(err)
	}
	data = extractResponseData(data)

	var items []json.RawMessage
	if json.Unmarshal(data, &items) != nil {
		items = []json.RawMessage{data}
	}
	printProvenance(cmd, len(items), prov)

	if flags.csv {
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		wrapped, werr := wrapWithProvenance(data, prov)
		if werr != nil {
			return werr
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var rows []map[string]any
		if json.Unmarshal(data, &rows) == nil && len(rows) > 0 {
			return printAutoTable(cmd.OutOrStdout(), rows)
		}
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

// mapGroupsAPIError translates a 404 from the inferred /api/groups endpoint
// into a friendlier "stub" message so users don't mistake it for a general
// outage. All other errors pass through classifyAPIError.
func mapGroupsAPIError(err error) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return notFoundErr(fmt.Errorf(
			"groups endpoint %s not implemented by Happenstance yet (HTTP 404). "+
				"This command is a hpn-CLI-parity stub — see `contact-goat-pp-cli groups --help`",
			apiErr.Path))
	}
	return classifyAPIError(err)
}
