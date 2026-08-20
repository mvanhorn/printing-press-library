// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newTicketsUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyAssigneeTeam string
	var bodyAssigneeUser string
	var bodyChannel string
	var bodyClosedDatetime string
	var bodyCustomFields string
	var bodyCustomer string
	var bodyExternalId string
	var bodyFromAgent bool
	var bodyIsUnread bool
	var bodyLanguage string
	var bodyLastMessageDatetime string
	var bodyLastReceivedMessageDatetime string
	var bodyMeta string
	var bodyOpenedDatetime string
	var bodyPriority string
	var bodySnoozeDatetime string
	var bodySpam bool
	var bodyStatus string
	var bodySubject string
	var bodyTags string
	var bodyTrashedDatetime string
	var bodyUpdatedDatetime string
	var bodyVia string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update a ticket (`id`).",
		Example:     "  gorgias-pp-cli tickets update 123456789",
		Annotations: map[string]string{"pp:endpoint": "tickets.update", "pp:method": "PUT", "pp:path": "/tickets/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/tickets/{id}"
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
				if bodyAssigneeTeam != "" {
					parsed, err := parseBodyJSONField("assignee_team", bodyAssigneeTeam)
					if err != nil {
						return err
					}
					body["assignee_team"] = parsed
				}
				if bodyAssigneeUser != "" {
					parsed, err := parseBodyJSONField("assignee_user", bodyAssigneeUser)
					if err != nil {
						return err
					}
					body["assignee_user"] = parsed
				}
				if bodyChannel != "" {
					body["channel"] = bodyChannel
				}
				if bodyClosedDatetime != "" {
					body["closed_datetime"] = bodyClosedDatetime
				}
				if bodyCustomFields != "" {
					parsed, err := parseBodyJSONField("custom_fields", bodyCustomFields)
					if err != nil {
						return err
					}
					body["custom_fields"] = parsed
				}
				if bodyCustomer != "" {
					parsed, err := parseBodyJSONField("customer", bodyCustomer)
					if err != nil {
						return err
					}
					body["customer"] = parsed
				}
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyFromAgent != false {
					body["from_agent"] = bodyFromAgent
				}
				if bodyIsUnread != false {
					body["is_unread"] = bodyIsUnread
				}
				if bodyLanguage != "" {
					body["language"] = bodyLanguage
				}
				if bodyLastMessageDatetime != "" {
					body["last_message_datetime"] = bodyLastMessageDatetime
				}
				if bodyLastReceivedMessageDatetime != "" {
					body["last_received_message_datetime"] = bodyLastReceivedMessageDatetime
				}
				if bodyMeta != "" {
					var parsedMeta any
					if err := json.Unmarshal([]byte(bodyMeta), &parsedMeta); err != nil {
						return fmt.Errorf("parsing --meta JSON: %w", err)
					}
					body["meta"] = parsedMeta
				}
				if bodyOpenedDatetime != "" {
					body["opened_datetime"] = bodyOpenedDatetime
				}
				if bodyPriority != "" {
					parsed, err := parseBodyJSONField("priority", bodyPriority)
					if err != nil {
						return err
					}
					body["priority"] = parsed
				}
				if bodySnoozeDatetime != "" {
					body["snooze_datetime"] = bodySnoozeDatetime
				}
				if bodySpam != false {
					body["spam"] = bodySpam
				}
				if bodyStatus != "" {
					parsed, err := parseBodyJSONField("status", bodyStatus)
					if err != nil {
						return err
					}
					body["status"] = parsed
				}
				if bodySubject != "" {
					body["subject"] = bodySubject
				}
				if bodyTags != "" {
					parsed, err := parseBodyJSONField("tags", bodyTags)
					if err != nil {
						return err
					}
					body["tags"] = parsed
				}
				if bodyTrashedDatetime != "" {
					body["trashed_datetime"] = bodyTrashedDatetime
				}
				if bodyUpdatedDatetime != "" {
					body["updated_datetime"] = bodyUpdatedDatetime
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
	cmd.Flags().StringVar(&bodyAssigneeTeam, "assignee-team", "", "The team to assign to the ticket")
	cmd.Flags().StringVar(&bodyAssigneeUser, "assignee-user", "", "The user to assign to the ticket")
	cmd.Flags().StringVar(&bodyChannel, "channel", "", "Channel used to initiate the conversation (enum: aircall, api, chat, contact_form, email, facebook, facebook-mention, facebook-messenger).")
	cmd.Flags().StringVar(&bodyClosedDatetime, "closed-datetime", "", "When the ticket was closed (ISO 8601).")
	cmd.Flags().StringVar(&bodyCustomFields, "custom-fields", "", "Custom fields associated with the ticket (JSON array of {id, value} objects).")
	cmd.Flags().StringVar(&bodyCustomer, "customer", "", "Customer associated with the ticket (JSON object with id, or a customer ID).")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the ticket in a foreign system. This field is not used by Gorgias, feel free to set it as you wish.")
	cmd.Flags().BoolVar(&bodyFromAgent, "from-agent", false, "Whether the first message of the ticket was sent by your company to a customer, or the opposite.")
	cmd.Flags().BoolVar(&bodyIsUnread, "is-unread", false, "Mark the ticket as read/unread.")
	cmd.Flags().StringVar(&bodyLanguage, "language", "", "Language primarily used in the ticket (ISO 639-1 code: en, fr, es, de, it, pt, nl, ja, zh, etc.).")
	cmd.Flags().StringVar(&bodyLastMessageDatetime, "last-message-datetime", "", "When the last message was sent (ISO 8601).")
	cmd.Flags().StringVar(&bodyLastReceivedMessageDatetime, "last-received-message-datetime", "", "When the last customer's message was received (ISO 8601).")
	cmd.Flags().StringVar(&bodyMeta, "meta", "", "Metadata associated with the ticket. Use to store structured key-value information about the ticket (JSON object).")
	cmd.Flags().StringVar(&bodyOpenedDatetime, "opened-datetime", "", "When the ticket was first opened by a user (ISO 8601).")
	cmd.Flags().StringVar(&bodyPriority, "priority", "", "Priority of the ticket (one of: low, normal, high, urgent).")
	cmd.Flags().StringVar(&bodySnoozeDatetime, "snooze-datetime", "", "When the ticket will be re-opened automatically (ISO 8601).")
	cmd.Flags().BoolVar(&bodySpam, "spam", false, "Whether the ticket is considered as spam.")
	cmd.Flags().StringVar(&bodyStatus, "status", "", "Status of the ticket (one of: open, closed, snoozed).")
	cmd.Flags().StringVar(&bodySubject, "subject", "", "Subject of the ticket")
	cmd.Flags().StringVar(&bodyTags, "tags", "", "Tags of the ticket")
	cmd.Flags().StringVar(&bodyTrashedDatetime, "trashed-datetime", "", "A datetime will move the ticket to the trash, `null` will move the ticket out of the trash")
	cmd.Flags().StringVar(&bodyUpdatedDatetime, "updated-datetime", "", "When the ticket was lastly updated")
	cmd.Flags().StringVar(&bodyVia, "via", "", "How the first message of the ticket has been received or sent from Gorgias (enum: aircall, api, chat, contact_form, email, facebook, facebook-mention, facebook-messenger — per spec-sources/gorgias-crowd.yaml; see https://developers.gorgias.com/reference/post_api-tickets for the live API enum)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
