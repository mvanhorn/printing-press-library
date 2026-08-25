// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newBenchCmd(flags *rootFlags) *cobra.Command {
	var pattern, industry, platform, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "bench",
		Short:       "Rank apps by local screen count for a Mobbin pattern.",
		Example:     "  mobbin-pp-cli bench --pattern paywall --industry fintech --platform web --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			if db == nil {
				return flags.printJSON(cmd, map[string]any{
					"rows":  []any{},
					"count": 0,
					"note":  "no screens in local store; run `mobbin-pp-cli sync` first to populate it",
				})
			}
			defer db.Close()
			where := []string{"sp.pattern_slug=" + sqlQuote(pattern)}
			if platform != "" {
				where = append(where, "screens.platform="+sqlQuote(platform))
			}
			if industry != "" {
				where = append(where, "apps.app_categories LIKE "+sqlQuote("%"+industry+"%"))
			}
			q := `SELECT screens.app_id, apps.app_name, COUNT(*) AS n, MAX(screens.captured_at) AS last_seen
FROM screens JOIN screen_patterns sp ON sp.screen_id=screens.id
LEFT JOIN apps ON apps.id=screens.app_id
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY screens.app_id, apps.app_name ORDER BY n DESC LIMIT ` + fmt.Sprint(limit)
			rows, err := db.RawQuery(cmd.Context(), q)
			if err != nil {
				return err
			}
			// Empty-store guidance: distinguish "local cache is empty"
			// from "this query matched zero rows" so first-time users
			// don't assume bench is broken before they've run sync.
			if len(rows) == 0 {
				countRows, _ := db.RawQuery(cmd.Context(), `SELECT COUNT(*) AS n FROM screens`)
				if len(countRows) == 1 && fmt.Sprint(countRows[0]["n"]) == "0" {
					return flags.printJSON(cmd, map[string]any{
						"rows":  []any{},
						"count": 0,
						"note":  "no screens in local store; run `mobbin-pp-cli sync` first to populate it",
					})
				}
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&pattern, "pattern", "", "Pattern slug, e.g. paywall")
	cmd.Flags().StringVar(&industry, "industry", "", "App category, e.g. fintech")
	cmd.Flags().StringVar(&platform, "platform", "web", "Platform to filter")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum apps to return")
	return cmd
}
