// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/cliutil"
	"github.com/spf13/cobra"
)

// flatSmsSendArgs holds the ergonomic flat-flag inputs for sms send.
type flatSmsSendArgs struct {
	to             string
	from           string
	body           string
	channelID      string
	idempotencyKey string
}

// newSmsFlatSendCmd registers a hand-built `sms send` that wraps the generated
// channels-messages POST endpoint with flat ergonomics (--to, --body, --from,
// --idempotency-key) and an optional --channel-id falling back to BIRD_CHANNEL_ID.
func newSmsFlatSendCmd(flags *rootFlags) *cobra.Command {
	var args flatSmsSendArgs
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an SMS message with flat --to/--body flags.",
		Long: `Sends an SMS message via Bird's Channels API. Wraps the underlying
POST /channels/{channelId}/messages with flat ergonomic flags so you don't
have to assemble the receiver/body envelopes by hand.

Use --idempotency-key for safe retries; the value is a free-form string and
should be deterministic per logical message (e.g. a campaign-row hash).`,
		Example: `  bird-pp-cli sms send --to +31612345678 --body "Hello from bird-pp-cli"
  bird-pp-cli sms send --to +14155550100 --body "Code: 123" --idempotency-key otp-2026-05-10-42 --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if args.channelID == "" {
				args.channelID = defaultChannelID()
			}
			if args.channelID == "" {
				return fmt.Errorf("--channel-id is required (or set BIRD_CHANNEL_ID)")
			}
			if args.to == "" {
				return fmt.Errorf("--to is required (E.164 phone number)")
			}
			if args.body == "" {
				return fmt.Errorf("--body is required")
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id":     "msg_verify",
					"status": "accepted",
					"to":     args.to,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			payload := buildFlatSendPayload(args)
			path := fmt.Sprintf("/channels/%s/messages", args.channelID)
			data, _, err := c.Post(path, payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var out map[string]any
			_ = json.Unmarshal(data, &out)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&args.to, "to", "", "Recipient phone number (E.164, required)")
	cmd.Flags().StringVar(&args.from, "from", "", "Optional sender override (channel default if omitted)")
	cmd.Flags().StringVar(&args.body, "body", "", "Text body of the SMS message (required)")
	cmd.Flags().StringVar(&args.channelID, "channel-id", "", "SMS channel ID (or set BIRD_CHANNEL_ID)")
	cmd.Flags().StringVar(&args.idempotencyKey, "idempotency-key", "", "Idempotency key for safe retries (recommended for batch sends)")
	return cmd
}

func buildFlatSendPayload(a flatSmsSendArgs) map[string]any {
	receiver := map[string]any{
		"contacts": []map[string]any{
			{
				"identifierKey":   "phonenumber",
				"identifierValue": a.to,
			},
		},
	}
	body := map[string]any{
		"type": "text",
		"text": map[string]any{"text": a.body},
	}
	out := map[string]any{
		"receiver": receiver,
		"body":     body,
	}
	if a.from != "" {
		out["sender"] = map[string]any{
			"connector": map[string]any{
				"identifierValue": a.from,
			},
		}
	}
	if a.idempotencyKey != "" {
		out["idempotencyKey"] = a.idempotencyKey
	}
	return out
}
