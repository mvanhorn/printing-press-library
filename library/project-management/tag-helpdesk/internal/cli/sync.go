// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/tag-helpdesk/internal/odoo"
	"github.com/mvanhorn/printing-press-library/library/project-management/tag-helpdesk/internal/store"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var force bool
	var batchSize int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull helpdesk tickets from Odoo into local SQLite cache",
		Long: `Connects to the Odoo instance and fetches all helpdesk.ticket records
into a local SQLite database (~/.tag-helpdesk/tickets.db).

On first run, fetches all tickets. On subsequent runs, fetches only tickets
modified since the last sync (incremental). Use --force to re-fetch everything.`,
		Example: `  tag-helpdesk-pp-cli sync
  tag-helpdesk-pp-cli sync --force
  tag-helpdesk-pp-cli sync --batch 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromEnv()
			if err != nil {
				return err
			}
			progress := cmd.OutOrStdout()
			if flags.asJSON {
				progress = os.Stderr
			}
			fmt.Fprintf(progress, "Authenticating with %s...\n", c.URL)
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			fmt.Fprintf(progress, "Authenticated as UID %d\n", c.UID)

			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			lastSync := db.GetMeta("last_sync")
			var domain []interface{}
			if force || lastSync == "" {
				domain = []interface{}{[]interface{}{"active", "in", []interface{}{true, false}}}
				fmt.Fprintf(progress, "Full sync (fetching all tickets)...\n")
			} else {
				domain = []interface{}{
					[]interface{}{"write_date", ">=", lastSync},
					[]interface{}{"active", "in", []interface{}{true, false}},
				}
				fmt.Fprintf(progress, "Incremental sync (since %s)...\n", lastSync)
			}

			total := 0
			offset := 0
			for {
				batch, err := c.SearchTickets(domain, batchSize, offset, "id asc")
				if err != nil {
					return fmt.Errorf("fetching tickets: %w", err)
				}
				if len(batch) == 0 {
					break
				}
				rows := make([]store.TicketRow, 0, len(batch))
				for _, t := range batch {
					rows = append(rows, ticketMapToRow(t))
				}
				if err := db.UpsertTickets(rows); err != nil {
					return fmt.Errorf("storing tickets: %w", err)
				}
				total += len(batch)
				fmt.Fprintf(progress, "  synced %d tickets...\n", total)
				if len(batch) < batchSize {
					break
				}
				offset += batchSize
			}

			now := time.Now().UTC().Format("2006-01-02 15:04:05")
			if err := db.SetMeta("last_sync", now); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save sync timestamp: %v\n", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"synced": total,
					"total":  db.Count(),
					"status": "ok",
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSync complete: %d tickets synced. Total in store: %d\n", total, db.Count())
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Full resync — ignore last sync timestamp")
	cmd.Flags().IntVar(&batchSize, "batch", 100, "Records per Odoo API call")
	return cmd
}

// ticketMapToRow converts the raw map from Odoo XML-RPC into a store.TicketRow.
func ticketMapToRow(t map[string]interface{}) store.TicketRow {
	return store.TicketRow{
		ID:              odoo.IntVal(t["id"]),
		Number:          odoo.StringVal(t["number"]),
		Name:            odoo.StringVal(t["name"]),
		DescriptionText: odoo.StripHTML(odoo.StringVal(t["description"])),
		PartnerID:       odoo.IDFromMany2one(t["partner_id"]),
		PartnerName:     odoo.StringVal(t["partner_name"]),
		PartnerEmail:    odoo.StringVal(t["partner_email"]),
		UserID:          odoo.IDFromMany2one(t["user_id"]),
		UserName:        odoo.NameFromMany2one(t["user_id"]),
		TeamID:          odoo.IDFromMany2one(t["team_id"]),
		TeamName:        odoo.NameFromMany2one(t["team_id"]),
		StageID:         odoo.IDFromMany2one(t["stage_id"]),
		StageName:       odoo.NameFromMany2one(t["stage_id"]),
		Priority:        odoo.StringVal(t["priority"]),
		KanbanState:     odoo.StringVal(t["kanban_state"]),
		CategoryID:      odoo.IDFromMany2one(t["category_id"]),
		CategoryName:    odoo.NameFromMany2one(t["category_id"]),
		ChannelID:       odoo.IDFromMany2one(t["channel_id"]),
		ChannelName:     odoo.NameFromMany2one(t["channel_id"]),
		AssignedDate:    odoo.StringVal(t["assigned_date"]),
		ClosedDate:      odoo.StringVal(t["closed_date"]),
		Closed:          odoo.BoolVal(t["closed"]),
		Unattended:      odoo.BoolVal(t["unattended"]),
		LastStageUpdate: odoo.StringVal(t["last_stage_update"]),
		WriteDate:       odoo.StringVal(t["write_date"]),
		Active:          odoo.BoolVal(t["active"]),
	}
}

func openStore() (*store.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, ".tag-helpdesk", "tickets.db")
	return store.Open(dbPath)
}
