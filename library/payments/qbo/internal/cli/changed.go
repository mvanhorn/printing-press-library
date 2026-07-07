// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/store"
	"time"

	"github.com/spf13/cobra"
)

type changeItem struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	DocNumber   string `json:"doc_number"`
	LastUpdated string `json:"last_updated"`
}

func newChangedCmd(flags *rootFlags) *cobra.Command {
	var sinceFlag string

	cmd := &cobra.Command{
		Use:     "changed",
		Short:   "Show ledger entities modified since a duration or timestamp",
		Example: "  qbo-pp-cli changed --since 24h\n  qbo-pp-cli changed --since 2026-06-04",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sinceFlag == "" {
				sinceFlag = "24h" // Default to last 24 hours
			}

			cutoff, err := parseCutoff(sinceFlag)
			if err != nil {
				return err
			}

			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			cutoffStr := cutoff.UTC().Format("2006-01-02T15:04:05Z")

			var changes []changeItem

			for _, et := range store.EntityTypes {
				query := fmt.Sprintf(`
					SELECT id, name, doc_number, last_updated 
					FROM %s 
					WHERE last_updated >= ?
					ORDER BY last_updated DESC
				`, et.TableName)

				rows, err := s.DB().Query(query, cutoffStr)
				if err != nil {
					return fmt.Errorf("querying %s: %w", et.Name, err)
				}

				for rows.Next() {
					var item changeItem
					item.Type = et.Name
					var name, docNum sql.NullString
					if err := rows.Scan(&item.ID, &name, &docNum, &item.LastUpdated); err != nil {
						rows.Close()
						return fmt.Errorf("scanning %s row: %w", et.Name, err)
					}
					item.Name = name.String
					item.DocNumber = docNum.String
					changes = append(changes, item)
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("reading %s rows: %w", et.Name, err)
				}
				rows.Close()
			}

			if flags.asJSON {
				return flags.printJSON(cmd, changes)
			}

			if len(changes) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No records modified since %s (%s).\n", sinceFlag, cutoffStr)
				return nil
			}

			headers := []string{"TYPE", "ID", "DOC NUMBER", "NAME", "LAST UPDATED"}
			var rows [][]string
			for _, c := range changes {
				rows = append(rows, []string{
					c.Type,
					c.ID,
					c.DocNumber,
					c.Name,
					c.LastUpdated,
				})
			}

			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&sinceFlag, "since", "24h", "Time cutoff as duration (e.g. 24h, 1h) or timestamp (e.g. 2026-06-04)")
	return cmd
}

func parseCutoff(since string) (time.Time, error) {
	if d, err := cliutil.ParseDurationLoose(since); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", since); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid since format: %q. Use duration (e.g. 24h, 1h) or date (e.g. 2026-06-04)", since)
}
