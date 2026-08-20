// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newNovelNotificationInboxCmd(flags *rootFlags) *cobra.Command {
	var unread bool
	var since string
	var limit int
	var allHosts bool

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "See all notifications in one inbox, with host column",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would fetch notifications from /notifications")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"limit": fmt.Sprintf("%d", limit),
			}
			if unread {
				params["all"] = "false"
			} else {
				params["all"] = "true"
			}
			if since != "" {
				// If it looks like a duration (e.g. "24h"), convert to RFC3339
				if d, err := time.ParseDuration(since); err == nil {
					params["since"] = time.Now().Add(-d).UTC().Format(time.RFC3339)
				} else {
					params["since"] = since
				}
			}

			data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "notifications", "/notifications", params, nil, false, "page", "", "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
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

			var notifs []map[string]any
			if json.Unmarshal(data, &notifs) != nil || len(notifs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No notifications found.")
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tREPO\tSUBJECT\tTYPE\tUPDATED")
			for _, n := range notifs {
				id := fmt.Sprintf("%.0f", n["id"])
				repo := ""
				if r, ok := n["repository"].(map[string]any); ok {
					repo = fmt.Sprintf("%v", r["full_name"])
				}
				subject := ""
				stype := ""
				if s, ok := n["subject"].(map[string]any); ok {
					subject = truncate(fmt.Sprintf("%v", s["title"]), 50)
					stype = fmt.Sprintf("%v", s["type"])
				}
				updated := ""
				if u, ok := n["updated_at"].(string); ok && len(u) >= 10 {
					updated = u[:10]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", id, repo, subject, stype, updated)
			}
			_ = tw.Flush()

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(notifs), prov)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&unread, "unread", false, "Show only unread notifications")
	cmd.Flags().StringVar(&since, "since", "", "Show notifications updated after this time (RFC3339 or duration like 24h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of notifications to show")
	cmd.Flags().BoolVar(&allHosts, "all-hosts", false, "Aggregate notifications from all configured host profiles (not yet implemented)")
	_ = allHosts // multi-host aggregation requires SQLite sync; single-host API used otherwise

	return cmd
}
