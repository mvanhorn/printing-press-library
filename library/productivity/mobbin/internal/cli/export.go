// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source local
//
// export writes a user-chosen file (via --out), so it is intentionally NOT
// annotated mcp:read-only: a false read-only hint on a command that writes
// outside the cache is a real bug.
func newExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, out, format, table string
	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export local Mobbin store tables to a file or stdout (json or csv).",
		Example: "  mobbin-pp-cli export --table screens --format csv --out screens.csv",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if format != "json" && format != "csv" {
				return usageErr(fmt.Errorf("--format must be json or csv, got %q", format))
			}
			tables := analyticsTables
			if table != "" {
				if !isDomainTable(table) {
					return usageErr(fmt.Errorf("--table %q is not a known domain table", table))
				}
				tables = []string{table}
			}
			if format == "csv" && table == "" {
				return usageErr(fmt.Errorf("--format csv requires a single --table"))
			}

			// export writes a file; under verify short-circuit before touching
			// the store or the filesystem.
			if cliutil.IsVerifyEnv() {
				dest := "stdout"
				if out != "" {
					dest = out
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "would export tables=%v format=%s to %s\n", tables, format, dest)
				return nil
			}

			db, err := openStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local store yet; run `mobbin-pp-cli sync` first")
				return flags.printJSON(cmd, map[string]any{})
			}
			defer db.Close()

			if format == "csv" {
				rows, err := db.RawQuery(cmd.Context(), "SELECT * FROM "+tables[0])
				if err != nil {
					return err
				}
				payload, err := rowsToCSV(rows)
				if err != nil {
					return err
				}
				return writeExport(cmd, flags, out, payload, len(rows))
			}

			// JSON: single table -> array; bundle -> {table: rows}.
			var value any
			total := 0
			if len(tables) == 1 {
				rows, err := db.RawQuery(cmd.Context(), "SELECT * FROM "+tables[0])
				if err != nil {
					return err
				}
				value = rows
				total = len(rows)
			} else {
				bundle := map[string]any{}
				for _, t := range tables {
					rows, err := db.RawQuery(cmd.Context(), "SELECT * FROM "+t)
					if err != nil {
						return err
					}
					bundle[t] = rows
					total += len(rows)
				}
				value = bundle
			}
			if out == "" {
				return flags.printJSON(cmd, value)
			}
			payload, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			return writeExport(cmd, flags, out, payload, total)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	cmd.Flags().StringVar(&out, "out", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	cmd.Flags().StringVar(&table, "table", "", "Single domain table to export (default: bundle of all domain tables)")
	return cmd
}

func isDomainTable(name string) bool {
	for _, t := range analyticsTables {
		if t == name {
			return true
		}
	}
	return false
}

// rowsToCSV renders rows to CSV with a deterministic header (sorted union of
// all keys) so column order is stable across runs.
func rowsToCSV(rows []map[string]any) ([]byte, error) {
	colSet := map[string]bool{}
	for _, r := range rows {
		for k := range r {
			colSet[k] = true
		}
	}
	cols := make([]string, 0, len(colSet))
	for k := range colSet {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(cols); err != nil {
		return nil, err
	}
	for _, r := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			if v, ok := r[c]; ok && v != nil {
				rec[i] = fmt.Sprint(v)
			}
		}
		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func writeExport(cmd *cobra.Command, flags *rootFlags, out string, payload []byte, count int) error {
	if out == "" {
		_, err := cmd.OutOrStdout().Write(payload)
		return err
	}
	if dir := filepath.Dir(out); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(out, payload, 0o644); err != nil {
		return err
	}
	return flags.printJSON(cmd, map[string]any{"exported": count, "path": out})
}
