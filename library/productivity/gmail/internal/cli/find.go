// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: live Gmail search with hydrated metadata. Absorbs gws search
// and MCP search_emails, beating both with batched metadata and agent output.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFindCmd(flags))
	})
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "find <gmail-query>",
		Short: "Live Gmail search with the same operators as the Gmail search box",
		Long: `Searches the live mailbox with Gmail query syntax (from:, subject:,
is:unread, has:attachment, newer_than:2d, larger:5M, ...) and hydrates
sender/subject/date metadata in batched requests.`,
		Example: strings.Trim(`
  gmail-pp-cli find "is:unread newer_than:2d"
  gmail-pp-cli find "from:stripe.com subject:invoice" --limit 20
  gmail-pp-cli find "has:attachment larger:5M" --json --select id,from,subject`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "gmail-query=is:unread",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "live Gmail search")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a Gmail query argument is required"))
			}
			query := strings.Join(args, " ")
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			msgs, skipped, err := gmailSearchMetadata(cmd.Context(), c, query, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			warnSkipped(cmd, skipped)
			rows := toMessageRows(msgs)
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no messages match the query")
				return nil
			}
			printMessageRows(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum messages to return")
	return cmd
}
