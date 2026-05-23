// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored local SQLite backup archive.

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newBackupCmd(flags *rootFlags) *cobra.Command {
	output := ""
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a portable local-store backup",
		Long:  "Dump local SQLite mirror tables as JSON Lines and CSV into a zip archive. Invoice PDFs are not included in this version.",
		Example: `  partitaiva24-pp-cli backup
  partitaiva24-pp-cli backup -o ~/partitaiva24-backup-20260509.zip`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if output == "" {
				output = fmt.Sprintf("~/partitaiva24-backup-%s.zip", time.Now().Format("20060102-150405"))
			}
			path := homeExpanded(output)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			defer f.Close()
			zw := zip.NewWriter(f)
			tables := []string{"invoices", "income", "customers", "fiscal_year", "attachments", "notifications", "subscriptions", "tickets"}
			manifest := map[string]int{}
			totalRows := 0
			for _, table := range tables {
				n, err := dumpTable(cmd, s.DB(), zw, table)
				if err != nil {
					_ = zw.Close()
					return err
				}
				manifest[table] = n
				totalRows += n
			}
			mw, err := zw.Create("manifest.json")
			if err != nil {
				_ = zw.Close()
				return err
			}
			if err := json.NewEncoder(mw).Encode(map[string]any{"created_at": time.Now().Format(time.RFC3339), "tables": manifest}); err != nil {
				_ = zw.Close()
				return err
			}
			if err := zw.Close(); err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"path":     path,
					"tables":   len(tables),
					"rows":     totalRows,
					"by_table": manifest,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Backed up %d rows across %d tables to %s\n", totalRows, len(tables), path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Zip output path")
	return cmd
}

func dumpTable(cmd *cobra.Command, db *sql.DB, zw *zip.Writer, table string) (int, error) {
	rows, err := db.QueryContext(cmd.Context(), fmt.Sprintf(`SELECT * FROM %s`, table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	// archive/zip writers are sequential — each Create() finalizes the previous
	// entry. Materialize the rows once, then write JSONL and CSV back-to-back.
	type row struct {
		obj    map[string]any
		record []string
	}
	var collected []row
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return 0, err
		}
		r := row{obj: map[string]any{}, record: make([]string, len(cols))}
		for i, col := range cols {
			switch v := vals[i].(type) {
			case nil:
				r.obj[col] = nil
				r.record[i] = ""
			case []byte:
				r.obj[col] = string(v)
				r.record[i] = string(v)
			default:
				r.obj[col] = v
				r.record[i] = fmt.Sprint(v)
			}
		}
		collected = append(collected, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	jw, err := zw.Create(table + ".jsonl")
	if err != nil {
		return 0, err
	}
	for _, r := range collected {
		line, err := json.Marshal(r.obj)
		if err != nil {
			return 0, err
		}
		if _, err := jw.Write(append(line, '\n')); err != nil {
			return 0, err
		}
	}

	cwFile, err := zw.Create(table + ".csv")
	if err != nil {
		return 0, err
	}
	cw := csv.NewWriter(cwFile)
	if err := cw.Write(cols); err != nil {
		return 0, err
	}
	for _, r := range collected {
		if err := cw.Write(r.record); err != nil {
			return 0, err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return 0, err
	}
	return len(collected), nil
}
