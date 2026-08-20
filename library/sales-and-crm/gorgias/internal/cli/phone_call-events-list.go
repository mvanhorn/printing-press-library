// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPhoneCallEventsListCmd(flags *rootFlags) *cobra.Command {
	var flagCallId string
	var flagCursor string
	var flagOrderBy string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "call-events-list",
		Short:       "List voice-call lifecycle events.",
		Example:     "  gorgias-pp-cli phone call-events-list",
		Annotations: map[string]string{"pp:endpoint": "phone.call-events-list", "pp:method": "GET", "pp:path": "/phone/voice-call-events", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/phone/voice-call-events"
			params := map[string]string{}
			if flagCursor != "" {
				params["cursor"] = flagCursor
			}
			if flagOrderBy != "" {
				params["order_by"] = flagOrderBy
			}
			if flagLimit != 0 {
				params["limit"] = fmt.Sprintf("%v", flagLimit)
			}
			if flagCallId != "" {
				params["call_id"] = flagCallId
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "phone", true, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
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

	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Pagination cursor from a prior response's meta.next_cursor.")
	cmd.Flags().StringVar(&flagOrderBy, "order-by", "", "Sort expression, e.g. created_datetime:desc.")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Page size (Gorgias hard cap: 100 for most resources).")

	cmd.Flags().StringVar(&flagCallId, "call-id", "", "Filter events to a single call.")

	return cmd
}
