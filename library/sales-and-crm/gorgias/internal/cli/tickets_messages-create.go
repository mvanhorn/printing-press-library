// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newTicketsMessagesCreateCmd(flags *rootFlags) *cobra.Command {
	var flagAction string
	var bodyChannel string
	var bodyFromAgent bool
	var bodyActions string
	var bodyAttachments string
	var bodyBodyHtml string
	var bodyBodyText string
	var bodyCreatedDatetime string
	var bodyDeletedDatetime string
	var bodyExternalId string
	var bodyFailedDatetime string
	var bodyHeaders string
	var bodyIntegrationId int
	var bodyLastSendingError string
	var bodyMacros string
	var bodyMessageId string
	var bodyMeta string
	var bodyOpenedDatetime string
	var bodyPublic bool
	var bodySentDatetime string
	var bodySource string
	var bodyStrippedHtml string
	var bodyStrippedSignature string
	var bodyStrippedText string
	var bodySubject string
	var bodyVia string
	var bodyImported bool
	var bodyMentionIds string
	var bodyReceiver string
	var bodySender string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "messages-create <id>",
		Short:       "Post a new message on ticket (`id`).",
		Example:     "  gorgias-pp-cli tickets messages-create 123456789",
		Annotations: map[string]string{"pp:endpoint": "tickets.messages-create", "pp:method": "POST", "pp:path": "/tickets/{id}/messages"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// --stdin replaces the entire body. Reject the ambiguous case
			// where the user also passed individual body flags — silently
			// dropping them was a real foot-gun.
			if stdinBody {
				// Reject every body flag — silently dropping them was a real
				// footgun. Walk the local (non-inherited) flags; any one set
				// by the user other than --stdin itself is a conflict.
				var conflicts []string
				cmd.LocalFlags().Visit(func(f *pflag.Flag) {
					if f.Name != "stdin" {
						conflicts = append(conflicts, "--"+f.Name)
					}
				})
				if len(conflicts) > 0 {
					return fmt.Errorf("--stdin is mutually exclusive with %s; pass the full body via stdin OR use individual flags, not both", strings.Join(conflicts, ", "))
				}
			} else {
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

			path := "/tickets/{id}/messages"
			path = replacePathParam(path, "id", args[0])
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
				if bodyActions != "" {
					var parsedActions any
					if err := json.Unmarshal([]byte(bodyActions), &parsedActions); err != nil {
						return fmt.Errorf("parsing --actions JSON: %w", err)
					}
					body["actions"] = parsedActions
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
				if bodyCreatedDatetime != "" {
					body["created_datetime"] = bodyCreatedDatetime
				}
				if bodyDeletedDatetime != "" {
					body["deleted_datetime"] = bodyDeletedDatetime
				}
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyFailedDatetime != "" {
					body["failed_datetime"] = bodyFailedDatetime
				}
				if bodyHeaders != "" {
					var parsedHeaders any
					if err := json.Unmarshal([]byte(bodyHeaders), &parsedHeaders); err != nil {
						return fmt.Errorf("parsing --headers JSON: %w", err)
					}
					body["headers"] = parsedHeaders
				}
				if bodyIntegrationId != 0 {
					body["integration_id"] = bodyIntegrationId
				}
				if bodyLastSendingError != "" {
					body["last_sending_error"] = bodyLastSendingError
				}
				if bodyMacros != "" {
					body["macros"] = bodyMacros
				}
				if bodyMessageId != "" {
					body["message_id"] = bodyMessageId
				}
				if bodyMeta != "" {
					body["meta"] = bodyMeta
				}
				if bodyOpenedDatetime != "" {
					body["opened_datetime"] = bodyOpenedDatetime
				}
				if bodyPublic != false {
					body["public"] = bodyPublic
				}
				if bodySentDatetime != "" {
					body["sent_datetime"] = bodySentDatetime
				}
				if bodySource != "" {
					body["source"] = bodySource
				}
				if bodyStrippedHtml != "" {
					body["stripped_html"] = bodyStrippedHtml
				}
				if bodyStrippedSignature != "" {
					body["stripped_signature"] = bodyStrippedSignature
				}
				if bodyStrippedText != "" {
					body["stripped_text"] = bodyStrippedText
				}
				if bodySubject != "" {
					body["subject"] = bodySubject
				}
				if bodyVia != "" {
					body["via"] = bodyVia
				}
				if bodyImported != false {
					body["imported"] = bodyImported
				}
				if bodyMentionIds != "" {
					body["mention_ids"] = bodyMentionIds
				}
				if bodyReceiver != "" {
					body["receiver"] = bodyReceiver
				}
				if bodySender != "" {
					body["sender"] = bodySender
				}
			}
			data, statusCode, err := c.Post(path, body)
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
					"action":   "post",
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
	cmd.Flags().StringVar(&bodyChannel, "channel", "", "Channel used to send the message (enum: aircall, api, chat, contact_form, email, facebook, facebook-mention, facebook-messenger).")
	cmd.Flags().BoolVar(&bodyFromAgent, "from-agent", false, "Whether the message was sent by your company")
	cmd.Flags().StringVar(&bodyActions, "actions", "", "Actions associated with the message")
	cmd.Flags().StringVar(&bodyAttachments, "attachments", "", "Attachments associated with the message")
	cmd.Flags().StringVar(&bodyBodyHtml, "body-html", "", "HTML version of the body of the message")
	cmd.Flags().StringVar(&bodyBodyText, "body-text", "", "Text version of the body of the message")
	cmd.Flags().StringVar(&bodyCreatedDatetime, "created-datetime", "", "When the message was created")
	cmd.Flags().StringVar(&bodyDeletedDatetime, "deleted-datetime", "", "When the message was deleted")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the message in a foreign system (Aircall, Zendesk, etc...).")
	cmd.Flags().StringVar(&bodyFailedDatetime, "failed-datetime", "", "When the message failed to be sent. Messages that couldn't be sent can be resent.")
	cmd.Flags().StringVar(&bodyHeaders, "headers", "", "Headers of the message")
	cmd.Flags().IntVar(&bodyIntegrationId, "integration-id", 0, "ID of the Integration used to send the message")
	cmd.Flags().StringVar(&bodyLastSendingError, "last-sending-error", "", "Details of the last error encountered when Gorgias attempted to send the message")
	cmd.Flags().StringVar(&bodyMacros, "macros", "", "Macros used to compose the message")
	cmd.Flags().StringVar(&bodyMessageId, "message-id", "", "ID of the message on the service that sent the message.")
	cmd.Flags().StringVar(&bodyMeta, "meta", "", "Metadata associated with the ticket message.")
	cmd.Flags().StringVar(&bodyOpenedDatetime, "opened-datetime", "", "When the message was seen by its recipient")
	cmd.Flags().BoolVar(&bodyPublic, "public", false, "Whether the message is public. Only internal notes are private.")
	cmd.Flags().StringVar(&bodySentDatetime, "sent-datetime", "", "When the message was sent. If omitted, the message will be sent by Gorgias.")
	cmd.Flags().StringVar(&bodySource, "source", "", "Information used to route the message. It contains the names and the addresses of the sender and receivers.")
	cmd.Flags().StringVar(&bodyStrippedHtml, "stripped-html", "", "HTML version of the body of the message without email signatures and previous replies.")
	cmd.Flags().StringVar(&bodyStrippedSignature, "stripped-signature", "", "Signature stripped from the body of the message")
	cmd.Flags().StringVar(&bodyStrippedText, "stripped-text", "", "Text version of the body of the message without email signatures and previous replies.")
	cmd.Flags().StringVar(&bodySubject, "subject", "", "Subject of the message")
	cmd.Flags().StringVar(&bodyVia, "via", "", "How the message has been received or sent from Gorgias (enum: aircall, api, chat, contact_form, email, facebook, facebook-mention, facebook-messenger)")
	cmd.Flags().BoolVar(&bodyImported, "imported", false, "Whether the message was created by a historical import.")
	cmd.Flags().StringVar(&bodyMentionIds, "mention-ids", "", "List of User IDs to mention along with the internal note.")
	cmd.Flags().StringVar(&bodyReceiver, "receiver", "", "Primary receiver of the message. It can be a user or a customer. Optional when the source type is 'internal-note'.")
	cmd.Flags().StringVar(&bodySender, "sender", "", "Person who sent the message. It can be a user or a customer.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
