// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Phase 3: personal reading list — queue DOIs locally and track read/unread state.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/nejm/internal/store"
	"github.com/spf13/cobra"
)

// ensureReadingListTable creates the reading_list table if it doesn't exist.
func ensureReadingListTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS reading_list (
		doi TEXT PRIMARY KEY,
		added_at TEXT NOT NULL,
		read_at TEXT,
		notes TEXT
	)`)
	return err
}

func newNovelReadingListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reading-list",
		Short:       "Queue NEJM articles by DOI locally and track read/unread state.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}

	// add
	addCmd := &cobra.Command{
		Use:     "add <doi>",
		Short:   "Add an article to the reading list",
		Example: "  nejm-pp-cli reading-list add 10.1056/NEJMoa2506905",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			doi := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("nejm-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := ensureReadingListTable(db); err != nil {
				return err
			}
			_, err = db.DB().ExecContext(ctx,
				`INSERT INTO reading_list (doi, added_at) VALUES (?, ?)
				 ON CONFLICT(doi) DO NOTHING`,
				doi, time.Now().UTC().Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("adding to reading list: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s to reading list\n", doi)
			return nil
		},
	}

	// ls
	lsCmd := &cobra.Command{
		Use:         "ls",
		Short:       "List articles in the reading list",
		Aliases:     []string{"list"},
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli reading-list ls\n  nejm-pp-cli reading-list ls --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("nejm-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := ensureReadingListTable(db); err != nil {
				return err
			}

			rows, err := db.DB().QueryContext(ctx,
				`SELECT r.doi, r.added_at, r.read_at, a.title, a.authors
				 FROM reading_list r
				 LEFT JOIN article a ON a.doi = r.doi
				 ORDER BY r.added_at DESC`)
			if err != nil {
				return fmt.Errorf("querying reading list: %w", err)
			}
			defer rows.Close()

			type rlRow struct {
				DOI     string `json:"doi"`
				AddedAt string `json:"added_at"`
				ReadAt  string `json:"read_at,omitempty"`
				Title   string `json:"title,omitempty"`
				Authors string `json:"authors,omitempty"`
				Read    bool   `json:"read"`
			}
			results := make([]rlRow, 0)
			for rows.Next() {
				var doi, addedAt string
				var readAtNull *string
				var title sql.NullString
				var authors sql.NullString

				if err := rows.Scan(&doi, &addedAt, &readAtNull, &title, &authors); err != nil {
					continue
				}

				titleStr := ""
				if title.Valid {
					titleStr = title.String
				}
				authorsStr := ""
				if authors.Valid {
					authorsStr = authors.String
				}

				readAtStr := ""
				read := false
				if readAtNull != nil {
					readAtStr = *readAtNull
					read = true
				}

				results = append(results, rlRow{
					DOI:     doi,
					AddedAt: addedAt,
					ReadAt:  readAtStr,
					Title:   titleStr,
					Authors: authorsStr,
					Read:    read,
				})
			}

			// Surface database errors raised during row iteration.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading list rows error: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "reading list is empty")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Reading List (%d entries)\n", len(results))
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))
			for _, r := range results {
				status := "[ ]"
				if r.Read {
					status = "[✓]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", status, r.DOI)
				if r.Title != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", r.Title)
				}
				if r.Authors != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    by %s\n", r.Authors)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "    added: %s\n", r.AddedAt)
				if r.Read {
					fmt.Fprintf(cmd.OutOrStdout(), "    read: %s\n", r.ReadAt)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	// read — mark an article as read.
	//
	// Without this command the read_at column, the [✓] indicator in ls, and the
	// "read" field in JSON output were unreachable: nothing ever wrote to
	// read_at, so every entry rendered as unread forever.
	readCmd := &cobra.Command{
		Use:     "read <doi>",
		Short:   "Mark an article in the reading list as read",
		Example: "  nejm-pp-cli reading-list read 10.1056/NEJMoa2506905",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			doi := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("nejm-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := ensureReadingListTable(db); err != nil {
				return err
			}
			res, err := db.DB().ExecContext(ctx,
				`UPDATE reading_list SET read_at = ? WHERE doi = ?`,
				time.Now().UTC().Format(time.RFC3339), doi)
			if err != nil {
				return fmt.Errorf("marking as read: %w", err)
			}
			// A DOI that is not on the list is a user error, not a silent no-op:
			// otherwise a typo would report success and leave nothing marked.
			n, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("marking as read: %w", err)
			}
			if n == 0 {
				return fmt.Errorf("%s is not in the reading list; add it first with 'reading-list add %s'", doi, doi)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "marked %s as read\n", doi)
			return nil
		},
	}

	// unread — clear the read mark.
	//
	// A tracked state has to be reversible; without this, marking an article
	// read by mistake could only be undone by removing and re-adding it.
	unreadCmd := &cobra.Command{
		Use:     "unread <doi>",
		Short:   "Mark an article in the reading list as unread",
		Example: "  nejm-pp-cli reading-list unread 10.1056/NEJMoa2506905",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			doi := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("nejm-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := ensureReadingListTable(db); err != nil {
				return err
			}
			res, err := db.DB().ExecContext(ctx,
				`UPDATE reading_list SET read_at = NULL WHERE doi = ?`, doi)
			if err != nil {
				return fmt.Errorf("marking as unread: %w", err)
			}
			n, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("marking as unread: %w", err)
			}
			if n == 0 {
				return fmt.Errorf("%s is not in the reading list", doi)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "marked %s as unread\n", doi)
			return nil
		},
	}

	cmd.AddCommand(addCmd, lsCmd, readCmd, unreadCmd)
	return cmd
}
