// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export cached notebooks to a JSON file for backup or sharing",
		Example: `  notebooklm-pp-cli sync --json
  notebooklm-pp-cli export --out notebooks-backup.json --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSON(map[string]any{"exported": 0, "path": outPath, "dry_run": true})
			}
			if outPath == "" {
				return usageErr(fmt.Errorf("--out is required"))
			}
			st, err := store.Open(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			rows, err := st.ReadOnlyQuery(cmd.Context(), `SELECT payload FROM notebooks ORDER BY title`, 10000)
			if err != nil {
				return apiErr(err)
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil && filepath.Dir(outPath) != "." {
				return configErr(err)
			}
			f, err := os.Create(outPath) // #nosec G304 -- user-specified export path
			if err != nil {
				return configErr(err)
			}
			defer f.Close()
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(rows); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return printJSON(map[string]any{"exported": len(rows), "path": outPath})
			}
			fmt.Printf("exported %d notebooks to %s\n", len(rows), outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output JSON file path")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite cache path")
	return cmd
}
