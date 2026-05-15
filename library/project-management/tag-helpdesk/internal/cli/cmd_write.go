// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/tag-helpdesk/internal/odoo"
)

func newCreateCmd(flags *rootFlags) *cobra.Command {
	var name, description, partnerEmail, partnerName string
	var priority string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new helpdesk ticket in Odoo",
		Example: `  tag-helpdesk-pp-cli create --name "Printer offline" --description "HP LaserJet not responding" --partner-email user@example.com
  tag-helpdesk-pp-cli create --name "Urgent: server down" --priority 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			c, err := odoo.NewFromEnv()
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return err
			}

			vals := map[string]interface{}{
				"name":        name,
				"description": description,
			}
			if partnerEmail != "" {
				vals["partner_email"] = partnerEmail
			}
			if partnerName != "" {
				vals["partner_name"] = partnerName
			}
			if priority != "" {
				vals["priority"] = priority
			}

			id, err := c.CreateTicket(vals)
			if err != nil {
				return fmt.Errorf("creating ticket: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": id, "status": "created"}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created ticket ID %d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Ticket title (required)")
	cmd.Flags().StringVar(&description, "description", "", "Ticket body")
	cmd.Flags().StringVar(&partnerEmail, "partner-email", "", "Customer email")
	cmd.Flags().StringVar(&partnerName, "partner-name", "", "Customer name")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority: 0=Low, 1=Medium, 2=High, 3=Very High")
	return cmd
}

func newUpdateCmd(flags *rootFlags) *cobra.Command {
	var stageID int
	var userID int
	var priority string
	var kanbanState string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update ticket fields in Odoo",
		Example: `  tag-helpdesk-pp-cli update 42 --priority 2
  tag-helpdesk-pp-cli update 42 --kanban-state done
  tag-helpdesk-pp-cli update 42 --stage-id 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ticket ID %q", args[0])
			}

			c, err := odoo.NewFromEnv()
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return err
			}

			vals := map[string]interface{}{}
			if cmd.Flags().Changed("stage-id") {
				vals["stage_id"] = stageID
			}
			if cmd.Flags().Changed("user-id") {
				vals["user_id"] = userID
			}
			if priority != "" {
				vals["priority"] = priority
			}
			if kanbanState != "" {
				if !isValid(kanbanState, "normal", "done", "blocked") {
					return fmt.Errorf("--kanban-state must be one of: normal, done, blocked")
				}
				vals["kanban_state"] = kanbanState
			}
			if len(vals) == 0 {
				return fmt.Errorf("no fields to update — specify at least one of --stage-id, --user-id, --priority, --kanban-state")
			}

			if err := c.UpdateTicket(id, vals); err != nil {
				return fmt.Errorf("updating ticket: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated ticket %d\n", id)
			return nil
		},
	}
	cmd.Flags().IntVar(&stageID, "stage-id", 0, "Stage ID")
	cmd.Flags().IntVar(&userID, "user-id", 0, "Assignee user ID (0 = unassign)")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority: 0=Low, 1=Medium, 2=High, 3=Very High")
	cmd.Flags().StringVar(&kanbanState, "kanban-state", "", "Kanban state: normal, done, blocked")
	return cmd
}

func newNoteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note <id> <message>",
		Short: "Post an internal note on a ticket",
		Example: `  tag-helpdesk-pp-cli note 42 "Contacted customer, awaiting callback"
  tag-helpdesk-pp-cli note 42 "Resolved — upgraded firmware to v3.2"`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ticket ID %q", args[0])
			}
			body := strings.Join(args[1:], " ")

			c, err := odoo.NewFromEnv()
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return err
			}

			msgID, err := c.PostNote(id, body)
			if err != nil {
				return fmt.Errorf("posting note: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Posted note (message ID %d) on ticket %d\n", msgID, id)
			return nil
		},
	}
	return cmd
}

func isValid(val string, options ...string) bool {
	for _, o := range options {
		if val == o {
			return true
		}
	}
	return false
}
