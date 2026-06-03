// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCustomersListCmd(flags *rootFlags) *cobra.Command {
	var flagExternalId string
	var flagLanguage string
	var flagName string
	var flagTimezone string
	var flagViewId string
	var flagChannelType string
	var flagChannelAddress string
	var flagCursor string
	var flagOrderBy string
	var flagEmail string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List customers with pagination and optional filter params.",
		Example:     "  gorgias-pp-cli customers list",
		Annotations: map[string]string{"pp:endpoint": "customers.list", "pp:method": "GET", "pp:path": "/customers", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/customers"
			params := map[string]string{}
			if flagEmail != "" {
				params["email"] = fmt.Sprintf("%v", flagEmail)
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
			if flagExternalId != "" {
				params["external_id"] = flagExternalId
			}
			if flagLanguage != "" {
				params["language"] = flagLanguage
			}
			if flagName != "" {
				params["name"] = flagName
			}
			if flagTimezone != "" {
				params["timezone"] = flagTimezone
			}
			if flagViewId != "" {
				params["view_id"] = flagViewId
			}
			if flagChannelType != "" {
				params["channel_type"] = flagChannelType
			}
			if flagChannelAddress != "" {
				params["channel_address"] = flagChannelAddress
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "customers", true, path, params, nil)
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
	cmd.Flags().StringVar(&flagEmail, "email", "", "")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "")

	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Pagination cursor from a prior response's meta.next_cursor.")
	cmd.Flags().StringVar(&flagOrderBy, "order-by", "", "Sort expression, e.g. created_datetime:desc.")

	cmd.Flags().StringVar(&flagExternalId, "external-id", "", "Filter by customer external_id.")
	cmd.Flags().StringVar(&flagLanguage, "language", "", "Filter by ISO-639-1 language code.")
	cmd.Flags().StringVar(&flagName, "name", "", "Substring match on customer name.")
	cmd.Flags().StringVar(&flagTimezone, "timezone", "", "Filter by customer timezone.")
	cmd.Flags().StringVar(&flagViewId, "view-id", "", "Filter to customers in a saved view.")
	cmd.Flags().StringVar(&flagChannelType, "channel-type", "", "Filter by channel type (email/phone/etc.).")
	cmd.Flags().StringVar(&flagChannelAddress, "channel-address", "", "Filter by channel address.")

	return cmd
}
