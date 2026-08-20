// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newSatisfactionSurveysCreateCmd(flags *rootFlags) *cobra.Command {
	var bodyCustomerId int
	var bodyTicketId int
	var bodyBodyText string
	var bodyCreatedDatetime string
	var bodyMeta string
	var bodyScore string
	var bodyScoredDatetime string
	var bodySentDatetime string
	var bodyShouldSendDatetime string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a satisfaction-survey instance attached to one ticket and customer.",
		Example:     "  gorgias-pp-cli satisfaction-surveys create",
		Annotations: map[string]string{"pp:endpoint": "satisfaction-surveys.create", "pp:method": "POST", "pp:path": "/satisfaction-surveys"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("customer-id") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "customer-id")
				}
				if !cmd.Flags().Changed("ticket-id") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "ticket-id")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/satisfaction-surveys"
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
				if bodyCustomerId != 0 {
					body["customer_id"] = bodyCustomerId
				}
				if bodyTicketId != 0 {
					body["ticket_id"] = bodyTicketId
				}
				if bodyBodyText != "" {
					body["body_text"] = bodyBodyText
				}
				if bodyCreatedDatetime != "" {
					body["created_datetime"] = bodyCreatedDatetime
				}
				if bodyMeta != "" {
					parsed, err := parseBodyJSONField("meta", bodyMeta)
					if err != nil {
						return err
					}
					body["meta"] = parsed
				}
				if bodyScore != "" {
					body["score"] = bodyScore
				}
				if bodyScoredDatetime != "" {
					body["scored_datetime"] = bodyScoredDatetime
				}
				if bodySentDatetime != "" {
					body["sent_datetime"] = bodySentDatetime
				}
				if bodyShouldSendDatetime != "" {
					body["should_send_datetime"] = bodyShouldSendDatetime
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
					"resource": "satisfaction-surveys",
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
	cmd.Flags().IntVar(&bodyCustomerId, "customer-id", 0, "The ID of the customer who filled the survey. (example: 120)")
	cmd.Flags().IntVar(&bodyTicketId, "ticket-id", 0, "The ID of the ticket the survey is associated with. (example: 12)")
	cmd.Flags().StringVar(&bodyBodyText, "body-text", "", "The comment sent by the customer. (example: Such speed, much pertinent.)")
	cmd.Flags().StringVar(&bodyCreatedDatetime, "created-datetime", "", "When the survey was created. (example: 2019-11-16T15:59:41.966927)")
	cmd.Flags().StringVar(&bodyMeta, "meta", "", "Data associated with the satisfaction survey.")
	cmd.Flags().StringVar(&bodyScore, "score", "", "The level of satisfaction. Scores range from 1 to 5. (example: 2)")
	cmd.Flags().StringVar(&bodyScoredDatetime, "scored-datetime", "", "When the survey was filled by the customer. (example: 2019-11-25T15:59:41.966927)")
	cmd.Flags().StringVar(&bodySentDatetime, "sent-datetime", "", "When the survey was sent. If is not set it means that it was not sent yet. (example: 2019-11-23T15:59:41.966927)")
	cmd.Flags().StringVar(&bodyShouldSendDatetime, "should-send-datetime", "", "When the survey should be sent.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
