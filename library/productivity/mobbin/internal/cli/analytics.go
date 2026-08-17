// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source local
func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "analytics",
		Short:       "Summarize the local Mobbin store: per-table counts plus top patterns and apps.",
		Example:     "  mobbin-pp-cli analytics --agent",
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
				return flags.printJSON(cmd, map[string]any{
					"counts":       map[string]any{},
					"top_patterns": []any{},
					"top_apps":     []any{},
					"note":         "no local store yet; run `mobbin-pp-cli sync` first to populate it",
				})
			}
			defer db.Close()

			counts := map[string]any{}
			for _, table := range analyticsTables {
				rows, err := db.RawQuery(cmd.Context(), "SELECT COUNT(*) AS n FROM "+table)
				if err != nil {
					return err
				}
				if len(rows) == 1 {
					counts[table] = rows[0]["n"]
				}
			}

			topPatterns, err := db.RawQuery(cmd.Context(),
				`SELECT pattern_slug, COUNT(*) AS screens FROM screen_patterns
GROUP BY pattern_slug ORDER BY screens DESC LIMIT 5`)
			if err != nil {
				return err
			}
			topApps, err := db.RawQuery(cmd.Context(),
				`SELECT screens.app_id, apps.app_name, COUNT(*) AS screens
FROM screens LEFT JOIN apps ON apps.id=screens.app_id
GROUP BY screens.app_id, apps.app_name ORDER BY screens DESC LIMIT 5`)
			if err != nil {
				return err
			}

			return flags.printJSON(cmd, map[string]any{
				"counts":       counts,
				"top_patterns": topPatterns,
				"top_apps":     topApps,
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	return cmd
}

// analyticsTables is the domain-table set analytics counts rows over. Fixed
// allow-list: table names are interpolated into SQL, so they must never come
// from user input.
var analyticsTables = []string{
	"apps", "screens", "flows", "app_versions",
	"patterns", "elements", "flow_actions",
	"screen_patterns", "screen_elements", "collections",
}
