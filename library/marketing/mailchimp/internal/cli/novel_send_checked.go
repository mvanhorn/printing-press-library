// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type checklistItem struct {
	Type    string `json:"type"`
	Heading string `json:"heading"`
	Details string `json:"details"`
	ID      int    `json:"id"`
}

// newSendCheckedCmd is the CI-gate variant of campaigns send: runs the
// send-checklist first and refuses to send unless every item is "passed."
// The spec-emitted raw send is still reachable via 'campaigns actions
// post-campaigns-id-send' for cases where the user wants to bypass the gate.
func newSendCheckedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-checked <campaign-id>",
		Short: "Send a campaign only if the official Mailchimp send-checklist passes. Exits 2 with the failing items if not.",
		Long: `Run /campaigns/{id}/send-checklist before POST /campaigns/{id}/actions/send.

If the checklist returns is_ready=false OR any item has type=error, this
command:
  - prints the failing items to stderr
  - exits with code 2 (typed gate-failure)
  - does NOT call the send endpoint

If the checklist passes, the send proceeds normally and the command exits 0.

Use this in CI pipelines to refuse to ship broken campaigns. For an unchecked
send, use 'mailchimp-pp-cli campaigns actions post-campaigns-id-send'.`,
		Example: `  mailchimp-pp-cli send-checked 7f8a9b0c1d
  mailchimp-pp-cli send-checked 7f8a9b0c1d --dry-run     # show checklist without sending`,
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			cid := args[0]
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_check":          fmt.Sprintf("/campaigns/%s/send-checklist", cid),
					"would_send_if_passed": fmt.Sprintf("/campaigns/%s/actions/send", cid),
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Use GetNoCache: this is a CI gate. A cached checklist (up to
			// 5 minutes stale) could pass a campaign whose current state
			// would fail, or block one that has since been fixed. The gate
			// must always reflect the server's view at send time.
			data, err := c.GetNoCache(fmt.Sprintf("/campaigns/%s/send-checklist", cid), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var checklist struct {
				IsReady bool            `json:"is_ready"`
				Items   []checklistItem `json:"items"`
			}
			if err := json.Unmarshal(data, &checklist); err != nil {
				return fmt.Errorf("parsing send-checklist response: %w", err)
			}

			var errors []checklistItem
			for _, item := range checklist.Items {
				if item.Type == "error" {
					errors = append(errors, item)
				}
			}

			if !checklist.IsReady || len(errors) > 0 {
				result := map[string]any{
					"campaign_id": cid,
					"gate":        "failed",
					"is_ready":    checklist.IsReady,
					"errors":      errors,
					"all_items":   checklist.Items,
				}
				// Print structured failure to stdout; the typed exit code carries
				// the gate-fail signal for CI.
				_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
				return &cliError{code: 2, err: fmt.Errorf("send-checklist gate failed: %d error item(s); campaign not sent", len(errors))}
			}

			// Checklist passed — send.
			sendData, _, err := c.Post(fmt.Sprintf("/campaigns/%s/actions/send", cid), map[string]any{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			result := map[string]any{
				"campaign_id": cid,
				"gate":        "passed",
				"sent":        true,
				"response":    json.RawMessage(sendData),
			}
			// Mailchimp returns 204 No Content for successful send; sendData may be empty.
			if len(sendData) == 0 {
				result["response"] = map[string]string{"status": "queued"}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}
