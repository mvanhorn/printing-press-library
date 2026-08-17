// Hand-authored `watch` command: snapshot a saved location over time and diff
// rating/price drift. The only Hotelist feature that exploits local history —
// the website has no historical state. Implemented over a hand-authored
// snapshots table in the local SQLite mirror. Not generated.
package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/store"
)

func ensureWatchTables(ctx context.Context, db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watch_scopes (
			scope TEXT PRIMARY KEY,
			label TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS hotel_snapshots (
			scope TEXT NOT NULL,
			batch TEXT NOT NULL,
			hotel_id TEXT NOT NULL,
			name TEXT,
			rating REAL,
			price REAL,
			scraped_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (scope, batch, hotel_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_scope_batch ON hotel_snapshots(scope, batch)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().ExecContext(ctx, s); err != nil {
			return fmt.Errorf("creating watch tables: %w", err)
		}
	}
	return nil
}

func countSnapshots(db *store.Store) int {
	var n int
	_ = db.DB().QueryRow(`SELECT COUNT(DISTINCT scope || batch) FROM hotel_snapshots`).Scan(&n)
	return n
}

func countWatchScopes(db *store.Store) int {
	var n int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM watch_scopes`).Scan(&n)
	return n
}

// takeSnapshot writes a new snapshot batch for the scope.
func takeSnapshot(ctx context.Context, hotels []hlHotel, db *store.Store, scope, label string) (string, error) {
	batch := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO watch_scopes(scope, label) VALUES(?, ?)
		 ON CONFLICT(scope) DO UPDATE SET label=excluded.label`, scope, label); err != nil {
		return "", err
	}
	for _, h := range hotels {
		if h.HotelID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO hotel_snapshots(scope, batch, hotel_id, name, rating, price)
			 VALUES(?, ?, ?, ?, ?, ?)`,
			scope, batch, h.HotelID, h.Name, h.Rating, h.Price); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return batch, nil
}

type snapEntry struct {
	Name   string
	Rating float64
	Price  float64
}

// loadBaseline returns the snapshot batch to compare against. If since is set,
// it picks the earliest batch on/after that date; otherwise the most recent
// existing batch.
func loadBaseline(ctx context.Context, db *store.Store, scope, since string) (string, map[string]snapEntry, error) {
	var batch string
	var err error
	if since != "" {
		err = db.DB().QueryRowContext(ctx,
			`SELECT batch FROM hotel_snapshots WHERE scope=? AND date(scraped_at) >= date(?)
			 ORDER BY scraped_at ASC LIMIT 1`, scope, since).Scan(&batch)
	} else {
		err = db.DB().QueryRowContext(ctx,
			`SELECT batch FROM hotel_snapshots WHERE scope=?
			 ORDER BY scraped_at DESC LIMIT 1`, scope).Scan(&batch)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil // genuinely no baseline yet
	}
	if err != nil {
		// A real query error (locked DB, I/O, cancelled context) must not be
		// mistaken for "no baseline" — that would overwrite the user's history.
		return "", nil, fmt.Errorf("looking up baseline snapshot: %w", err)
	}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT hotel_id, name, rating, price FROM hotel_snapshots WHERE scope=? AND batch=?`, scope, batch)
	if err != nil {
		return batch, nil, err
	}
	defer rows.Close()
	out := map[string]snapEntry{}
	for rows.Next() {
		var id, name string
		var rating, price float64
		if err := rows.Scan(&id, &name, &rating, &price); err != nil {
			continue
		}
		out[id] = snapEntry{Name: name, Rating: rating, Price: price}
	}
	if err := rows.Err(); err != nil {
		return batch, nil, fmt.Errorf("reading baseline snapshot: %w", err)
	}
	return batch, out, nil
}

type moverRow struct {
	HotelID string  `json:"hotel_id"`
	Name    string  `json:"name"`
	From    float64 `json:"from"`
	To      float64 `json:"to"`
	Delta   float64 `json:"delta"`
}

type watchDiffView struct {
	Source       string     `json:"source"`
	Disclaimer   string     `json:"disclaimer"`
	Scope        string     `json:"scope"`
	BaselineTime string     `json:"baseline_time,omitempty"`
	CurrentTime  string     `json:"current_time"`
	Metric       string     `json:"metric"`
	RatingUp     []moverRow `json:"rating_improved,omitempty"`
	RatingDown   []moverRow `json:"rating_declined,omitempty"`
	PriceDown    []moverRow `json:"price_dropped,omitempty"`
	PriceUp      []moverRow `json:"price_increased,omitempty"`
	NewHotels    []string   `json:"new_hotels,omitempty"`
	Note         string     `json:"note,omitempty"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Snapshot a location over time and diff how its hotels' ratings and prices drift",
		Long: "Track how a place's hotels change over time — something Hotelist's website cannot do, " +
			"because it has no historical state. 'watch add' saves a location and takes a first snapshot; " +
			"'watch diff' re-fetches and reports which hotels improved, declined, or changed price since " +
			"the last snapshot. Data is scraped from hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli watch add lisbon
  hotelist-pp-cli watch diff lisbon --metric both
  hotelist-pp-cli watch list`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchDiffCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add <location>",
		Short:   "Save a location and take its first snapshot",
		Example: "  hotelist-pp-cli watch add lisbon\n  hotelist-pp-cli watch add thailand --json",
		// Writes a snapshot batch to the local SQLite store (takeSnapshot), so it is NOT mcp:read-only.
		Annotations: map[string]string{"pp:happy-args": "<location>=lisbon"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a location is required"))
			}
			c, err := flags.politeClient()
			if err != nil {
				return err
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := ensureWatchTables(cmd.Context(), db); err != nil {
				return err
			}
			loc, err := resolveLocation(cmd.Context(), c, db, args[0])
			if err != nil {
				return err
			}
			hotels, err := adaptiveFetch(cmd.Context(), c, loc.Filters, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			hotels = dedupeHotels(hotels)
			scope := makeSlug(args[0])
			batch, err := takeSnapshot(cmd.Context(), hotels, db, scope, loc.Label)
			if err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}
			out := cmd.OutOrStdout()
			view := map[string]any{
				"source": hotelistSource, "disclaimer": hotelistDisclaimer,
				"scope": scope, "label": loc.Label, "snapshot": batch, "hotels": len(hotels),
			}
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, view, flags)
			}
			fmt.Fprintf(out, "Watching %q (%s) — snapshot of %d hotels saved.\nRun 'hotelist-pp-cli watch diff %s' later to see what changed.\n",
				scope, loc.Label, len(hotels), scope)
			return nil
		},
	}
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List watched locations and their snapshot counts",
		Example:     "  hotelist-pp-cli watch list\n  hotelist-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := ensureWatchTables(cmd.Context(), db); err != nil {
				return err
			}
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT w.scope, w.label, COUNT(DISTINCT s.batch)
				 FROM watch_scopes w LEFT JOIN hotel_snapshots s ON s.scope=w.scope
				 GROUP BY w.scope, w.label ORDER BY w.scope`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type scopeRow struct {
				Scope     string `json:"scope"`
				Label     string `json:"label"`
				Snapshots int    `json:"snapshots"`
			}
			var list []scopeRow
			for rows.Next() {
				var sr scopeRow
				if err := rows.Scan(&sr.Scope, &sr.Label, &sr.Snapshots); err == nil {
					list = append(list, sr)
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading watch scopes: %w", err)
			}
			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, map[string]any{"source": hotelistSource, "watches": list}, flags)
			}
			if len(list) == 0 {
				fmt.Fprintln(out, "No watched locations yet. Add one with 'hotelist-pp-cli watch add <location>'.")
				return nil
			}
			for _, sr := range list {
				fmt.Fprintf(out, "%-20s %-30s %d snapshots\n", sr.Scope, sr.Label, sr.Snapshots)
			}
			return nil
		},
	}
}

func newWatchDiffCmd(flags *rootFlags) *cobra.Command {
	var since, metric string
	cmd := &cobra.Command{
		Use:     "diff <location>",
		Short:   "Re-fetch a watched location and report rating/price drift since the last snapshot",
		Example: "  hotelist-pp-cli watch diff lisbon\n  hotelist-pp-cli watch diff lisbon --metric price --json",
		// Records a fresh snapshot batch to the local SQLite store on every run
		// (takeSnapshot), so it is NOT mcp:read-only despite being read-oriented.
		Annotations: map[string]string{"pp:happy-args": "<location>=lisbon"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a location is required"))
			}
			if metric == "" {
				metric = "both"
			}
			if metric != "rating" && metric != "price" && metric != "both" {
				return usageErr(fmt.Errorf("--metric must be rating, price, or both"))
			}
			c, err := flags.politeClient()
			if err != nil {
				return err
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := ensureWatchTables(cmd.Context(), db); err != nil {
				return err
			}
			loc, err := resolveLocation(cmd.Context(), c, db, args[0])
			if err != nil {
				return err
			}
			scope := makeSlug(args[0])

			baselineBatch, baseline, err := loadBaseline(cmd.Context(), db, scope, since)
			if err != nil {
				return err
			}

			hotels, err := adaptiveFetch(cmd.Context(), c, loc.Filters, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			hotels = dedupeHotels(hotels)
			currentBatch, err := takeSnapshot(cmd.Context(), hotels, db, scope, loc.Label)
			if err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}

			view := watchDiffView{
				Source: hotelistSource, Disclaimer: hotelistDisclaimer,
				Scope: scope, BaselineTime: baselineBatch, CurrentTime: currentBatch, Metric: metric,
			}
			if len(baseline) == 0 {
				view.Note = "no prior snapshot to compare against; baseline established now. Run 'watch diff' again later."
				return printWatchDiff(cmd.OutOrStdout(), flags, view)
			}

			for _, h := range hotels {
				prev, ok := baseline[h.HotelID]
				if !ok {
					view.NewHotels = append(view.NewHotels, cleanTitle(h.Name))
					continue
				}
				if metric == "rating" || metric == "both" {
					if d := round2(h.Rating - prev.Rating); d != 0 {
						row := moverRow{HotelID: h.HotelID, Name: cleanTitle(h.Name), From: round1(prev.Rating), To: round1(h.Rating), Delta: d}
						if d > 0 {
							view.RatingUp = append(view.RatingUp, row)
						} else {
							view.RatingDown = append(view.RatingDown, row)
						}
					}
				}
				if metric == "price" || metric == "both" {
					if d := round2(h.Price - prev.Price); d != 0 {
						row := moverRow{HotelID: h.HotelID, Name: cleanTitle(h.Name), From: round2(prev.Price), To: round2(h.Price), Delta: d}
						if d < 0 {
							view.PriceDown = append(view.PriceDown, row)
						} else {
							view.PriceUp = append(view.PriceUp, row)
						}
					}
				}
			}
			sortMovers(view.RatingUp, true)
			sortMovers(view.RatingDown, false)
			sortMovers(view.PriceDown, false)
			sortMovers(view.PriceUp, true)
			return printWatchDiff(cmd.OutOrStdout(), flags, view)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Compare against the first snapshot on/after this date (YYYY-MM-DD); default is the latest snapshot")
	cmd.Flags().StringVar(&metric, "metric", "both", "What to diff: rating, price, or both")
	return cmd
}

func sortMovers(rows []moverRow, descending bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		if descending {
			return rows[i].Delta > rows[j].Delta
		}
		return rows[i].Delta < rows[j].Delta
	})
}

func printWatchDiff(out io.Writer, flags *rootFlags, view watchDiffView) error {
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, view, flags)
	}
	fmt.Fprintf(out, "Drift for %q (baseline %s → now)\n", view.Scope, shortTime(view.BaselineTime))
	fmt.Fprintln(out, strings.Repeat("-", 60))
	if view.Note != "" {
		fmt.Fprintf(out, "%s\n", view.Note)
	}
	printMoverSection(out, "Rating improved", view.RatingUp, "⭐")
	printMoverSection(out, "Rating declined", view.RatingDown, "⭐")
	printMoverSection(out, "Price dropped", view.PriceDown, "$")
	printMoverSection(out, "Price increased", view.PriceUp, "$")
	if len(view.NewHotels) > 0 {
		fmt.Fprintf(out, "New since baseline: %d\n", len(view.NewHotels))
	}
	fmt.Fprintf(out, "%s\n", view.Disclaimer)
	return nil
}

func printMoverSection(out io.Writer, title string, rows []moverRow, unit string) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s:\n", title)
	for i, r := range rows {
		if i >= 10 {
			fmt.Fprintf(out, "  ... and %d more\n", len(rows)-10)
			break
		}
		fmt.Fprintf(out, "  %-38s %s%.1f → %s%.1f (%+.1f)\n", truncate(r.Name, 38), unit, r.From, unit, r.To, r.Delta)
	}
}

func shortTime(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	if s == "" {
		return "none"
	}
	return s
}
