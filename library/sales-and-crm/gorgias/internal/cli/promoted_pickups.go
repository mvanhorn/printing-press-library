// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPickupsPromotedCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "pickups <id>",
		Short: "Delete a Gorgias pickup record by ID (single destructive endpoint).",
		Long: "Hard-deletes one pickup record identified by `<id>`. This is the only\n" +
			"operation Gorgias's REST API exposes on the /pickups resource, so the\n" +
			"command lives as a leaf verb at the root rather than under a `pickups`\n" +
			"parent. Pass `--ignore-missing` to make the delete idempotent.",
		Example:     "  gorgias-pp-cli pickups 123456789 --ignore-missing",
		Annotations: map[string]string{"pp:endpoint": "pickups.delete", "pp:method": "DELETE", "pp:path": "/pickups/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/pickups/{id}"
			if len(args) < 1 {
				// JSON envelope: {error, usage}. Written first; the
				// usageErr return preserves exit code 2 across modes.
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "id is required",
						"usage": fmt.Sprintf("%s <%s>", cmd.CommandPath(), "id"),
					}, flags); printErr != nil {
						return printErr
					}
				}
				return usageErr(fmt.Errorf("id is required\nUsage: %s <%s>", cmd.CommandPath(), "id"))
			}
			path = replacePathParam(path, "id", args[0])
			data, _, err := c.Delete(path)
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

	// Wire sibling endpoints and sub-resources as subcommands

	return cmd
}
