// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// newTailCmd shows the most-recently-synced rows across the corpus. Useful
// after a sync to confirm new arrivals and quickly scan what changed.
// Read-only; no remote calls.
func newTailCmd(flags *rootFlags) *cobra.Command {
	var since string
	var resourceFilter string
	var limit int
	cmd := &cobra.Command{
		Use:         "tail",
		Short:       "Show recently-synced rows across the corpus (newest first) — products, reviews, videos",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  coffee-goat-pp-cli tail
  coffee-goat-pp-cli tail --since 24h
  coffee-goat-pp-cli tail --resource products --limit 50
  coffee-goat-pp-cli tail --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cutoff, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since %q: %w (try 24h, 7d, 1h)", since, err)
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			out := map[string][]tailRow{}
			total := 0
			if resourceFilter == "" || resourceFilter == "products" {
				rows, err := tailProducts(ctx, db.DB(), cutoff, limit)
				if err != nil {
					return err
				}
				out["products"] = rows
				total += len(rows)
			}
			if resourceFilter == "" || resourceFilter == "reviews" {
				rows, err := tailReviews(ctx, db.DB(), cutoff, limit)
				if err != nil {
					return err
				}
				out["reviews"] = rows
				total += len(rows)
			}
			if resourceFilter == "" || resourceFilter == "videos" {
				rows, err := tailVideos(ctx, db.DB(), cutoff, limit)
				if err != nil {
					return err
				}
				out["videos"] = rows
				total += len(rows)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"by_resource": out,
					"total":       total,
					"since":       since,
				}, flags)
			}
			w := cmd.OutOrStdout()
			if total == 0 {
				fmt.Fprintf(w, "  No recent rows. Run 'coffee-goat-pp-cli sync' to populate the local store.\n")
				return nil
			}
			for _, kind := range []string{"products", "reviews", "videos"} {
				rows, ok := out[kind]
				if !ok || len(rows) == 0 {
					continue
				}
				fmt.Fprintf(w, "\n  [%s] %d rows\n", kind, len(rows))
				for _, r := range rows {
					fmt.Fprintf(w, "    %s  %s  %s\n", r.SyncedAt, r.ID, r.Summary)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only rows synced within this window (e.g. 24h, 7d, 1h)")
	cmd.Flags().StringVar(&resourceFilter, "resource", "", "Limit to one resource: products | reviews | videos")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max rows per resource")
	return cmd
}

type tailRow struct {
	ID       string `json:"id"`
	SyncedAt string `json:"synced_at"`
	Summary  string `json:"summary"`
}

func tailProducts(ctx context.Context, db *sql.DB, cutoff sql.NullTime, limit int) ([]tailRow, error) {
	q := `SELECT roaster_slug || '/' || handle AS id, last_seen_at, COALESCE(title,'') || ' — ' || COALESCE(origin,'?') AS summary
	      FROM roaster_products
	      WHERE 1=1`
	args := []any{}
	if cutoff.Valid {
		q += ` AND last_seen_at >= ?`
		args = append(args, cutoff.Time)
	}
	q += ` ORDER BY last_seen_at DESC LIMIT ?`
	args = append(args, limit)
	return queryTailRows(ctx, db, q, args...)
}

func tailReviews(ctx context.Context, db *sql.DB, cutoff sql.NullTime, limit int) ([]tailRow, error) {
	q := `SELECT id, COALESCE(last_seen_at, published_at) AS synced_at,
	             COALESCE(roaster_name,'') || ' — ' || COALESCE(bean_name,'') AS summary
	      FROM reviews
	      WHERE 1=1`
	args := []any{}
	if cutoff.Valid {
		q += ` AND COALESCE(last_seen_at, published_at) >= ?`
		args = append(args, cutoff.Time)
	}
	q += ` ORDER BY synced_at DESC LIMIT ?`
	args = append(args, limit)
	return queryTailRows(ctx, db, q, args...)
}

func tailVideos(ctx context.Context, db *sql.DB, cutoff sql.NullTime, limit int) ([]tailRow, error) {
	q := `SELECT video_id, last_synced_at,
	             COALESCE(creator,'') || ' — ' || COALESCE(video_title,'') AS summary
	      FROM youtube_reviews
	      WHERE 1=1`
	args := []any{}
	if cutoff.Valid {
		q += ` AND last_synced_at >= ?`
		args = append(args, cutoff.Time)
	}
	q += ` ORDER BY last_synced_at DESC LIMIT ?`
	args = append(args, limit)
	return queryTailRows(ctx, db, q, args...)
}

func queryTailRows(ctx context.Context, db *sql.DB, q string, args ...any) ([]tailRow, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		// Missing table → empty (don't error: youtube_reviews might be empty
		// on a fresh install where sync has only hit shopify so far).
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []tailRow
	for rows.Next() {
		var r tailRow
		var syncedAt sql.NullTime
		if err := rows.Scan(&r.ID, &syncedAt, &r.Summary); err != nil {
			continue
		}
		if syncedAt.Valid {
			r.SyncedAt = syncedAt.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tail rows: %w", err)
	}
	return out, nil
}

// parseSince accepts shorthand like "24h", "7d", "1h" and returns a SQL
// time cutoff. Empty string is permitted (returns invalid NullTime).
func parseSince(s string) (sql.NullTime, error) {
	if s == "" {
		return sql.NullTime{}, nil
	}
	// "7d" is convenient shorthand; time.ParseDuration only accepts h/m/s.
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil || n <= 0 {
			return sql.NullTime{}, fmt.Errorf("bad day count")
		}
		return sql.NullTime{Time: time.Now().Add(-time.Duration(n) * 24 * time.Hour), Valid: true}, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: time.Now().Add(-d), Valid: true}, nil
}
