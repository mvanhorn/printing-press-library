// Copyright 2026 james-frewin. Licensed under Apache-2.0. See LICENSE.

// Export streams locally-cached search-analytics rows for a site as
// either JSONL (one row per line) or CSV. Reads only from the store, so
// it works offline; pair with `sync` to populate.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var (
		format  string
		outPath string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export cached analytics rows for a site as JSONL or CSV",
		Long: strings.TrimSpace(`
Streams search-analytics rows (date × query × page × country × device,
with clicks/impressions/ctr/position) from the local store. Writes
to stdout by default; --output <path> writes to a file. The CSV
header matches the JSONL field order so the two outputs map cleanly.

No network calls — pair with sync to hydrate the store first.
`),
		Example:     "  google-search-console-pp-cli export --site sc-domain:example.com --window 28d --format jsonl",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			if format != "jsonl" && format != "csv" {
				return usageErr(fmt.Errorf("--format must be jsonl or csv, got %q", format))
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}

			days := parseWindow(cf.window, 28)
			start, end := dateRange(days)

			query := `
				SELECT date, query, page, country, device,
				       clicks, impressions, ctr, position
				FROM search_analytics_rows
				WHERE site_url = ? AND date BETWEEN ? AND ?
				ORDER BY date ASC, clicks DESC`
			qArgs := []any{cf.site, start, end}
			if limit > 0 {
				query += " LIMIT ?"
				qArgs = append(qArgs, limit)
			}
			rows, err := s.DB().QueryContext(cmd.Context(), query, qArgs...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := cmd.OutOrStdout()
			if outPath != "" {
				f, err := os.Create(outPath)
				if err != nil {
					return configErr(err)
				}
				defer f.Close()
				out = f
			}

			var csvW *csv.Writer
			if format == "csv" {
				csvW = csv.NewWriter(out)
				if err := csvW.Write([]string{"date", "query", "page", "country", "device",
					"clicks", "impressions", "ctr", "position"}); err != nil {
					return apiErr(err)
				}
			}

			for rows.Next() {
				var (
					date, query, page, country, device string
					clicks, impressions, ctr, position float64
				)
				if err := rows.Scan(&date, &query, &page, &country, &device,
					&clicks, &impressions, &ctr, &position); err != nil {
					return apiErr(err)
				}
				if format == "csv" {
					if err := csvW.Write([]string{date, query, page, country, device,
						strconv.FormatFloat(clicks, 'f', -1, 64),
						strconv.FormatFloat(impressions, 'f', -1, 64),
						strconv.FormatFloat(ctr, 'f', -1, 64),
						strconv.FormatFloat(position, 'f', -1, 64)}); err != nil {
						return apiErr(err)
					}
				} else {
					rec := map[string]any{
						"date": date, "query": query, "page": page,
						"country": country, "device": device,
						"clicks": clicks, "impressions": impressions,
						"ctr": ctr, "position": position,
					}
					enc := json.NewEncoder(out)
					if err := enc.Encode(rec); err != nil {
						return apiErr(err)
					}
				}
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}
			if csvW != nil {
				csvW.Flush()
				if err := csvW.Error(); err != nil {
					return apiErr(err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "28d", "Window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().StringVar(&format, "format", "jsonl", "Output format: jsonl or csv.")
	cmd.Flags().StringVar(&outPath, "output", "", "Output file path (default stdout).")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to export (0 = no limit).")
	return cmd
}
