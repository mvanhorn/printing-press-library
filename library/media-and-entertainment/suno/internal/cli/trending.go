// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func trendingRunE(flags *rootFlags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}

		path := "/api/trending/"
		params := map[string]string{}
		data, prov, err := resolveRead(cmd.Context(), c, flags, "trending", false, path, params, nil, cmd.ErrOrStderr())
		if err != nil {
			return classifyAPIError(err, flags)
		}
		data = extractResponseData(data)

		if wantsHumanTable(cmd.OutOrStdout(), flags) {
			var countItems []json.RawMessage
			if json.Unmarshal(data, &countItems) != nil {
				countItems = []json.RawMessage{data}
			}
			printProvenance(cmd, len(countItems), prov)
		}
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
	}
}

func newTrendingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "trending",
		Short:       "Show trending clips",
		Long:        "Show trending clips.",
		Example:     "  suno-pp-cli trending",
		Annotations: map[string]string{"pp:endpoint": "trending.list", "pp:method": "GET", "pp:path": "/api/trending/", "mcp:read-only": "true"},
		RunE:        trendingRunE(flags),
	}

	cmd.AddCommand(newTrendingListCmd(flags))
	return cmd
}

func newTrendingListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "Show trending clips",
		Long:        "Show trending clips.",
		Example:     "  suno-pp-cli trending list",
		Annotations: map[string]string{"pp:endpoint": "trending.list", "pp:method": "GET", "pp:path": "/api/trending/", "mcp:read-only": "true"},
		RunE:        trendingRunE(flags),
	}
}
