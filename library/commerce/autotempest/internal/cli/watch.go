// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/cliutil"

	"github.com/spf13/cobra"
)

// newNovelWatchCmd is the `watch` parent. Subcommands register, list, remove,
// and replay saved searches. Only `watch run` hits the network; the rest read
// or write the local store only. All are read-only in the MCP sense (no
// external mutation — the API is read-only and persistence is local).
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Register named searches with their filters, then replay them through run so drops have snapshots to compare.",
		Example: strings.Trim(`
  autotempest-pp-cli watch add mysearch "honda civic" --zip 33701
  autotempest-pp-cli watch run mysearch --json
  autotempest-pp-cli watch list --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchLsCmd(flags))
	cmd.AddCommand(newWatchRmCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	return cmd
}

// newWatchAddCmd persists a saved search (name + query + JSON params) without
// running it.
func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var (
		mk, model, zip, dbPath                 string
		radius                                 int
		minPrice, maxPrice, minYear, maxYear   int
		minMiles, maxMiles                     int
		body, drive, fuel, transmission, color string
		title, seller, sortBy, sitesCSV        string
		rpp, maxPages, limit                   int
	)
	cmd := &cobra.Command{
		Use:   "add <name> [query] [flags]",
		Short: "Register a saved search by name (does not run it)",
		Example: strings.Trim(`
  autotempest-pp-cli watch add mysearch "honda civic" --zip 33701 --max-price 25000`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			opts := findOpts{
				Make: mk, Model: model, Zip: zip, Radius: radius,
				MinPrice: minPrice, MaxPrice: maxPrice,
				MinYear: minYear, MaxYear: maxYear,
				MinMiles: minMiles, MaxMiles: maxMiles,
				Body: body, Drive: drive, Fuel: fuel, Transmission: transmission,
				Color: color, Title: title, Seller: seller, Sort: sortBy,
				Sites: splitCSV(sitesCSV), RPP: rpp, MaxPages: maxPages, Limit: limit,
				SaveName: name,
			}
			applyPositional(&opts, args[1:])

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openAutoTempestStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			// markRun=false: add registers but does not execute.
			if err := saveSearchRow(ctx, db.DB(), name, opts, false); err != nil {
				return err
			}
			return emitNovel(cmd, flags, []map[string]any{{
				"name":  name,
				"query": strings.TrimSpace(opts.Make + " " + opts.Model),
				"saved": true,
			}})
		},
	}
	// Register flags via cmd.Flags() directly (not an `f :=` alias) so the
	// library's verify_skill.py static tracer can see each declaration.
	cmd.Flags().StringVar(&mk, "make", "", "Make slug")
	cmd.Flags().StringVar(&model, "model", "", "Model slug")
	cmd.Flags().StringVar(&zip, "zip", "", "ZIP code")
	cmd.Flags().IntVar(&radius, "radius", -1, "Search radius in miles (-1 = national)")
	cmd.Flags().IntVar(&minPrice, "min-price", -1, "Minimum price")
	cmd.Flags().IntVar(&maxPrice, "max-price", -1, "Maximum price")
	cmd.Flags().IntVar(&minYear, "min-year", -1, "Minimum year")
	cmd.Flags().IntVar(&maxYear, "max-year", -1, "Maximum year")
	cmd.Flags().IntVar(&minMiles, "min-miles", -1, "Minimum mileage")
	cmd.Flags().IntVar(&maxMiles, "max-miles", -1, "Maximum mileage")
	cmd.Flags().StringVar(&body, "body", "", "Body style filter")
	cmd.Flags().StringVar(&drive, "drive", "", "Drivetrain filter")
	cmd.Flags().StringVar(&fuel, "fuel", "", "Fuel type filter")
	cmd.Flags().StringVar(&transmission, "transmission", "", "Transmission filter")
	cmd.Flags().StringVar(&color, "color", "", "Exterior color filter")
	cmd.Flags().StringVar(&title, "title", "", "Title status filter")
	cmd.Flags().StringVar(&seller, "seller", "", "Seller type filter")
	cmd.Flags().StringVar(&sortBy, "sort", "best_match", "Sort order")
	cmd.Flags().StringVar(&sitesCSV, "sites", strings.Join(defaultSites(), ","), "Comma-separated source codes")
	cmd.Flags().IntVar(&rpp, "rpp", 50, "Results per page")
	cmd.Flags().IntVar(&maxPages, "max-pages", 1, "Max pages per source")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max total listings")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

// newWatchLsCmd lists saved searches.
func newWatchLsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved searches with their query, filters, and last-run time",
		Example: strings.Trim(`
  autotempest-pp-cli watch list --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openAutoTempestStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := listSavedSearches(ctx, db.DB())
			if err != nil {
				return err
			}
			return emitNovel(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

// newWatchRmCmd removes a saved search.
func newWatchRmCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a saved search",
		Example: strings.Trim(`
  autotempest-pp-cli watch rm mysearch`, "\n"),
		// The error-path probe pre-creates the name via `watch add` before
		// running `rm <name>`, so the row deterministically exists and rm
		// succeeds (exit 0). rm still errors on a genuinely-missing name in
		// normal use (see the n==0 branch below); the probe just can't observe
		// it given that ordering, so opt out.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openAutoTempestStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.DB().ExecContext(ctx, `DELETE FROM at_saved_searches WHERE name = ?`, name)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			// Removing a name that doesn't exist is a usage error, matching
			// `watch run`'s not-found behavior (and the profile commands).
			if n == 0 {
				return notFoundErr(fmt.Errorf("saved search %q not found", name))
			}
			return emitNovel(cmd, flags, []map[string]any{{"name": name, "removed": true}})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

// newWatchRunCmd replays one or all saved searches live, persisting listings +
// snapshots. This is the only watch subcommand that hits the network.
func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Replay saved searches live, persisting listings and price snapshots",
		Example: strings.Trim(`
  autotempest-pp-cli watch run mysearch --json
  autotempest-pp-cli watch run --json`, "\n"),
		// run errors on a genuinely-unknown name (see the not-found branch), but
		// the error-path probe pre-creates the name via `watch add` first, so the
		// search exists and run succeeds (exit 0). Opt out of the probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if len(args) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "would replay saved search %q\n", args[0])
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "would replay all saved searches")
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := openAutoTempestStore(ctx, dbPath)
			if err != nil {
				return err
			}
			searches, err := loadSavedSearchOpts(ctx, db.DB(), argOrEmpty(args))
			_ = db.Close()
			if err != nil {
				return err
			}
			if len(searches) == 0 {
				if len(args) > 0 {
					return usageErr(fmt.Errorf("saved search %q not found", args[0]))
				}
				return usageErr(fmt.Errorf("no saved searches; add one with: autotempest-pp-cli watch add <name> <query>"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results := make([]findEnvelope, 0, len(searches))
			for _, opts := range searches {
				if cliutil.IsDogfoodEnv() {
					opts.MaxPages = 1
				}
				env, err := runFind(ctx, c, flags, opts, dbPath)
				if err != nil {
					return err
				}
				// Mark last_run.
				rdb, rerr := openAutoTempestStore(ctx, dbPath)
				if rerr == nil {
					_, _ = rdb.DB().ExecContext(ctx, `UPDATE at_saved_searches SET last_run = ? WHERE name = ?`,
						time.Now().Unix(), opts.SaveName)
					_ = rdb.Close()
				}
				results = append(results, env)
			}

			// Single search: emit its envelope directly so output matches find.
			if len(results) == 1 {
				return emitFindEnvelope(cmd, flags, results[0])
			}
			// Multiple: emit a combined array of envelopes.
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

func argOrEmpty(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// listSavedSearches returns display rows for `watch list`.
func listSavedSearches(ctx context.Context, sqlDB *sql.DB) ([]map[string]any, error) {
	dbRows, err := sqlDB.QueryContext(ctx, `SELECT name, query, created, last_run FROM at_saved_searches ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()
	rows := make([]map[string]any, 0)
	for dbRows.Next() {
		var name, query sql.NullString
		var created, lastRun sql.NullInt64
		if err := dbRows.Scan(&name, &query, &created, &lastRun); err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{
			"name":     name.String,
			"query":    query.String,
			"created":  tsDisplay(created),
			"last_run": tsDisplay(lastRun),
		})
	}
	return rows, dbRows.Err()
}

func tsDisplay(v sql.NullInt64) string {
	if !v.Valid || v.Int64 == 0 {
		return ""
	}
	return time.Unix(v.Int64, 0).UTC().Format(time.RFC3339)
}

// loadSavedSearchOpts reads saved searches and reconstructs findOpts. When name
// is non-empty, only that search is returned.
func loadSavedSearchOpts(ctx context.Context, sqlDB *sql.DB, name string) ([]findOpts, error) {
	query := `SELECT name, params FROM at_saved_searches`
	var argv []any
	if name != "" {
		query += ` WHERE name = ?`
		argv = append(argv, name)
	}
	query += ` ORDER BY name`

	dbRows, err := sqlDB.QueryContext(ctx, query, argv...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()
	var out []findOpts
	for dbRows.Next() {
		var rowName, paramsJSON sql.NullString
		if err := dbRows.Scan(&rowName, &paramsJSON); err != nil {
			return nil, err
		}
		var p savedSearchParams
		if paramsJSON.Valid && paramsJSON.String != "" {
			if err := json.Unmarshal([]byte(paramsJSON.String), &p); err != nil {
				return nil, fmt.Errorf("decoding saved search %q params: %w", rowName.String, err)
			}
		}
		out = append(out, paramsToOpts(p, rowName.String))
	}
	return out, dbRows.Err()
}
