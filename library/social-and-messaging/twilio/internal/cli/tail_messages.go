// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/cliutil"

	"github.com/spf13/cobra"
)

func newTailMessagesCmd(flags *rootFlags) *cobra.Command {
	var follow bool
	var status string
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "tail-messages",
		Short: "Stream new messages with a status filter as they happen (--follow that twilio-cli does not ship)",
		Long: `Polls the messages list endpoint with DateUpdated>=last_seen every N seconds.
Streams each new row as a JSON line on stdout. Useful for incident triage
("tail SMS failures during a delivery degradation") and on-call response.

Short-circuits to a no-op when PRINTING_PRESS_VERIFY=1 is set so verify
runs do not start a polling loop.`,
		Example: `  twilio-pp-cli tail-messages --follow --status failed
  twilio-pp-cli tail-messages --status delivered --interval 10s --follow`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				// Verifier short-circuit: never start a polling loop during a
				// shipcheck pass, even if --follow was passed.
				fmt.Fprintln(cmd.OutOrStdout(), "tail-messages: skipped under PRINTING_PRESS_VERIFY=1")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			accountSid := getAccountSidFromConfig(flags)
			if accountSid == "" {
				return authErr(fmt.Errorf("TWILIO_ACCOUNT_SID is required to construct the messages URL"))
			}
			path := fmt.Sprintf("/2010-04-01/Accounts/%s/Messages.json", accountSid)

			if !follow {
				_, err := tailOnce(cmd.Context(), c, path, status, "", cmd.OutOrStdout())
				return err
			}

			lastSeen := time.Now().UTC().Format(time.RFC3339)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-cmd.Context().Done():
					return nil
				case <-ticker.C:
					next, err := tailOnce(cmd.Context(), c, path, status, lastSeen, cmd.OutOrStdout())
					if err != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), "tail-messages: error:", err)
						continue
					}
					if next != "" {
						lastSeen = next
					}
				}
			}
		},
	}
	cmd.Flags().BoolVar(&follow, "follow", false, "Stream new messages as they arrive (otherwise one-shot)")
	cmd.Flags().StringVar(&status, "status", "", "Only stream messages with this status (e.g. failed, delivered, undelivered)")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Poll interval when --follow is set")
	return cmd
}

// tailOnce performs one fetch of recent messages and writes each match as a
// JSON line to w. Returns the most recent date_updated seen so the next poll
// can use it as the lower bound.
func tailOnce(ctx context.Context, c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, path, status, lastSeen string, w interface{ Write([]byte) (int, error) }) (string, error) {
	params := map[string]string{}
	if lastSeen != "" {
		params["DateSent>"] = lastSeen
	}
	if status != "" {
		params["Status"] = status
	}
	data, err := c.Get(path, params)
	if err != nil {
		return "", err
	}
	var env struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", err
	}
	maxSeen := lastSeen
	for _, m := range env.Messages {
		if _, err := w.Write(append([]byte(strings.TrimSpace(string(m))), '\n')); err != nil {
			return "", err
		}
		var msg struct {
			DateUpdated string `json:"date_updated"`
		}
		if jerr := json.Unmarshal(m, &msg); jerr == nil && msg.DateUpdated > maxSeen {
			maxSeen = msg.DateUpdated
		}
	}
	return maxSeen, nil
}
