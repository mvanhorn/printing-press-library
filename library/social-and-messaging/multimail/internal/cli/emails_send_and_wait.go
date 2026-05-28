// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live

func newNovelEmailsSendAndWaitCmd(flags *rootFlags) *cobra.Command {
	var flagMailbox string
	var flagTo string
	var flagSubject string
	var flagBody string
	var flagTimeout time.Duration
	var flagPollInterval time.Duration
	var stdinBody bool

	cmd := &cobra.Command{
		Use:   "send-and-wait",
		Short: "Send an email and block until a reply arrives or timeout — the agent test loop in one command.",
		Long: `Send-and-wait chains the send API with a poll loop: sends the email, then
repeatedly checks the mailbox for a reply to the thread until one arrives
or the timeout expires. This is the agent test loop in one command.`,
		Annotations: map[string]string{"pp:endpoint": "emails.send-and-wait", "pp:method": "POST"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagMailbox == "" {
				return fmt.Errorf("--mailbox is required")
			}
			if flagTo == "" {
				return fmt.Errorf("--to is required")
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Build send body
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
			} else {
				body = map[string]any{
					"to":      flagTo,
					"subject": flagSubject,
				}
				if flagBody != "" {
					body["markdown"] = flagBody
				}
			}

			// Send the email
			sendPath := fmt.Sprintf("/v1/mailboxes/%s/send", flagMailbox)
			sendData, _, err := c.Post(ctx, sendPath, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var sendResult map[string]any
			if err := json.Unmarshal(sendData, &sendResult); err != nil {
				return fmt.Errorf("parsing send response: %w", err)
			}

			emailID, _ := sendResult["id"].(string)
			threadID, _ := sendResult["thread_id"].(string)
			status, _ := sendResult["status"].(string)

			fmt.Fprintf(os.Stderr, "sent email %s (thread: %s, status: %s)\n", emailID, threadID, status)

			if status == "pending_oversight" {
				fmt.Fprintf(os.Stderr, "warning: email is pending oversight approval — reply may not arrive until approved\n")
			}

			if threadID == "" {
				// Can't poll without a thread ID
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"phase":   "sent",
					"email":   sendResult,
					"reply":   nil,
					"message": "no thread_id in send response; cannot poll for reply",
				}, flags)
			}

			// Poll for reply
			fmt.Fprintf(os.Stderr, "waiting for reply (timeout: %s, poll: %s)...\n", flagTimeout, flagPollInterval)

			deadline := time.Now().Add(flagTimeout)
			threadPath := fmt.Sprintf("/v1/mailboxes/%s/threads/%s", flagMailbox, threadID)
			pollCount := 0

			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(flagPollInterval):
				}
				pollCount++

				threadData, err := c.Get(ctx, threadPath, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  poll %d: error fetching thread: %v\n", pollCount, err)
					continue
				}

				var thread map[string]any
				if err := json.Unmarshal(threadData, &thread); err != nil {
					continue
				}

				// Check for new messages
				msgCount := 0
				if mc, ok := thread["message_count"].(float64); ok {
					msgCount = int(mc)
				}
				if emails, ok := thread["emails"].([]any); ok {
					msgCount = len(emails)
					// Look for an inbound reply (direction != "outbound", id != our sent email)
					for _, e := range emails {
						if em, ok := e.(map[string]any); ok {
							eid, _ := em["id"].(string)
							dir, _ := em["direction"].(string)
							if eid != emailID && (dir == "inbound" || dir == "received") {
								fmt.Fprintf(os.Stderr, "  reply received after %d polls\n", pollCount)
								return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
									"phase":      "reply_received",
									"sent_email": sendResult,
									"reply":      em,
									"thread":     thread,
									"polls":      pollCount,
									"elapsed":    time.Since(deadline.Add(-flagTimeout)).String(),
								}, flags)
							}
						}
					}
				}

				fmt.Fprintf(os.Stderr, "  poll %d: %d messages, no reply yet\n", pollCount, msgCount)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"phase":      "timeout",
				"sent_email": sendResult,
				"reply":      nil,
				"thread_id":  threadID,
				"polls":      pollCount,
				"timeout":    flagTimeout.String(),
				"message":    "no reply received before timeout",
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagMailbox, "mailbox", "", "Mailbox ID to send from (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Recipient email address (required)")
	cmd.Flags().StringVar(&flagSubject, "subject", "", "Email subject")
	cmd.Flags().StringVar(&flagBody, "body", "", "Email body (markdown)")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 5*time.Minute, "Maximum time to wait for a reply")
	cmd.Flags().DurationVar(&flagPollInterval, "poll-interval", 10*time.Second, "How often to check for a reply")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read full send request body from stdin as JSON")
	return cmd
}
