// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newTicketSearchPromotedCmd(flags *rootFlags) *cobra.Command {
	var bodyType string
	var bodyQuery string
	var bodySize int

	cmd := &cobra.Command{
		Use:   "ticket-search",
		Short: "Search the Gorgias /search index (customers, agents, tags, teams, integrations).",
		Long: "Calls POST /search against the live Gorgias index.\n\n" +
			"Despite the legacy command name, this endpoint does NOT index tickets or messages —\n" +
			"it covers customer_profile, agent_profile, tag, team, and integration. For full-text\n" +
			"search over tickets/messages, sync first and use `gorgias-pp-cli search --data-source local`\n" +
			"(FTS5 over the synced mirror) or `gorgias-pp-cli sql`.\n\n" +
			"The `--type` flag picks the index; `--query` is the search string; `--size` caps results.",
		Example: `  # Find a customer by email
  gorgias-pp-cli ticket-search --type customer_profile --query customer-lookup-placeholder

  # Find a tag by name
  gorgias-pp-cli ticket-search --type tag --query "refund"`,
		Annotations: map[string]string{"pp:endpoint": "ticket-search.query", "pp:method": "POST", "pp:path": "/search", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("type") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "type")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/search"
			// HasStore + non-GET falls through to a live API call here
			// rather than through resolveRead (GET-only internally); a
			// body-aware cached read helper is filed as #425 for when a
			// second store-backed POST-search consumer ships.
			body := map[string]any{}
			if bodyType != "" {
				body["type"] = bodyType
			}
			if bodyQuery != "" {
				body["query"] = bodyQuery
			}
			if bodySize != 0 {
				body["size"] = bodySize
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
	cmd.Flags().StringVar(&bodyType, "type", "", "Index to search (one of: customer_profile, agent_profile, tag, team, integration). Required.")
	cmd.Flags().StringVar(&bodyQuery, "query", "", "Text query (example: customer-lookup-placeholder).")
	cmd.Flags().IntVar(&bodySize, "size", 0, "Maximum number of results returned (default: 10 server-side; example: 10).")

	// Wire sibling endpoints and sub-resources as subcommands

	return cmd
}
