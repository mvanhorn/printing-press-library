// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newMessagesPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagCursor string
	var flagLimit int
	var flagOrderBy string
	var flagTicketId string

	cmd := &cobra.Command{
		Use:         "messages",
		Short:       "List messages account-wide (single endpoint, paginated). Use --ticket-id to scope to one ticket.",
		Example:     "  gorgias-pp-cli messages --ticket-id 123456789 --limit 20",
		Annotations: map[string]string{"pp:endpoint": "messages.list", "pp:method": "GET", "pp:path": "/messages", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/messages"
			params := map[string]string{}
			if flagCursor != "" {
				params["cursor"] = flagCursor
			}
			if flagLimit != 0 {
				params["limit"] = fmt.Sprintf("%v", flagLimit)
			}
			if flagOrderBy != "" {
				params["order_by"] = flagOrderBy
			}
			if flagTicketId != "" {
				params["ticket_id"] = flagTicketId
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "messages", true, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Unwrap API response envelopes (e.g. {"status":"success","data":[...]})
			// so output helpers see the inner data, not the wrapper.
			data = extractResponseData(data)

			// Print provenance to stderr
			{
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					// Single object, not an array
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}
			// For JSON output, wrap with provenance envelope. --select wins over
			// --compact when both are set; --compact only runs when no explicit
			// fields were requested. Explicit format flags (--csv, --quiet, --plain)
			// opt out of the auto-JSON path so piped consumers that asked for a
			// non-JSON format reach the standard pipeline below.
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

	// Wire sibling endpoints and sub-resources as subcommands

	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Pagination cursor from a prior response's meta.next_cursor.")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Page size (Gorgias hard cap: 100).")
	cmd.Flags().StringVar(&flagOrderBy, "order-by", "", "Sort expression, e.g. created_datetime:desc.")
	cmd.Flags().StringVar(&flagTicketId, "ticket-id", "", "Filter messages to a single ticket.")

	return cmd
}
