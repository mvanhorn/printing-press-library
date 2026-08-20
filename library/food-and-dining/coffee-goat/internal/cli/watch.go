// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// watchRow models a single watchlist entry.
type watchRow struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Query          string     `json:"query"`
	LastSyncAnchor *time.Time `json:"last_sync_anchor,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Persistent saved queries — emit only newly-arrived matching products on each run",
		Example: `  coffee-goat-pp-cli watch save kenyas "kenya"
  coffee-goat-pp-cli watch run kenyas --agent`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchSaveCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	return cmd
}

func newWatchSaveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "save <name> <query>",
		Short:   "Save a new watch entry",
		Example: `  coffee-goat-pp-cli watch save bermudez "diego bermudez gesha"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("watch save requires <name> and <query>"))
			}
			name := args[0]
			query := strings.Join(args[1:], " ")
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			// Leave last_sync_anchor NULL on save so the first
			// `watch run` reports every current match (subsequent
			// runs advance the anchor and only emit additions).
			_, err = db.DB().Exec(
				`INSERT INTO watchlist (name, query, last_sync_anchor) VALUES (?, ?, NULL)
				 ON CONFLICT(name) DO UPDATE SET query=excluded.query, last_sync_anchor=NULL`,
				name, query,
			)
			if err != nil {
				return fmt.Errorf("watch save: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "saved", "name": name, "query": query}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved watch %q -> %q\n", name, query)
			return nil
		},
	}
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved watches with name, query, last-run time, and last-seen product count",
		Example:     `  coffee-goat-pp-cli watch list`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().Query(`SELECT id, name, query, last_sync_anchor FROM watchlist ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			var watches []watchRow
			for rows.Next() {
				var w watchRow
				var anchor sql.NullTime
				if err := rows.Scan(&w.ID, &w.Name, &w.Query, &anchor); err != nil {
					return err
				}
				if anchor.Valid {
					t := anchor.Time
					w.LastSyncAnchor = &t
				}
				watches = append(watches, w)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate watchlist rows: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), watches, flags)
			}
			if len(watches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no watches saved")
				return nil
			}
			for _, w := range watches {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %q\n", w.Name, w.Query)
			}
			return nil
		},
	}
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "run [name]",
		Short:       "Run watches and emit only new matches (since last_sync_anchor)",
		Example:     `  coffee-goat-pp-cli watch run kenyas --agent`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			q := `SELECT id, name, query, last_sync_anchor FROM watchlist`
			var qargs []any
			if len(args) > 0 {
				q += ` WHERE name=?`
				qargs = append(qargs, args[0])
			}
			rows, err := db.DB().Query(q, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			var watches []watchRow
			for rows.Next() {
				var w watchRow
				var anchor sql.NullTime
				if err := rows.Scan(&w.ID, &w.Name, &w.Query, &anchor); err != nil {
					return err
				}
				if anchor.Valid {
					t := anchor.Time
					w.LastSyncAnchor = &t
				}
				watches = append(watches, w)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate watchlist rows: %w", err)
			}
			// If the caller asked for a specific watch but no row matched,
			// surface a typed not-found error so error-path probes exit non-zero.
			if len(args) > 0 && len(watches) == 0 {
				return notFoundErr(fmt.Errorf("watch %q not found; run `watch list` to see saved queries", args[0]))
			}

			type result struct {
				Watch    string      `json:"watch"`
				NewItems []searchHit `json:"new_items"`
			}
			var results []result
			for _, w := range watches {
				hits, herr := runSearch(db, w.Query, "", "", false, 0, 50)
				if herr != nil {
					continue
				}
				// Filter by first_seen_at > anchor.
				anchor := time.Time{}
				if w.LastSyncAnchor != nil {
					anchor = *w.LastSyncAnchor
				}
				var fresh []searchHit
				for _, h := range hits {
					var firstSeen time.Time
					var firstSeenStr string
					_ = db.DB().QueryRow(`SELECT first_seen_at FROM roaster_products WHERE roaster_slug=? AND handle=?`, h.Roaster, h.Handle).Scan(&firstSeenStr)
					if firstSeenStr != "" {
						t, perr := time.Parse("2006-01-02 15:04:05", firstSeenStr)
						if perr != nil {
							t, _ = time.Parse(time.RFC3339, firstSeenStr)
						}
						firstSeen = t
					}
					if firstSeen.IsZero() || firstSeen.After(anchor) {
						fresh = append(fresh, h)
					}
				}
				if len(fresh) > 0 {
					results = append(results, result{Watch: w.Name, NewItems: fresh})
				}
				// Advance anchor.
				_, _ = db.DB().Exec(`UPDATE watchlist SET last_sync_anchor = CURRENT_TIMESTAMP WHERE id = ?`, w.ID)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				// Silent on no-new-matches per spec.
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "watch %s — %d new\n", r.Watch, len(r.NewItems))
				for _, h := range r.NewItems {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s / %s — %s\n", h.Roaster, h.Title, h.Origin)
				}
			}
			return nil
		},
	}
	return cmd
}
