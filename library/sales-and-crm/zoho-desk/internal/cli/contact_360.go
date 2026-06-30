// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented RunE body.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelContact360Cmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	type tixRow struct {
		ID           string `json:"id"`
		TicketNumber string `json:"ticketNumber"`
		Subject      string `json:"subject"`
		Status       string `json:"status"`
		Priority     string `json:"priority"`
		DueDate      string `json:"dueDate"`
	}

	cmd := &cobra.Command{
		Use:         "contact-360 <email>",
		Short:       "Everything about one customer: their contact record, account, and tickets in a single view.",
		Example:     "  zoho-desk-pp-cli contact-360 jane@acme.com --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("an <email> argument is required"))
			}
			email := strings.TrimSpace(args[0])
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()

			contacts, err := loadByType(cmd.Context(), db, "contacts")
			if err != nil {
				return fmt.Errorf("reading contacts: %w", err)
			}

			var contact map[string]any
			for _, c := range contacts {
				if strings.EqualFold(strings.TrimSpace(str(c, "email")), email) {
					contact = c
					break
				}
			}

			if contact == nil {
				view := struct {
					Contact any    `json:"contact"`
					Note    string `json:"note"`
				}{
					Contact: nil,
					Note:    fmt.Sprintf("no contact found matching email %q; run 'zoho-desk-pp-cli sync' to refresh local data", email),
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			contactID := str(contact, "id")
			accountID := str(contact, "accountId")

			var account map[string]any
			if accountID != "" {
				accounts, _ := loadByType(cmd.Context(), db, "accounts")
				for _, a := range accounts {
					if str(a, "id") == accountID {
						account = a
						break
					}
				}
			}

			tickets, err := loadTickets(cmd.Context(), db)
			if err != nil {
				return fmt.Errorf("reading tickets: %w", err)
			}
			rows := make([]tixRow, 0)
			openCount := 0
			for _, t := range tickets {
				if str(t, "contactId") != contactID {
					continue
				}
				if !isClosedStatus(str(t, "status")) {
					openCount++
				}
				rows = append(rows, tixRow{
					ID:           str(t, "id"),
					TicketNumber: str(t, "ticketNumber"),
					Subject:      str(t, "subject"),
					Status:       str(t, "status"),
					Priority:     str(t, "priority"),
					DueDate:      str(t, "dueDate"),
				})
			}

			view := struct {
				Contact     map[string]any `json:"contact"`
				Account     map[string]any `json:"account"`
				Tickets     []tixRow       `json:"tickets"`
				TicketCount int            `json:"ticketCount"`
				OpenCount   int            `json:"openCount"`
			}{
				Contact:     contact,
				Account:     account,
				Tickets:     rows,
				TicketCount: len(rows),
				OpenCount:   openCount,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
