// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCustomFieldsListCmd(flags *rootFlags) *cobra.Command {
	var flagObjectType string
	var flagSearch string
	var flagArchived bool
	var flagLimit int
	var flagCursor string
	var flagOrderBy string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List custom field definitions for a single `object_type`.",
		Example:     "  gorgias-pp-cli custom-fields list --object-type example-value",
		Annotations: map[string]string{"pp:endpoint": "custom-fields.list", "pp:method": "GET", "pp:path": "/custom-fields", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("object-type") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "object-type")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/custom-fields"
			params := map[string]string{}
			if flagObjectType != "" {
				params["object_type"] = fmt.Sprintf("%v", flagObjectType)
			}
			if flagSearch != "" {
				params["search"] = fmt.Sprintf("%v", flagSearch)
			}
			if flagArchived != false {
				params["archived"] = fmt.Sprintf("%v", flagArchived)
			}
			if flagLimit != 0 {
				params["limit"] = fmt.Sprintf("%v", flagLimit)
			}
			if flagCursor != "" {
				params["cursor"] = fmt.Sprintf("%v", flagCursor)
			}
			if flagOrderBy != "" {
				params["order_by"] = fmt.Sprintf("%v", flagOrderBy)
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "custom-fields", true, path, params, nil)
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
	cmd.Flags().StringVar(&flagObjectType, "object-type", "", "Filter to a single resource type: Ticket or Customer. REQUIRED by Gorgias — calls without it return 400.")
	cmd.Flags().StringVar(&flagSearch, "search", "", "Substring search across custom-field names.")
	cmd.Flags().BoolVar(&flagArchived, "archived", false, "Include archived fields when true.")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Page size (Gorgias hard cap: 100).")
	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Pagination cursor from a prior response's meta.next_cursor.")
	cmd.Flags().StringVar(&flagOrderBy, "order-by", "", "Sort expression, e.g. 'created_datetime:desc'.")

	return cmd
}
