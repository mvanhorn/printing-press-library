// PATCH(novel-feature): events watchlist save|run|ls|rm — persistent named
// filter sets stored in a new watchlists table in the local SQLite database.
// Hand-authored on top of the generator output.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

// watchlistsTableDDL creates the watchlists table on demand. The generator
// owns the canonical schema migrations; this command introduces an extension
// table that's idempotent and isolated from the rest of the store.
const watchlistsTableDDL = `CREATE TABLE IF NOT EXISTS watchlists (
	name TEXT PRIMARY KEY,
	description TEXT,
	venue_ids TEXT,
	attraction_ids TEXT,
	segments TEXT,
	genres TEXT,
	dma_ids TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`

func ensureWatchlistsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, watchlistsTableDDL)
	return err
}

type watchlist struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	VenueIDs       []string `json:"venue_ids,omitempty"`
	AttractionIDs  []string `json:"attraction_ids,omitempty"`
	Segments       []string `json:"segments,omitempty"`
	Genres         []string `json:"genres,omitempty"`
	DMAIDs         []string `json:"dma_ids,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

func newEventsWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Save, list, run, and remove named filter sets in the local store",
		Long: strings.TrimSpace(`
Persistent named watchlists stored in the local SQLite database. Each
watchlist is a named set of filters (venue IDs, attraction IDs, segments,
genres, DMA IDs) you can compose once and re-apply later via 'watchlist run'.

This is the generic primitive behind curated 'my venues' / 'my artists'
recipes — same shape regardless of metro.
`),
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	cmd.AddCommand(newEventsWatchlistSaveCmd(flags))
	cmd.AddCommand(newEventsWatchlistLsCmd(flags))
	cmd.AddCommand(newEventsWatchlistRmCmd(flags))
	cmd.AddCommand(newEventsWatchlistRunCmd(flags))
	return cmd
}

func newEventsWatchlistSaveCmd(flags *rootFlags) *cobra.Command {
	var description string
	var venueIDs, attractionIDs, segments, genres, dmaIDs string

	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save or update a named watchlist",
		Example: strings.Trim(`
  ticketmaster-pp-cli events watchlist save seattle \
    --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA \
    --description "Seattle marquee venues"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(args[0])
			if name == "" {
				return usageErr(fmt.Errorf("name required"))
			}
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := ensureWatchlistsTable(cmd.Context(), db.DB()); err != nil {
				return fmt.Errorf("watchlists table: %w", err)
			}
			wl := watchlist{
				Name:          name,
				Description:   description,
				VenueIDs:      splitCSV(venueIDs),
				AttractionIDs: splitCSV(attractionIDs),
				Segments:      splitCSV(segments),
				Genres:        splitCSV(genres),
				DMAIDs:        splitCSV(dmaIDs),
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
			}
			if _, err := db.DB().ExecContext(cmd.Context(),
				`INSERT INTO watchlists (name, description, venue_ids, attraction_ids, segments, genres, dma_ids, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
				 ON CONFLICT(name) DO UPDATE SET
				   description=excluded.description,
				   venue_ids=excluded.venue_ids,
				   attraction_ids=excluded.attraction_ids,
				   segments=excluded.segments,
				   genres=excluded.genres,
				   dma_ids=excluded.dma_ids,
				   updated_at=CURRENT_TIMESTAMP`,
				name, description,
				strings.Join(wl.VenueIDs, ","),
				strings.Join(wl.AttractionIDs, ","),
				strings.Join(wl.Segments, ","),
				strings.Join(wl.Genres, ","),
				strings.Join(wl.DMAIDs, ","),
			); err != nil {
				return fmt.Errorf("save watchlist: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), wl, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved watchlist %q (venues=%d attractions=%d segments=%d dmas=%d)\n",
				name, len(wl.VenueIDs), len(wl.AttractionIDs), len(wl.Segments), len(wl.DMAIDs))
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Human-readable description")
	cmd.Flags().StringVar(&venueIDs, "venue-ids", "", "Comma-separated Ticketmaster venue IDs (e.g. KovZ917Ahkk,KovZpZAFkvEA)")
	cmd.Flags().StringVar(&attractionIDs, "attraction-ids", "", "Comma-separated Ticketmaster attraction IDs")
	cmd.Flags().StringVar(&segments, "segments", "", "Comma-separated classification segment names (e.g. Music,Arts & Theatre)")
	cmd.Flags().StringVar(&genres, "genres", "", "Comma-separated genre names")
	cmd.Flags().StringVar(&dmaIDs, "dma-ids", "", "Comma-separated DMA IDs")
	return cmd
}

func newEventsWatchlistLsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List saved watchlists",
		Example: "  ticketmaster-pp-cli events watchlist ls --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := ensureWatchlistsTable(cmd.Context(), db.DB()); err != nil {
				return err
			}
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT name, COALESCE(description,''), COALESCE(venue_ids,''), COALESCE(attraction_ids,''),
				        COALESCE(segments,''), COALESCE(genres,''), COALESCE(dma_ids,''),
				        COALESCE(updated_at,'')
				 FROM watchlists ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			var out []watchlist
			for rows.Next() {
				var w watchlist
				var v, a, s, g, d string
				if err := rows.Scan(&w.Name, &w.Description, &v, &a, &s, &g, &d, &w.UpdatedAt); err != nil {
					return err
				}
				w.VenueIDs = splitCSV(v)
				w.AttractionIDs = splitCSV(a)
				w.Segments = splitCSV(s)
				w.Genres = splitCSV(g)
				w.DMAIDs = splitCSV(d)
				out = append(out, w)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No watchlists saved. Use 'events watchlist save <name>' to create one.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tVENUES\tATTRACTIONS\tSEGMENTS\tDMAS\tUPDATED")
			for _, w := range out {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%s\n",
					w.Name, len(w.VenueIDs), len(w.AttractionIDs), len(w.Segments), len(w.DMAIDs), w.UpdatedAt)
			}
			return tw.Flush()
		},
	}
}

func newEventsWatchlistRmCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a named watchlist",
		Example: "  ticketmaster-pp-cli events watchlist rm seattle",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(args[0])
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureWatchlistsTable(cmd.Context(), db.DB()); err != nil {
				return err
			}
			res, err := db.DB().ExecContext(cmd.Context(), `DELETE FROM watchlists WHERE name = ?`, name)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				fmt.Fprintf(os.Stderr, "warning: no watchlist named %q\n", name)
				return notFoundErr(fmt.Errorf("watchlist %q", name))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed watchlist %q\n", name)
			return nil
		},
	}
}

func loadWatchlist(ctx context.Context, db *sql.DB, name string) (*watchlist, error) {
	if err := ensureWatchlistsTable(ctx, db); err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx,
		`SELECT name, COALESCE(description,''), COALESCE(venue_ids,''), COALESCE(attraction_ids,''),
		        COALESCE(segments,''), COALESCE(genres,''), COALESCE(dma_ids,''),
		        COALESCE(updated_at,'')
		 FROM watchlists WHERE name = ?`, name)
	var w watchlist
	var v, a, s, g, d string
	if err := row.Scan(&w.Name, &w.Description, &v, &a, &s, &g, &d, &w.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("watchlist %q not found", name)
		}
		return nil, err
	}
	w.VenueIDs = splitCSV(v)
	w.AttractionIDs = splitCSV(a)
	w.Segments = splitCSV(s)
	w.Genres = splitCSV(g)
	w.DMAIDs = splitCSV(d)
	return &w, nil
}

func newEventsWatchlistRunCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Apply a saved watchlist to currently-synced events",
		Long: strings.TrimSpace(`
Run a saved watchlist against the local events table. Returns events whose
venue, attraction, segment, genre, or DMA matches any of the watchlist's
filters, AND whose start date falls within --days from today.

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events watchlist run seattle --days 60 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(args[0])
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			wl, err := loadWatchlist(cmd.Context(), db.DB(), name)
			if err != nil {
				return err
			}
			events, err := queryFilteredEvents(cmd.Context(), db.DB(), wl, days)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			renderEventTable(cmd.OutOrStdout(), events)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 60, "Window in days from today")
	return cmd
}

// splitCSV returns trimmed non-empty comma-separated tokens.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// queryFilteredEvents pulls events from the local store filtered by the
// watchlist's filters AND the date window. Used by both 'watchlist run' and
// 'events upcoming' (when a watchlist name is provided).
func queryFilteredEvents(ctx context.Context, db *sql.DB, wl *watchlist, days int) ([]json.RawMessage, error) {
	conds := []string{}
	argsq := []any{}
	if len(wl.VenueIDs) > 0 {
		conds = append(conds, jsonArrayContains("$._embedded.venues", "id", wl.VenueIDs, &argsq))
	}
	if len(wl.AttractionIDs) > 0 {
		conds = append(conds, jsonArrayContains("$._embedded.attractions", "id", wl.AttractionIDs, &argsq))
	}
	if len(wl.Segments) > 0 {
		conds = append(conds, jsonClassificationMatches("segment", wl.Segments, &argsq))
	}
	if len(wl.Genres) > 0 {
		conds = append(conds, jsonClassificationMatches("genre", wl.Genres, &argsq))
	}
	if len(wl.DMAIDs) > 0 {
		// dmas are array of {id} under _embedded.venues[0].dmas
		conds = append(conds, jsonDMAMatches(wl.DMAIDs, &argsq))
	}
	if days > 0 {
		end := time.Now().AddDate(0, 0, days).UTC().Format("2006-01-02")
		conds = append(conds,
			`(json_extract(data, '$.dates.start.localDate') BETWEEN date('now') AND ?)`)
		argsq = append(argsq, end)
	}
	where := "1=1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}
	q := `SELECT data FROM events WHERE ` + where +
		` ORDER BY json_extract(data, '$.dates.start.dateTime') ASC NULLS LAST`
	rows, err := db.QueryContext(ctx, q, argsq...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(s))
	}
	return out, rows.Err()
}

// jsonArrayContains builds a SQL fragment that matches when any element of a
// JSON array (e.g. $._embedded.venues) has its `field` (e.g. "id") in the
// allowed list. SQLite's json_each is used to expand the array.
func jsonArrayContains(arrayPath, field string, allowed []string, args *[]any) string {
	if len(allowed) == 0 {
		return "1=1"
	}
	ph := strings.Repeat("?,", len(allowed))
	ph = ph[:len(ph)-1]
	for _, v := range allowed {
		*args = append(*args, v)
	}
	// EXISTS (SELECT 1 FROM json_each(json_extract(data, '$.path')) AS j WHERE json_extract(j.value, '$.field') IN (...))
	return fmt.Sprintf(
		`EXISTS (SELECT 1 FROM json_each(json_extract(data, '%s')) AS j WHERE json_extract(j.value, '$.%s') IN (%s))`,
		arrayPath, field, ph)
}

func jsonClassificationMatches(level string, allowed []string, args *[]any) string {
	if len(allowed) == 0 {
		return "1=1"
	}
	ph := strings.Repeat("?,", len(allowed))
	ph = ph[:len(ph)-1]
	for _, v := range allowed {
		*args = append(*args, v)
	}
	return fmt.Sprintf(
		`EXISTS (SELECT 1 FROM json_each(json_extract(data, '$.classifications')) AS c WHERE json_extract(c.value, '$.%s.name') IN (%s))`,
		level, ph)
}

func jsonDMAMatches(allowed []string, args *[]any) string {
	if len(allowed) == 0 {
		return "1=1"
	}
	ph := strings.Repeat("?,", len(allowed))
	ph = ph[:len(ph)-1]
	for _, v := range allowed {
		*args = append(*args, v)
	}
	// dmas live under _embedded.venues[*].dmas[*].id
	return fmt.Sprintf(
		`EXISTS (SELECT 1 FROM json_each(json_extract(data, '$._embedded.venues')) AS v,
		  json_each(json_extract(v.value, '$.dmas')) AS d
		  WHERE CAST(json_extract(d.value, '$.id') AS TEXT) IN (%s))`, ph)
}
