// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add a local chronological calendar combining events, payouts, and renewals.

package cli

import (
	"database/sql"
	"sort"

	"github.com/spf13/cobra"
)

type calendarRow struct {
	Date      string `json:"date"`
	EventName string `json:"event_name"`
	Program   string `json:"program,omitempty"`
	Type      string `json:"type"`
}

func newEventsCalendarCmd(flags *rootFlags) *cobra.Command {
	var mine bool
	var limit int
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Show local events, payout due dates, and invitation renewals chronologically",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDefaultStore()
			if err != nil {
				return err
			}
			defer db.Close()
			invited, err := invitedProgramSet(db)
			if err != nil {
				return err
			}
			var rows []calendarRow
			events, err := loadResourceObjects(db, "events")
			if err != nil {
				return err
			}
			for _, e := range events {
				program := hacktivityProgramSlug(e)
				if mine && program != "" && !invited[program] {
					continue
				}
				if mine && program == "" && len(invited) > 0 {
					continue
				}
				rows = append(rows, calendarRow{
					Date:      stringAt(e, "date", "starts_at", "start_at", "start_date", "created_at"),
					EventName: stringAt(e, "name", "title", "event_name"),
					Program:   program,
					Type:      "event",
				})
			}
			reports, err := loadResourceObjects(db, "user-reports")
			if err != nil {
				return err
			}
			for _, r := range reports {
				state := stringAt(r, "state", "status")
				if state != "awaiting_payout" && state != "payout_pending" && state != "pending_payout" {
					continue
				}
				rows = append(rows, calendarRow{
					Date:      stringAt(r, "payout_due_at", "payout_at", "updated_at", "created_at"),
					EventName: hacktivityTitle(r),
					Program:   hacktivityProgramSlug(r),
					Type:      "payout-due",
				})
			}
			invs, err := loadResourceObjects(db, "user-invitations")
			if err != nil {
				return err
			}
			for _, inv := range invs {
				date := stringAt(inv, "renewal_at", "expires_at", "expiration_at")
				if date == "" {
					continue
				}
				rows = append(rows, calendarRow{
					Date:      date,
					EventName: "Invitation renewal",
					Program:   hacktivityProgramSlug(inv),
					Type:      "renewal",
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Date < rows[j].Date })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "Only include events tied to invited programs")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum calendar rows")
	return cmd
}

func invitedProgramSet(db interface {
	Query(string, ...any) (*sql.Rows, error)
}) (map[string]bool, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ?`, "user-invitations")
	if err != nil {
		return nil, err
	}
	objs, err := sqlRowsToObjects(rows)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, obj := range objs {
		if slug := hacktivityProgramSlug(obj); slug != "" {
			out[slug] = true
		}
	}
	return out, nil
}
