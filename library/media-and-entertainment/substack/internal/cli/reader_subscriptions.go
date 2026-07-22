// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(reader-subscriptions-list): hand-authored; see .printing-press-patches/.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newReaderSubscriptionsCmd(flags *rootFlags) *cobra.Command {
	var flagFilter string
	var flagCursor string
	var flagLimit string

	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "List all publications you subscribe to as a reader",
		Long: `Lists every publication your Substack account is subscribed to — both free and paid.

This is the reader-side subscription list: publications you follow, not your own publication's subscribers.
Auth is via your session cookie (substack.sid). Run 'substack-pp-cli auth login --chrome' if needed.`,
		Example: `  # All your subscriptions as JSON
  substack-pp-cli reader subscriptions --json

  # Compact — handle, name, tier only
  substack-pp-cli reader subscriptions --agent --select subscription_id,publication.handle,publication.name,membership_state

  # Paid subscriptions only
  substack-pp-cli reader subscriptions --filter paid --agent`,
		Annotations: map[string]string{
			"pp:endpoint":   "reader.subscriptions",
			"pp:method":     "GET",
			"pp:path":       "/reader/subscriptions",
			"pp:novel":      "reader subscriptions",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/reader/subscriptions"
			params := map[string]string{}
			if flagFilter != "" {
				params["filter"] = flagFilter
			}
			if flagCursor != "" {
				params["cursor"] = flagCursor
			}
			if flagLimit != "" {
				params["limit"] = flagLimit
			}

			data, prov, err := resolveReadWithStrategy(cmd.Context(), c, flags, "auto", "reader", false, path, params, nil, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
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
						fmt.Fprintf(os.Stderr, "\nShowing %d results. Add --limit, --json --select, or --filter to narrow.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&flagFilter, "filter", "", "Filter by subscription tier: 'paid', 'free', or leave empty for all")
	cmd.Flags().StringVar(&flagCursor, "cursor", "", "Opaque pagination cursor from prior response")
	cmd.Flags().StringVar(&flagLimit, "limit", "", "Maximum number of subscriptions to return")

	return cmd
}
