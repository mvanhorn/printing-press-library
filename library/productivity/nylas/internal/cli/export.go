// Copyright 2026 Nathan Kettles and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var resourceList string
	var grantID string
	var since string
	var dbPath string
	var format string
	var outFile string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Stream the local mirror to NDJSON for downstream analytics",
		Long: `Stream rows from the local SQLite mirror to NDJSON (one JSON object
per line). Each row carries resource type, ID, grant ID, sync timestamp,
and the upstream Nylas JSON payload as a sub-object, ready for DuckDB,
jq, pandas, or any line-oriented pipeline.`,
		Example: strings.Trim(`
  # Last 90 days of messages, NDJSON to stdout
  nylas-pp-cli export --resource messages --since 90d

  # Multiple resources, one grant, to a file
  nylas-pp-cli export --resource messages,events --grant grant_abc --output ./nylas.ndjson
`, "\n"),
		// output:ndjson tells dogfood/scorecard JSON-fidelity probes
		// that this command's contract is one JSON object per line
		// (NDJSON), not a single JSON value. The probe should validate
		// each line as JSON rather than the whole stream.
		Annotations: map[string]string{"mcp:read-only": "true", "output:ndjson": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if format != "ndjson" {
				return fmt.Errorf("only --format ndjson is supported in this release (got %q)", format)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("nylas-pp-cli")
			}
			autoRefreshIfStale(cmd.Context(), dbPath, cmd.ErrOrStderr())
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			defer db.Close()

			selected := []string{"messages", "threads", "events", "contacts"}
			if resourceList != "" {
				selected = nil
				for _, r := range strings.Split(resourceList, ",") {
					r = strings.TrimSpace(r)
					if r != "" {
						selected = append(selected, r)
					}
				}
			}

			var w *bufio.Writer
			if outFile == "" || outFile == "-" {
				w = bufio.NewWriter(cmd.OutOrStdout())
			} else {
				f, err := os.Create(outFile)
				if err != nil {
					return err
				}
				defer f.Close()
				w = bufio.NewWriter(f)
			}
			defer w.Flush()

			var cutoff string
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since %q: %w", since, err)
				}
				cutoff = ts.UTC().Format("2006-01-02 15:04:05")
			}

			total := 0
			for _, r := range selected {
				table, ok := sinceTables[r]
				if !ok {
					return fmt.Errorf("unknown resource %q", r)
				}
				q := fmt.Sprintf(`SELECT id, COALESCE(grants_id,'') AS grants_id, synced_at, data FROM %q`, table)
				params := []any{}
				where := []string{}
				if cutoff != "" {
					where = append(where, "synced_at >= ?")
					params = append(params, cutoff)
				}
				if grantID != "" {
					where = append(where, "grants_id = ?")
					params = append(params, grantID)
				}
				if len(where) > 0 {
					q += " WHERE " + strings.Join(where, " AND ")
				}
				q += " ORDER BY synced_at DESC"
				if limit > 0 {
					q += fmt.Sprintf(" LIMIT %d", limit)
				}
				rows, err := db.DB().QueryContext(cmd.Context(), q, params...)
				if err != nil {
					if strings.Contains(err.Error(), "no such column: grants_id") {
						continue
					}
					return fmt.Errorf("querying %s: %w", table, err)
				}
				enc := json.NewEncoder(w)
				for rows.Next() {
					var id, grants, syncedAt string
					var data json.RawMessage
					if err := rows.Scan(&id, &grants, &syncedAt, &data); err != nil {
						rows.Close()
						return err
					}
					rec := map[string]any{
						"resource":  r,
						"id":        id,
						"grants_id": grants,
						"synced_at": syncedAt,
						"data":      data,
					}
					if err := enc.Encode(rec); err != nil {
						rows.Close()
						return err
					}
					total++
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("iterating %s: %w", table, err)
				}
				rows.Close()
			}
			if !flags.asJSON && (outFile != "" && outFile != "-") {
				fmt.Fprintf(cmd.ErrOrStderr(), "exported %d rows to %s\n", total, outFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&resourceList, "resource", "", "Comma-separated resource types (default: messages,threads,events,contacts)")
	cmd.Flags().StringVar(&grantID, "grant", "", "Scope to one grant ID (default: all grants)")
	cmd.Flags().StringVar(&since, "since", "", "Only rows synced within this duration (e.g. 24h, 7d, 90d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite database")
	cmd.Flags().StringVar(&format, "format", "ndjson", "Output format: ndjson")
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "Write to file instead of stdout")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows per resource (0 = no cap)")
	return cmd
}
