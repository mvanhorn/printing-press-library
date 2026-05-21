// Copyright 2026 synota. Licensed under Apache-2.0. See LICENSE.
//
// EIA-specific series cache. The generated sync/search are generic
// pagination walkers; EIA's API is series-keyed, so we maintain a
// dedicated SQLite mirror of route metadata + recent data points, with
// an FTS5 index on series id/name/route. This is a separate sub-store
// from the generic resources store — the EIA series taxonomy doesn't
// fit the generic "resource type + JSON blob" model cleanly.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/eia/internal/client"

	_ "modernc.org/sqlite"

	"github.com/spf13/cobra"
)

// seriesRoutes lists the priority routes the EIA sync walks. Each route
// pulls metadata (frequencies, facets, columns) and the most recent
// `seriesRecentRows` data points by default.
var seriesRoutes = []seriesRoute{
	{ID: "electricity/retail-sales", Slug: "electricity-retail-sales", Title: "Electricity Retail Sales", Frequency: "monthly", DataColumns: []string{"price"}, FacetID: "stateid"},
	{ID: "electricity/rto/fuel-type-data", Slug: "electricity-rto-fuel-mix", Title: "BA Hourly Fuel Mix", Frequency: "hourly", DataColumns: []string{"value"}, FacetID: "respondent"},
	{ID: "electricity/rto/region-data", Slug: "electricity-rto-region", Title: "BA Hourly Demand & Generation", Frequency: "hourly", DataColumns: []string{"value"}, FacetID: "respondent"},
	{ID: "electricity/electric-power-operational-data", Slug: "electricity-generation", Title: "Monthly Net Generation by State", Frequency: "monthly", DataColumns: []string{"generation"}, FacetID: "location"},
	{ID: "natural-gas/pri/sum", Slug: "natural-gas-price-summary", Title: "Natural Gas Price Summary", Frequency: "monthly", DataColumns: []string{"value"}, FacetID: "duoarea"},
	{ID: "natural-gas/pri/fut", Slug: "natural-gas-futures", Title: "Natural Gas Futures and Henry Hub", Frequency: "daily", DataColumns: []string{"value"}, FacetID: "series"},
	{ID: "petroleum/pri/spt", Slug: "petroleum-spot-prices", Title: "Petroleum Spot Prices (WTI, Brent)", Frequency: "daily", DataColumns: []string{"value"}, FacetID: "series"},
	{ID: "steo", Slug: "steo", Title: "Short-Term Energy Outlook Forecasts", Frequency: "monthly", DataColumns: []string{"value"}, FacetID: "seriesId"},
	{ID: "seds", Slug: "seds-state-energy", Title: "State Energy Data System (SEDS)", Frequency: "annual", DataColumns: []string{"value"}, FacetID: "seriesId"},
}

const seriesRecentRows = 50

type seriesRoute struct {
	ID          string
	Slug        string
	Title       string
	Frequency   string
	DataColumns []string
	FacetID     string
}

// seriesStorePath returns the SQLite path for the EIA-specific series mirror.
// XDG: $XDG_DATA_HOME or ~/.local/share/eia-pp-cli/series.db.
func seriesStorePath() string {
	if v := os.Getenv("EIA_SERIES_DB"); v != "" {
		return v
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "eia-pp-cli", "series.db")
}

// openSeriesStore opens (and migrates) the EIA series mirror.
func openSeriesStore() (*sql.DB, error) {
	path := seriesStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS series_meta (
			id TEXT PRIMARY KEY,
			route TEXT NOT NULL,
			title TEXT,
			description TEXT,
			frequency TEXT,
			default_frequency TEXT,
			units TEXT,
			start_period TEXT,
			end_period TEXT,
			facets TEXT,
			columns TEXT,
			last_synced_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_series_meta_route ON series_meta(route)`,
		`CREATE TABLE IF NOT EXISTS series_data (
			series_id TEXT NOT NULL,
			period TEXT NOT NULL,
			value REAL,
			units TEXT,
			raw TEXT,
			PRIMARY KEY (series_id, period)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS series_fts USING fts5(
			id, title, description, route, tokenize='porter unicode61'
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			db.Close()
			return nil, fmt.Errorf("init schema: %w", err)
		}
	}
	return db, nil
}

// routeMetadata mirrors the discovery envelope returned by GET /v2/<route>/.
type routeMetadata struct {
	Response struct {
		ID               string            `json:"id"`
		Name             string            `json:"name"`
		Description      string            `json:"description"`
		Frequency        []json.RawMessage `json:"frequency"`
		Facets           []json.RawMessage `json:"facets"`
		Data             json.RawMessage   `json:"data"`
		StartPeriod      string            `json:"startPeriod"`
		EndPeriod        string            `json:"endPeriod"`
		DefaultFrequency string            `json:"defaultFrequency"`
		Routes           []json.RawMessage `json:"routes"`
	} `json:"response"`
}

func upsertSeriesMeta(db *sql.DB, route seriesRoute, meta *routeMetadata) error {
	facets, _ := json.Marshal(meta.Response.Facets)
	columns, _ := json.Marshal(meta.Response.Data)
	desc := strings.TrimSpace(meta.Response.Description)
	title := meta.Response.Name
	if title == "" {
		title = route.Title
	}
	_, err := db.Exec(`
		INSERT INTO series_meta (id, route, title, description, frequency, default_frequency, units, start_period, end_period, facets, columns, last_synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			route=excluded.route,
			title=excluded.title,
			description=excluded.description,
			frequency=excluded.frequency,
			default_frequency=excluded.default_frequency,
			units=excluded.units,
			start_period=excluded.start_period,
			end_period=excluded.end_period,
			facets=excluded.facets,
			columns=excluded.columns,
			last_synced_at=excluded.last_synced_at
	`,
		route.Slug,
		route.ID,
		title,
		desc,
		route.Frequency,
		meta.Response.DefaultFrequency,
		"",
		meta.Response.StartPeriod,
		meta.Response.EndPeriod,
		string(facets),
		string(columns),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	// FTS index: rebuild this row's entry by deleting first.
	if _, err := db.Exec(`DELETE FROM series_fts WHERE id = ?`, route.Slug); err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO series_fts (id, title, description, route)
		VALUES (?, ?, ?, ?)
	`, route.Slug, title, desc, route.ID)
	return err
}

func upsertSeriesData(db *sql.DB, seriesID string, rows []eiaRow, valueColumn string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO series_data (series_id, period, value, units, raw)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(series_id, period) DO UPDATE SET
			value=excluded.value,
			units=excluded.units,
			raw=excluded.raw
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	n := 0
	for _, row := range rows {
		period := rowString(row, "period")
		if period == "" {
			continue
		}
		v, _ := rowFloat(row, valueColumn)
		units := rowString(row, "units")
		if units == "" {
			units = rowString(row, valueColumn+"-units")
		}
		raw, _ := json.Marshal(row)
		if _, err := stmt.Exec(seriesID, period, v, units, string(raw)); err != nil {
			return n, err
		}
		n++
	}
	return n, tx.Commit()
}

// ----------------------------------------------------------------------
// series sync
// ----------------------------------------------------------------------

func newSeriesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "series",
		Short: "Local SQLite mirror of EIA series metadata (sync + FTS search)",
		Long: `The EIA API is series-keyed. This group maintains a local SQLite mirror
of route metadata and recent data points, plus an FTS5 full-text index
over series id / title / route. Use 'series sync' to populate and
'series search <query>' to find a series offline.`,
	}
	cmd.AddCommand(newSeriesSyncCmd(flags))
	cmd.AddCommand(newSeriesSearchCmd(flags))
	cmd.AddCommand(newSeriesListCmd(flags))
	return cmd
}

func newSeriesSyncCmd(flags *rootFlags) *cobra.Command {
	var metaOnly bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync priority EIA series metadata and recent data into local SQLite",
		Long: `Walks the priority EIA routes (electricity, natural-gas, petroleum,
steo, seds), fetches discovery metadata, and pulls the last ` + strconv.Itoa(seriesRecentRows) + ` data
points per route into the local mirror. Pass --meta-only to skip the
data-points pull.`,
		Example: "  eia-pp-cli series sync",
		Annotations: map[string]string{
			"pp:novel":       "series.sync",
			"pp:client-call": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openSeriesStore()
			if err != nil {
				return err
			}
			defer db.Close()
			type result struct {
				Route   string `json:"route"`
				Slug    string `json:"slug"`
				MetaOK  bool   `json:"meta_ok"`
				Rows    int    `json:"rows_upserted"`
				Error   string `json:"error,omitempty"`
			}
			results := make([]result, 0, len(seriesRoutes))
			for _, r := range seriesRoutes {
				res := result{Route: r.ID, Slug: r.Slug}
				// Metadata fetch
				rawMeta, err := c.Get("/"+r.ID+"/", nil)
				if err != nil {
					res.Error = err.Error()
					results = append(results, res)
					continue
				}
				var meta routeMetadata
				if err := json.Unmarshal(rawMeta, &meta); err != nil {
					res.Error = err.Error()
					results = append(results, res)
					continue
				}
				if err := upsertSeriesMeta(db, r, &meta); err != nil {
					res.Error = err.Error()
					results = append(results, res)
					continue
				}
				res.MetaOK = true
				if !metaOnly {
					params := map[string]string{
						"frequency":          r.Frequency,
						"sort[0][column]":    "period",
						"sort[0][direction]": "desc",
						"length":             strconv.Itoa(seriesRecentRows),
					}
					for _, col := range r.DataColumns {
						params["data[]"] = col
					}
					_, rows, derr := callEiaData(c, "/"+r.ID+"/data/", params)
					if derr != nil {
						res.Error = derr.Error()
						results = append(results, res)
						continue
					}
					valueCol := "value"
					if len(r.DataColumns) > 0 {
						valueCol = r.DataColumns[0]
					}
					n, uerr := upsertSeriesData(db, r.Slug, rows, valueCol)
					if uerr != nil {
						res.Error = uerr.Error()
						results = append(results, res)
						continue
					}
					res.Rows = n
				}
				results = append(results, res)
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(results, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d routes to %s\n", len(results), seriesStorePath())
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s — %s\n", r.Route, r.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s — %d rows\n", r.Route, r.Rows)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&metaOnly, "meta-only", false, "skip data points, sync metadata only")
	return cmd
}

func newSeriesSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "FTS5 search across synced series metadata",
		Long: `Full-text search over the local series mirror. Indexes id, title,
description, and route. Run 'series sync' first to populate.`,
		Example: "  eia-pp-cli series search 'henry hub'",
		Args:    cobra.MinimumNArgs(1),
		Annotations: map[string]string{
			"pp:novel": "series.search",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.Join(args, " ")
			db, err := openSeriesStore()
			if err != nil {
				return err
			}
			defer db.Close()
			if limit <= 0 {
				limit = 25
			}
			rows, err := db.Query(`
				SELECT f.id, f.title, f.route,
				       snippet(series_fts, 2, '[', ']', '…', 12) AS hit
				FROM series_fts AS f
				WHERE series_fts MATCH ?
				LIMIT ?
			`, q, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type hit struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Route string `json:"route"`
				Hit   string `json:"hit"`
			}
			var hits []hit
			for rows.Next() {
				var h hit
				if err := rows.Scan(&h.ID, &h.Title, &h.Route, &h.Hit); err != nil {
					return err
				}
				hits = append(hits, h)
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(hits, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no matches — run 'series sync' first?)")
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %s\n  route: /v2/%s/\n  %s\n\n", h.ID, h.Title, h.Route, h.Hit)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "max results")
	return cmd
}

func newSeriesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all synced series metadata rows",
		Annotations: map[string]string{
			"pp:novel": "series.list",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openSeriesStore()
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(`SELECT id, route, title, frequency, default_frequency, start_period, end_period, last_synced_at FROM series_meta ORDER BY id`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				ID               string `json:"id"`
				Route            string `json:"route"`
				Title            string `json:"title"`
				Frequency        string `json:"frequency"`
				DefaultFrequency string `json:"default_frequency"`
				StartPeriod      string `json:"start_period"`
				EndPeriod        string `json:"end_period"`
				LastSyncedAt     string `json:"last_synced_at"`
			}
			var out []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.ID, &r.Route, &r.Title, &r.Frequency, &r.DefaultFrequency, &r.StartPeriod, &r.EndPeriod, &r.LastSyncedAt); err != nil {
					return err
				}
				out = append(out, r)
			}
			if flags.asJSON {
				b, _ := json.MarshalIndent(out, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %s\n  route /v2/%s/  freq=%s (%s..%s)\n", r.ID, r.Title, r.Route, r.Frequency, r.StartPeriod, r.EndPeriod)
			}
			return nil
		},
	}
	return cmd
}

// Unused import shim to keep client referenced when callEiaData lives in
// novel_eia.go. (Go requires every import to be used in each file.)
var _ = client.New
