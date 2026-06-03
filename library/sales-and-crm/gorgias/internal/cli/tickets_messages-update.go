// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newTicketsMessagesUpdateCmd(flags *rootFlags) *cobra.Command {
	var flagAction string
	var bodyChannel string
	var bodyFromAgent bool
	var bodyAttachments string
	var bodyBodyHtml string
	var bodyBodyText string
	var bodyExternalId string
	var bodyFailedDatetime string
	var bodyMessageId string
	var bodyReceiver string
	var bodySender string
	var bodySentDatetime string
	var bodySource string
	var bodySubject string
	var bodyVia string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "messages-update <ticket-id> <message-id>",
		Short:       "Update a message: pass <ticket-id> <message-id>.",
		Example:     "  gorgias-pp-cli tickets messages-update 123456789 123456789 --channel example-value",
		Annotations: map[string]string{"pp:endpoint": "tickets.messages-update", "pp:method": "PUT", "pp:path": "/tickets/{ticket_id}/messages/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !stdinBody {
				if !cmd.Flags().Changed("channel") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "channel")
				}
				if !cmd.Flags().Changed("from-agent") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "from-agent")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/tickets/{ticket_id}/messages/{id}"
			path = replacePathParam(path, "ticket_id", args[0])
			if len(args) < 2 {
				return usageErr(fmt.Errorf("id is required\nUsage: %s <%s>", cmd.CommandPath(), "id"))
			}
			path = replacePathParam(path, "id", args[1])
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				body = map[string]any{}
				if bodyChannel != "" {
					body["channel"] = bodyChannel
				}
				if bodyFromAgent != false {
					body["from_agent"] = bodyFromAgent
				}
				if bodyAttachments != "" {
					body["attachments"] = bodyAttachments
				}
				if bodyBodyHtml != "" {
					body["body_html"] = bodyBodyHtml
				}
				if bodyBodyText != "" {
					body["body_text"] = bodyBodyText
				}
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyFailedDatetime != "" {
					body["failed_datetime"] = bodyFailedDatetime
				}
				if bodyMessageId != "" {
					body["message_id"] = bodyMessageId
				}
				if bodyReceiver != "" {
					var parsedReceiver any
					if err := json.Unmarshal([]byte(bodyReceiver), &parsedReceiver); err != nil {
						return fmt.Errorf("parsing --receiver JSON: %w", err)
					}
					body["receiver"] = parsedReceiver
				}
				if bodySender != "" {
					var parsedSender any
					if err := json.Unmarshal([]byte(bodySender), &parsedSender); err != nil {
						return fmt.Errorf("parsing --sender JSON: %w", err)
					}
					body["sender"] = parsedSender
				}
				if bodySentDatetime != "" {
					body["sent_datetime"] = bodySentDatetime
				}
				if bodySource != "" {
					var parsedSource any
					if err := json.Unmarshal([]byte(bodySource), &parsedSource); err != nil {
						return fmt.Errorf("parsing --source JSON: %w", err)
					}
					body["source"] = parsedSource
				}
				if bodySubject != "" {
					body["subject"] = bodySubject
				}
				if bodyVia != "" {
					body["via"] = bodyVia
				}
			}
			data, statusCode, err := c.Put(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "put",
					"resource": "tickets",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagAction, "action", "", "Policy applied on external actions associated with the message if they failed.")
	cmd.Flags().StringVar(&bodyChannel, "channel", "", "The channel used to send the message.")
	cmd.Flags().BoolVar(&bodyFromAgent, "from-agent", false, "Whether the message was sent by your company to a customer, or the opposite. (example: True)")
	cmd.Flags().StringVar(&bodyAttachments, "attachments", "", "A list of files attached to the message. (example: [])")
	cmd.Flags().StringVar(&bodyBodyHtml, "body-html", "", "The full HTML version of the body of the message, if any.")
	cmd.Flags().StringVar(&bodyBodyText, "body-text", "", "The full text version of the body of the message, if any.")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the message in a foreign system (Aircall, Zendesk, etc...).")
	cmd.Flags().StringVar(&bodyFailedDatetime, "failed-datetime", "", "When the message failed to be sent. Messages that couldn't be sent can be resend.")
	cmd.Flags().StringVar(&bodyMessageId, "message-id", "", "Foreign-system identifier for this message (email Message-ID, Messenger id, etc).")
	cmd.Flags().StringVar(&bodyReceiver, "receiver", "", "The primary receiver of the message.")
	cmd.Flags().StringVar(&bodySender, "sender", "", "The person who sent the message. It can be a user or a customer. (example: {'id': 93})")
	cmd.Flags().StringVar(&bodySentDatetime, "sent-datetime", "", "When the message was sent. If ommited, the message will be sent by Gorgias.")
	cmd.Flags().StringVar(&bodySource, "source", "", "Information used to route the message.")
	cmd.Flags().StringVar(&bodySubject, "subject", "", "The subject of the message. (example: Re:Refund request)")
	cmd.Flags().StringVar(&bodyVia, "via", "", "How the message has been received, or sent from Gorgias.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
