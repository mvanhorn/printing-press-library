// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (NOT generated).
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/internal/store"
)

type watchChange struct {
	APIID string `json:"api_id"`
	Name  string `json:"name"`
	Field string `json:"field"`
	Old   int    `json:"old"`
	New   int    `json:"new"`
	Delta int    `json:"delta"`
}

type watchView struct {
	Filter   string          `json:"filter"`
	Baseline bool            `json:"baseline"`
	Added    []lumaEventView `json:"added"`
	Removed  []lumaEventView `json:"removed"`
	Changed  []watchChange   `json:"changed"`
	Current  int             `json:"current_count"`
	Note     string          `json:"note,omitempty"`
}

type snapRow struct {
	guest  int
	ticket int
	start  string
	name   string
}

func ensureWatchTable(ctx context.Context, db *store.Store) error {
	_, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS luma_watch_snapshots (
		filter_key   TEXT NOT NULL,
		api_id       TEXT NOT NULL,
		snapshot_at  INTEGER NOT NULL,
		guest_count  INTEGER,
		ticket_count INTEGER,
		start_at     TEXT,
		name         TEXT,
		PRIMARY KEY (filter_key, api_id)
	)`)
	return err
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagPlaceID string
	var flagCategory string
	var flagLimit int
	var flagMaxScanPages int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Show what changed for a city/category's events since the previous watch run.",
		Long: "Fetch the current events for a city, place, or category and diff them against the last\n" +
			"watch run for that same filter: new events, removed events, and guest/ticket count changes.\n" +
			"The public API is stateless, so this keeps a local snapshot to compute change over time.\n\n" +
			"The first run for a filter captures a baseline; subsequent runs report the deltas.",
		Example:     "  luma-pp-cli watch --city sf --category cat-ai --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch current events and diff against the last snapshot")
				return nil
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("watch has no local-only data source; it fetches current events live and diffs against local history"))
			}
			if flagCity == "" && flagPlaceID == "" && flagCategory == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("watch needs one of --city, --place-id, or --category"))
			}

			params := map[string]string{}
			var parts []string
			if flagCity != "" {
				params["slug"] = flagCity
				parts = append(parts, "city:"+flagCity)
			}
			if flagPlaceID != "" {
				params["discover_place_api_id"] = flagPlaceID
				parts = append(parts, "place:"+flagPlaceID)
			}
			if flagCategory != "" {
				params["category_api_id"] = flagCategory
				parts = append(parts, "category:"+flagCategory)
			}
			filterKey := strings.Join(parts, "|")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Page size is the upstream fetch batch, independent of --limit
			// (the per-run tracking cap applied via dedupe/window below).
			const watchPageSize = 50
			entries, ferr := fetchEventEntries(ctx, c, params, watchPageSize, scanPagesForEnv(flagMaxScanPages))
			if ferr != nil && len(entries) == 0 {
				return classifyAPIError(ferr, flags)
			}
			entries = dedupeByID(entries)

			if flagDB == "" {
				flagDB = defaultDBPath("luma-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, flagDB)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := ensureWatchTable(ctx, db); err != nil {
				return fmt.Errorf("creating snapshot table: %w", err)
			}

			// Load prior snapshot for this filter.
			prior := map[string]snapRow{}
			rows, err := db.DB().QueryContext(ctx, `SELECT api_id, guest_count, ticket_count, start_at, name FROM luma_watch_snapshots WHERE filter_key = ?`, filterKey)
			if err != nil {
				return fmt.Errorf("reading snapshot: %w", err)
			}
			for rows.Next() {
				var id string
				var r snapRow
				if err := rows.Scan(&id, &r.guest, &r.ticket, &r.start, &r.name); err == nil {
					prior[id] = r
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close() // best-effort; the iteration error below is the real failure
				return fmt.Errorf("reading snapshot rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing snapshot rows: %w", err)
			}
			baseline := len(prior) == 0

			view := watchView{
				Filter:   filterKey,
				Baseline: baseline,
				Added:    make([]lumaEventView, 0),
				Removed:  make([]lumaEventView, 0),
				Changed:  make([]watchChange, 0),
				Current:  len(entries),
			}

			current := map[string]lumaEntry{}
			for _, e := range entries {
				id := e.id()
				if id == "" {
					continue
				}
				current[id] = e
				if baseline {
					continue
				}
				p, existed := prior[id]
				if !existed {
					view.Added = append(view.Added, e.view())
					continue
				}
				if e.GuestCount != p.guest {
					view.Changed = append(view.Changed, watchChange{APIID: id, Name: e.view().Name, Field: "guest_count", Old: p.guest, New: e.GuestCount, Delta: e.GuestCount - p.guest})
				}
				if e.TicketCount != p.ticket {
					view.Changed = append(view.Changed, watchChange{APIID: id, Name: e.view().Name, Field: "ticket_count", Old: p.ticket, New: e.TicketCount, Delta: e.TicketCount - p.ticket})
				}
			}
			if !baseline {
				for id, p := range prior {
					if _, ok := current[id]; !ok {
						view.Removed = append(view.Removed, lumaEventView{APIID: id, Name: p.name, StartAt: p.start})
					}
				}
			}
			sort.SliceStable(view.Changed, func(i, j int) bool { return abs(view.Changed[i].Delta) > abs(view.Changed[j].Delta) })

			// Persist the new snapshot (replace this filter's rows).
			tx, err := db.DB().BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("snapshot tx: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM luma_watch_snapshots WHERE filter_key = ?`, filterKey); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("clearing snapshot: %w", err)
			}
			stamp := time.Now().Unix()
			for id, e := range current {
				v := e.view()
				if _, err := tx.ExecContext(ctx, `INSERT INTO luma_watch_snapshots (filter_key, api_id, snapshot_at, guest_count, ticket_count, start_at, name) VALUES (?,?,?,?,?,?,?)`,
					filterKey, id, stamp, e.GuestCount, e.TicketCount, e.Event.StartAt, v.Name); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("writing snapshot: %w", err)
				}
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing snapshot: %w", err)
			}

			if baseline {
				view.Note = fmt.Sprintf("baseline captured for %d events; run watch again later to see changes", len(current))
			} else if len(view.Added) == 0 && len(view.Removed) == 0 && len(view.Changed) == 0 {
				view.Note = "no changes since last run"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City slug to watch, e.g. sf")
	cmd.Flags().StringVar(&flagPlaceID, "place-id", "", "Place api_id to watch (alternative to --city)")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Category api_id to watch, e.g. cat-ai")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max events to track per run")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 4, "Max pages to fetch before stopping")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/luma-pp-cli/data.db)")
	return cmd
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
