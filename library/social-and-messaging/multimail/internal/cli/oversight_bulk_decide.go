// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source auto

func newNovelOversightBulkDecideCmd(flags *rootFlags) *cobra.Command {
	var flagApprove bool
	var flagReject bool
	var flagMailbox string
	var flagSender string
	var flagOlderThan string
	var flagReason string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "bulk-decide",
		Short: "Approve or reject all pending emails matching a filter (by mailbox, sender, or age) in one command.",
		Long: `Bulk decide filters synced pending emails by mailbox, sender, or age
predicates, then calls the live decide API for each match. No batch decide
API exists — this command provides the bulk operation.

Requires at least one filter flag to prevent accidental mass decisions.
Use --approve or --reject to specify the action.`,
		Annotations: map[string]string{"pp:endpoint": "oversight.bulk-decide", "pp:method": "POST"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if !flagApprove && !flagReject {
				return fmt.Errorf("specify --approve or --reject")
			}
			if flagApprove && flagReject {
				return fmt.Errorf("cannot use --approve and --reject together")
			}

			// Require at least one filter to prevent accidental mass operations
			hasFilter := flagMailbox != "" || flagSender != "" || flagOlderThan != ""
			if !hasFilter {
				return fmt.Errorf("at least one filter required (--mailbox, --sender, or --older-than) to prevent accidental mass decisions")
			}

			action := "approve"
			if flagReject {
				action = "reject"
			}

			ctx := cmd.Context()

			// First, get pending emails from local store or live
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch all pending pages from live API — cursor pagination
			// ensures bulk-decide processes the full queue, not just page 1.
			pendingData, err := paginatedGet(ctx, c, "/v1/oversight/pending", nil, nil,
				true, "cursor", "cursor", "limit", "next_cursor", "has_more")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var pending []map[string]any
			if err := json.Unmarshal(pendingData, &pending); err != nil {
				// Try wrapped
				var wrapper struct {
					Data []map[string]any `json:"data"`
				}
				if err2 := json.Unmarshal(pendingData, &wrapper); err2 != nil {
					return fmt.Errorf("parsing pending response: %w", err)
				}
				pending = wrapper.Data
			}

			// Parse older-than duration
			var olderThanCutoff time.Time
			if flagOlderThan != "" {
				dur, err := time.ParseDuration(flagOlderThan)
				if err != nil {
					return fmt.Errorf("invalid --older-than value %q (use Go duration like 1h, 30m, 24h): %w", flagOlderThan, err)
				}
				olderThanCutoff = time.Now().Add(-dur)
			}

			// Filter pending
			var matched []map[string]any
			for _, email := range pending {
				if flagMailbox != "" {
					mailboxID, _ := email["mailbox_id"].(string)
					if mailboxID != flagMailbox {
						continue
					}
				}
				if flagSender != "" {
					from, _ := email["from"].(string)
					if !strings.Contains(strings.ToLower(from), strings.ToLower(flagSender)) {
						continue
					}
				}
				if !olderThanCutoff.IsZero() {
					if ts, ok := parseNumericTime(email["created_at"]); ok {
						if ts.After(olderThanCutoff) {
							continue
						}
					} else {
						continue
					}
				}
				matched = append(matched, email)
			}

			if len(matched) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  action,
					"matched": 0,
					"decided": 0,
					"message": "no pending emails matched the filter",
				}, flags)
			}

			// Apply limit
			if flagLimit > 0 && len(matched) > flagLimit {
				matched = matched[:flagLimit]
			}

			// Confirm unless --yes
			if !flags.yes && !flags.noInput {
				fmt.Fprintf(os.Stderr, "About to %s %d pending email(s). Continue? [y/N] ", action, len(matched))
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
					return fmt.Errorf("aborted")
				}
			}

			// Execute decisions
			type decisionResult struct {
				EmailID string `json:"email_id"`
				Status  string `json:"status"`
				Error   string `json:"error,omitempty"`
			}
			var results []decisionResult
			succeeded := 0
			failed := 0

			for _, email := range matched {
				emailID, _ := email["id"].(string)
				if emailID == "" {
					emailID, _ = email["email_id"].(string)
				}
				if emailID == "" {
					continue
				}

				body := map[string]any{
					"action":   action,
					"email_id": emailID,
				}
				if flagReason != "" {
					body["reason"] = flagReason
				}

				_, _, err := c.Post(ctx, "/v1/oversight/decide", body)
				if err != nil {
					failed++
					results = append(results, decisionResult{
						EmailID: emailID,
						Status:  "error",
						Error:   err.Error(),
					})
					fmt.Fprintf(os.Stderr, "  %s %s: %v\n", action, emailID, err)
				} else {
					succeeded++
					results = append(results, decisionResult{
						EmailID: emailID,
						Status:  "ok",
					})
				}
			}

			output := map[string]any{
				"action":    action,
				"matched":   len(matched),
				"succeeded": succeeded,
				"failed":    failed,
				"results":   results,
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	cmd.Flags().BoolVar(&flagApprove, "approve", false, "Approve all matched pending emails")
	cmd.Flags().BoolVar(&flagReject, "reject", false, "Reject all matched pending emails")
	cmd.Flags().StringVar(&flagMailbox, "mailbox", "", "Filter by mailbox ID")
	cmd.Flags().StringVar(&flagSender, "sender", "", "Filter by sender address (substring match)")
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "", "Filter emails older than duration (e.g. 1h, 30m, 24h)")
	cmd.Flags().StringVar(&flagReason, "reason", "", "Reason for the decision (optional)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of emails to decide (0 = unlimited)")
	return cmd
}
