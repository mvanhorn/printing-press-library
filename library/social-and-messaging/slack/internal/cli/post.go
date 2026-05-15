// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: post. A cron-safe wrapper over
// chat.postMessage with --dry-run and verify-env short-circuits.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/cliutil"
)

func newPostCmd(flags *rootFlags) *cobra.Command {
	var channel string
	var text string
	var thread string
	var blocksJSON string

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Post a message to a channel (cron-safe wrapper over chat.postMessage)",
		Long: `Post a message to a Slack channel or thread. A thin, cron-safe wrapper
over chat.postMessage: --dry-run prints the exact request without sending
it, and the command short-circuits automatically under the printing-press
verifier so a verify pass never posts to a real workspace.

Provide message content with --text, or a Block Kit layout with
--blocks-json pointing at a file containing a JSON blocks array.`,
		// Mutating verb — no mcp:read-only annotation.
		Example: strings.Trim(`
  # Post to #churnsales
  slack-pp-cli post --channel "#churnsales" --text "Sonria renewal closed"

  # Reply in a thread
  slack-pp-cli post --channel C0123ABCD --text "on it" --thread 1747000000.001200

  # Preview the request without sending (cron safety)
  slack-pp-cli post --channel "#csm" --text "weekly digest ready" --dry-run

  # Post a Block Kit layout from a file
  slack-pp-cli post --channel "#data" --blocks-json ./digest-blocks.json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify-friendly: no required-flag markers. Validate here.
			if channel == "" && text == "" && blocksJSON == "" {
				return cmd.Help()
			}
			if channel == "" {
				return usageErr(fmt.Errorf("--channel is required"))
			}
			if text == "" && blocksJSON == "" {
				return usageErr(fmt.Errorf("provide message content with --text or --blocks-json"))
			}

			fields := url.Values{}
			fields.Set("channel", channel)
			if text != "" {
				fields.Set("text", text)
			}
			if thread != "" {
				fields.Set("thread_ts", thread)
			}
			if blocksJSON != "" {
				raw, err := os.ReadFile(blocksJSON)
				if err != nil {
					return usageErr(fmt.Errorf("reading --blocks-json file %q: %w", blocksJSON, err))
				}
				if !json.Valid(raw) {
					return usageErr(fmt.Errorf("--blocks-json file %q is not valid JSON", blocksJSON))
				}
				fields.Set("blocks", string(raw))
			}

			// --dry-run: print the request, never send. The client's own
			// DryRun path also guards this, but printing here keeps the
			// output stable and lets the short-circuit happen before any
			// client construction.
			if dryRunOK(flags) {
				printPostPreview(cmd, "post", "/chat.postMessage", fields)
				return nil
			}
			// Verifier short-circuit — defense in depth for the verify pass.
			if cliutil.IsVerifyEnv() {
				printPostPreview(cmd, "post", "/chat.postMessage", fields)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostForm("/chat.postMessage", fields)
			if err != nil {
				if rl, ok := err.(*cliutil.RateLimitError); ok {
					return rateLimitErr(rl)
				}
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel id or name to post to (e.g. #churnsales or C0123ABCD)")
	cmd.Flags().StringVar(&text, "text", "", "Message text")
	cmd.Flags().StringVar(&thread, "thread", "", "Parent message ts to reply in a thread")
	cmd.Flags().StringVar(&blocksJSON, "blocks-json", "", "Path to a file containing a Block Kit blocks JSON array")
	return cmd
}

// printPostPreview emits the request that would be sent — used by the
// --dry-run and verify-env short-circuits so neither path touches the
// network.
func printPostPreview(cmd *cobra.Command, verb, path string, fields url.Values) {
	preview := map[string]any{
		"event":  "dry_run",
		"verb":   verb,
		"method": "POST",
		"path":   path,
		"fields": fieldsToMap(fields),
	}
	out, _ := json.Marshal(preview)
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	fmt.Fprintf(cmd.ErrOrStderr(), "would post: %s %s (no request sent)\n", "POST", path)
}

// fieldsToMap flattens url.Values to a single-valued map for preview.
func fieldsToMap(fields url.Values) map[string]string {
	m := make(map[string]string, len(fields))
	for k := range fields {
		m[k] = fields.Get(k)
	}
	return m
}
