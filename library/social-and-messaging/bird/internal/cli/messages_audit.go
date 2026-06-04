// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/cliutil"
	"github.com/spf13/cobra"
)

// auditEvent is one row in the merged delivery timeline.
type auditEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

// auditView is the consolidated audit envelope.
type auditView struct {
	MessageID string       `json:"messageId"`
	ChannelID string       `json:"channelId"`
	Status    string       `json:"status"`
	Direction string       `json:"direction,omitempty"`
	CreatedAt string       `json:"createdAt,omitempty"`
	Final     string       `json:"finalState"`
	Failed    bool         `json:"failed"`
	Events    []auditEvent `json:"events"`
	Summary   string       `json:"summary"`
}

func newMessagesAuditCmd(flags *rootFlags) *cobra.Command {
	var (
		channelID string
		wait      time.Duration
	)
	cmd := &cobra.Command{
		Use:   "audit <message_id>",
		Short: "Fold a message and its interactions into a chronological delivery timeline.",
		Long: `Fetches the message and its delivery interactions, merges them into a single
chronological timeline, and exits non-zero on terminal failure (failed, undelivered).

Use this command from a CI gate or a Slackbot trigger when you need a one-shot
answer to "did this SMS land?".`,
		Example: "  bird-pp-cli messages audit msg_abc --channel-id ch_sms_1 --json\n  bird-pp-cli messages audit msg_abc --wait 30s --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			messageID := args[0]
			if channelID == "" {
				return fmt.Errorf("--channel-id is required (or set BIRD_CHANNEL_ID)")
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				// Print a stable verify-mode envelope and return success.
				view := auditView{MessageID: messageID, ChannelID: channelID, Status: "delivered", Final: "delivered", Events: nil, Summary: "verify mode: skipping live audit"}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			deadline := time.Now().Add(wait)
			for {
				view, terminal, err := runAudit(c, channelID, messageID)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if terminal || time.Now().After(deadline) {
					if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
						return err
					}
					if view.Failed {
						return fmt.Errorf("message %s ended in terminal failure: %s", messageID, view.Final)
					}
					return nil
				}
				time.Sleep(2 * time.Second)
			}
		},
	}
	cmd.Flags().StringVar(&channelID, "channel-id", defaultChannelID(), "Channel ID (or set BIRD_CHANNEL_ID)")
	cmd.Flags().DurationVar(&wait, "wait", 0, "Wait up to this duration for a terminal delivery state (e.g. 30s)")
	return cmd
}

func runAudit(c clientGetter, channelID, messageID string) (auditView, bool, error) {
	msgPath := fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID)
	msgRaw, err := c.Get(msgPath, nil)
	if err != nil {
		return auditView{}, true, err
	}
	var msg map[string]any
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return auditView{}, true, fmt.Errorf("parsing message: %w", err)
	}
	intPath := fmt.Sprintf("/channels/%s/messages/%s/interactions", channelID, messageID)
	intRaw, err := c.Get(intPath, nil)
	if err != nil {
		return auditView{}, true, err
	}
	view := auditView{
		MessageID: messageID,
		ChannelID: channelID,
	}
	if s, ok := msg["status"].(string); ok {
		view.Status = s
	}
	if d, ok := msg["direction"].(string); ok {
		view.Direction = d
	}
	if t, ok := msg["createdAt"].(string); ok {
		view.CreatedAt = t
	}
	view.Events = parseInteractions(intRaw)
	if len(view.Events) > 0 {
		last := view.Events[len(view.Events)-1]
		view.Final = last.Type
	} else {
		view.Final = view.Status
	}
	view.Failed = isTerminalFailure(view.Final)
	terminal := view.Failed || isTerminalSuccess(view.Final)
	view.Summary = fmt.Sprintf("%d events, final=%s", len(view.Events), view.Final)
	return view, terminal, nil
}

func parseInteractions(raw json.RawMessage) []auditEvent {
	if len(raw) == 0 {
		return nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		// Try the {data: [...]} wrapper.
		var wrapped struct {
			Data []map[string]any `json:"data"`
		}
		if err2 := json.Unmarshal(raw, &wrapped); err2 != nil {
			return nil
		}
		arr = wrapped.Data
	}
	events := make([]auditEvent, 0, len(arr))
	for _, item := range arr {
		ev := auditEvent{}
		if t, ok := item["type"].(string); ok {
			ev.Type = t
		} else if t, ok := item["status"].(string); ok {
			ev.Type = t
		}
		if ts, ok := item["timestamp"].(string); ok {
			ev.Timestamp = ts
		} else if ts, ok := item["createdAt"].(string); ok {
			ev.Timestamp = ts
		}
		if r, ok := item["reason"].(string); ok {
			ev.Reason = r
		}
		events = append(events, ev)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})
	return events
}

func isTerminalFailure(state string) bool {
	switch strings.ToLower(state) {
	case "failed", "undelivered", "rejected", "expired":
		return true
	}
	return false
}

func isTerminalSuccess(state string) bool {
	switch strings.ToLower(state) {
	case "delivered", "read":
		return true
	}
	return false
}

// clientGetter is a narrow interface so tests can stub Get.
type clientGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func defaultChannelID() string {
	return os.Getenv("BIRD_CHANNEL_ID")
}
