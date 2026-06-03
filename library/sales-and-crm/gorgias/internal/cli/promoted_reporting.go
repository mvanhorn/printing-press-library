// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newReportingPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagCursor string
	var flagLimit int
	var bodyQuery string

	cmd := &cobra.Command{
		Use:         "reporting",
		Short:       "Run a Gorgias analytics query (single POST to /reporting/stats — pass --query <statement>).",
		Example:     "  gorgias-pp-cli reporting --query '{\"metric\":\"tickets.count\"}'",
		Annotations: map[string]string{"pp:endpoint": "reporting.stats", "pp:method": "POST", "pp:path": "/reporting/stats", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("query") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "query")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/reporting/stats"
			// HasStore + non-GET falls through to a live API call here
			// rather than through resolveRead (GET-only internally); a
			// body-aware cached read helper is filed as #425 for when a
			// second store-backed POST-search consumer ships.
			body := map[string]any{}
			if bodyQuery != "" {
				body["query"] = bodyQuery
			}
			data, _, err := c.Post(path, body)
			prov := attachFreshness(DataProvenance{Source: "live"}, flags)
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
	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Value indicating your position in the list of all analytics results.")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of analytics results to return.")
	cmd.Flags().StringVar(&bodyQuery, "query", "", "The statistics query, dependent on the requested metric.")

	// Wire sibling endpoints and sub-resources as subcommands

	return cmd
}
