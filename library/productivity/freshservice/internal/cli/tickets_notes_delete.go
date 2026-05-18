// Copyright 2026 mark-van-de-ven. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PATCH(freshservice-notes-delete): hand-authored DELETE /conversations/{id}.
// Freshservice documents the endpoint but does not declare it in the OpenAPI
// spec, so the generator never emitted a delete-note command. Notes live as
// conversations; the conversation id alone identifies the note (no ticket_id
// segment).
//
// Returns HTTP 204 No Content on success. The CLI surfaces success as a JSON
// envelope so an agent can verify before/after with `tickets conversations
// list-ticket <ticket-id>`.
func newTicketsNotesDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <conversation-id>",
		Short: "Delete a ticket note (conversation) by its conversation id",
		Long: `Delete a ticket note by its conversation id. Use 'tickets
conversations list-ticket <ticket-id>' to discover the conversation id —
notes appear in that list with their numeric id and a true 'private' flag (or
false for public replies). The id you want is the entry's "id" field.

This endpoint is not declared in the upstream OpenAPI spec but is documented
and supported by Freshservice (DELETE /api/v2/conversations/{id}).`,
		Example: `  # Look up the conversation id, then delete the note
  freshservice-pp-cli tickets conversations list-ticket 42 --json --select conversations.id,conversations.private,conversations.body_text
  freshservice-pp-cli tickets notes delete 26000123456 --dry-run
  freshservice-pp-cli tickets notes delete 26000123456 --yes`,
		Annotations: map[string]string{
			"pp:endpoint": "notes.delete",
			"pp:method":   "DELETE",
			"pp:path":     "/conversations/{id}",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/conversations/" + args[0]

			data, statusCode, err := c.Delete(path)
			if err != nil {
				return classifyDeleteError(err, flags)
			}

			envelope := map[string]any{
				"action":          "delete",
				"resource":        "notes",
				"path":            path,
				"conversation_id": args[0],
				"status":          statusCode,
				"success":         statusCode >= 200 && statusCode < 300,
			}
			if flags.dryRun {
				envelope["dry_run"] = true
			}
			if len(data) > 0 && string(data) != "null" {
				envelope["response"] = string(data)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, envelope)
			}
			if flags.quiet {
				return nil
			}
			if envelope["success"].(bool) {
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted note (conversation %s)\n", args[0])
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Delete returned HTTP %d for conversation %s\n", statusCode, args[0])
			return nil
		},
	}
	return cmd
}
