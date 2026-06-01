// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type postPair struct {
	EnID          string `json:"en_id"`
	DeID          string `json:"de_id"`
	EnTitle       string `json:"en_title,omitempty"`
	DeTitle       string `json:"de_title,omitempty"`
	LinkedAt      string `json:"linked_at"`
	EnPublication string `json:"en_publication,omitempty"`
	DePublication string `json:"de_publication,omitempty"`
}

type unpairedPost struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Publication string `json:"publication_id"`
	PublishDate string `json:"publish_date"`
	Language    string `json:"detected_language,omitempty"`
}

func ensurePostPairsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS post_pairs (
		en_id TEXT NOT NULL,
		de_id TEXT NOT NULL,
		linked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (en_id, de_id)
	)`)
	return err
}

func newPostsPairCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "pair <en-id-or-slug> <de-id-or-slug>",
		Short: "Record an EN ↔ DE post pairing for the bilingual-pair tracker.",
		Long: `Records a translation pairing between two posts you own. Resolves slugs to IDs
against the local posts cache before saving.`,
		Example:     "  substack-pp-cli posts pair my-en-slug my-de-slug",
		Annotations: map[string]string{"pp:novel": "posts pair"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()
			if err := ensurePostPairsTable(cmd.Context(), db); err != nil {
				return err
			}
			enID, err := resolvePostRef(cmd.Context(), db, args[0])
			if err != nil {
				return fmt.Errorf("resolving EN post %q: %w", args[0], err)
			}
			deID, err := resolvePostRef(cmd.Context(), db, args[1])
			if err != nil {
				return fmt.Errorf("resolving DE post %q: %w", args[1], err)
			}
			_, err = db.ExecContext(cmd.Context(),
				`INSERT OR REPLACE INTO post_pairs (en_id, de_id) VALUES (?, ?)`, enID, deID)
			if err != nil {
				return fmt.Errorf("recording pair: %w", err)
			}
			if flags.asJSON {
				raw, _ := json.Marshal(map[string]string{"en_id": enID, "de_id": deID, "status": "linked"})
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Linked %s ↔ %s\n", enID, deID)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newPostsPairsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		missing     bool
		publication string
	)
	cmd := &cobra.Command{
		Use:   "pairs",
		Short: "List recorded EN ↔ DE post pairings, or posts without a twin.",
		Long: `Without --missing, lists every recorded post pair from the local table.
With --missing, lists posts that have no paired translation recorded — perfect
input for 'posts twin' to spin up the missing translations as drafts.`,
		Example: `  substack-pp-cli posts pairs --json
  substack-pp-cli posts pairs --missing --publication mypub-en --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "posts pairs"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()
			if err := ensurePostPairsTable(cmd.Context(), db); err != nil {
				return err
			}

			if missing {
				// Posts not present as en_id or de_id in post_pairs
				q := `SELECT id, COALESCE(title, ''), COALESCE(publication_id, ''), COALESCE(publish_date, '')
				      FROM posts WHERE id NOT IN (SELECT en_id FROM post_pairs UNION SELECT de_id FROM post_pairs)`
				args2 := []any{}
				if publication != "" {
					q += ` AND (publication_id = ? OR publication_id IN (SELECT id FROM publications WHERE subdomain = ?))`
					args2 = append(args2, publication, publication)
				}
				q += ` ORDER BY publish_date DESC LIMIT 500`
				rows, err := db.QueryContext(cmd.Context(), q, args2...)
				if err != nil {
					return err
				}
				defer rows.Close()
				var out []unpairedPost
				for rows.Next() {
					var p unpairedPost
					if err := rows.Scan(&p.ID, &p.Title, &p.Publication, &p.PublishDate); err != nil {
						return err
					}
					out = append(out, p)
				}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					raw, _ := json.Marshal(out)
					return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
				}
				w := cmd.OutOrStdout()
				if len(out) == 0 {
					fmt.Fprintln(w, "No unpaired posts.")
					return nil
				}
				fmt.Fprintf(w, "%d unpaired post(s):\n", len(out))
				for _, p := range out {
					fmt.Fprintf(w, "  %s — %-30s [%s] %s\n",
						truncate(p.ID, 14), truncate(p.Title, 30), truncate(p.Publication, 14), p.PublishDate)
				}
				return nil
			}

			// All pairs with joined titles
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT pp.en_id, pp.de_id, pp.linked_at,
				       COALESCE(en.title, ''), COALESCE(de.title, ''),
				       COALESCE(en.publication_id, ''), COALESCE(de.publication_id, '')
				FROM post_pairs pp
				LEFT JOIN posts en ON en.id = pp.en_id
				LEFT JOIN posts de ON de.id = pp.de_id
				ORDER BY pp.linked_at DESC
			`)
			if err != nil {
				return err
			}
			defer rows.Close()
			var out []postPair
			for rows.Next() {
				var p postPair
				if err := rows.Scan(&p.EnID, &p.DeID, &p.LinkedAt, &p.EnTitle, &p.DeTitle, &p.EnPublication, &p.DePublication); err != nil {
					return err
				}
				out = append(out, p)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No pairs recorded yet. Run 'posts pair <en> <de>'.")
				return nil
			}
			fmt.Fprintf(w, "%d pair(s):\n", len(out))
			fmt.Fprintln(w, strings.Repeat("─", 78))
			for _, p := range out {
				fmt.Fprintf(w, "EN: %-30s [%s]\n", truncate(p.EnTitle, 30), truncate(p.EnPublication, 14))
				fmt.Fprintf(w, "DE: %-30s [%s]\n", truncate(p.DeTitle, 30), truncate(p.DePublication, 14))
				fmt.Fprintf(w, "    linked %s\n\n", p.LinkedAt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&missing, "missing", false, "Show posts without a recorded twin")
	cmd.Flags().StringVar(&publication, "publication", "", "Filter to a single publication")
	return cmd
}

// resolvePostRef accepts a post ID or slug and returns the post ID.
func resolvePostRef(ctx context.Context, db *sql.DB, ref string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM posts WHERE id = ? OR slug = ? LIMIT 1`, ref, ref).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("post not found in local cache; run 'sync --full' first")
		}
		return "", err
	}
	return id, nil
}
