// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newTicketsListCmd(flags *rootFlags) *cobra.Command {
	var flagViewId string
	var flagTrashed bool
	var flagExternalId string
	var flagRuleId string
	var flagTicketIds string
	var flagCursor string
	var flagOrderBy string
	var flagCustomerId string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tickets — scope by view, customer, rule, external ID, or specific IDs.",
		Long: "Returns paginated tickets, ordered by `order_by`. Server-side filters supported by\n" +
			"this endpoint: `view_id` (the canonical way to apply status/assignee/channel/tag\n" +
			"filters — define a saved view in the Gorgias UI and pass its ID), `customer_id`,\n" +
			"`external_id`, `rule_id`, `ticket_ids` (comma-separated), and `trashed`. There is\n" +
			"no direct filter for status/priority/channel/tag — Gorgias requires a view for\n" +
			"those. Pagination uses `cursor` + `limit` (max 100).",
		Example: `  gorgias-pp-cli tickets list
  gorgias-pp-cli tickets list --view-id 123456789 --limit 50
  gorgias-pp-cli tickets list --customer-id 123456789`,
		Annotations: map[string]string{"pp:endpoint": "tickets.list", "pp:method": "GET", "pp:path": "/tickets", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/tickets"
			params := map[string]string{}
			if flagCustomerId != "" {
				params["customer_id"] = fmt.Sprintf("%v", flagCustomerId)
			}
			if flagLimit != 0 {
				params["limit"] = fmt.Sprintf("%v", flagLimit)
			}
			if flagCursor != "" {
				params["cursor"] = flagCursor
			}
			if flagOrderBy != "" {
				params["order_by"] = flagOrderBy
			}
			if flagViewId != "" {
				params["view_id"] = flagViewId
			}
			if flagTrashed {
				params["trashed"] = "true"
			}
			if flagExternalId != "" {
				params["external_id"] = flagExternalId
			}
			if flagRuleId != "" {
				params["rule_id"] = flagRuleId
			}
			if flagTicketIds != "" {
				params["ticket_ids"] = flagTicketIds
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "tickets", true, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Honor --limit when the API accepts but ignores ?limit=N.
			data = truncateJSONArray(data, flagLimit)
			// Print provenance to stderr for human-facing output
			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}
			// For JSON output, wrap with provenance envelope before passing through flags.
			// --select wins over --compact when both are set; --compact only runs when
			// no explicit fields were requested. Explicit format flags (--csv, --quiet,
			// --plain) opt out of the auto-JSON path so piped consumers that asked for
			// a non-JSON format reach the standard pipeline below.
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			// For all other output modes (table, csv, plain, quiet), use the standard pipeline
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagCustomerId, "customer-id", "", "Filter to tickets belonging to this customer id.")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Max items per page (Gorgias caps at 100).")

	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Pagination cursor from a prior response's meta.next_cursor.")
	cmd.Flags().StringVar(&flagOrderBy, "order-by", "", "Sort expression, e.g. created_datetime:desc.")

	cmd.Flags().StringVar(&flagViewId, "view-id", "", "Filter to tickets in a saved view.")
	cmd.Flags().BoolVar(&flagTrashed, "trashed", false, "Include trashed tickets.")
	cmd.Flags().StringVar(&flagExternalId, "external-id", "", "Filter by customer external_id.")
	cmd.Flags().StringVar(&flagRuleId, "rule-id", "", "Filter to tickets that matched this rule.")
	cmd.Flags().StringVar(&flagTicketIds, "ticket-ids", "", "Comma-separated ticket IDs.")

	return cmd
}
