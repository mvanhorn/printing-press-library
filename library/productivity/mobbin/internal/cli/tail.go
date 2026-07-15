// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newTailCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "tail",
		Short:       "Show the most recently captured local rows across app_versions, screens, and flows.",
		Example:     "  mobbin-pp-cli tail --limit 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local store yet; run `mobbin-pp-cli sync` first")
				return flags.printJSON(cmd, []any{})
			}
			defer db.Close()

			if limit <= 0 {
				limit = 20
			}
			q := `SELECT 'app_version' AS kind, id, app_id, captured_at FROM app_versions
UNION ALL
SELECT 'screen' AS kind, id, app_id, captured_at FROM screens
UNION ALL
SELECT 'flow' AS kind, id, app_id, captured_at FROM flows
ORDER BY captured_at DESC LIMIT ` + fmt.Sprint(limit)
			rows, err := db.RawQuery(cmd.Context(), q)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum recent rows to return")
	return cmd
}
