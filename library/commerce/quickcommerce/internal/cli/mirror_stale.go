// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newNovelMirrorStaleCmd(flags *rootFlags) *cobra.Command {
	var maxAge, dbPath string
	cmd := &cobra.Command{
		Use: "stale", Short: "Find saved product and ETA observations that are older than a chosen trust window.",
		Example:     "  quickcommerce-pp-cli mirror stale --max-age 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror stale")
			}
			age, err := parseQCDuration(maxAge)
			if err != nil {
				return usageErr(err)
			}
			path := qcDBPath(dbPath)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return qcMissingMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, "products")
			} else if err != nil {
				return apiErr(err)
			}
			db, err := openQCCreatable(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer db.Close()
			cutoff := time.Now().UTC().Add(-age)
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT resource,item_id,platform,location,captured_at,query FROM quickcommerce_observations WHERE captured_at < ? ORDER BY captured_at ASC`, cutoff)
			if err != nil {
				return apiErr(fmt.Errorf("querying stale observations: %w", err))
			}
			out := make([]map[string]any, 0)
			for rows.Next() {
				var resource, itemID, platform, location, query string
				var at any
				if err := rows.Scan(&resource, &itemID, &platform, &location, &at, &query); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				captured := qcDBTime(at)
				row := map[string]any{"resource": resource, "platform": platform, "location": location, "captured_at": captured.Format(time.RFC3339), "age_hours": int(time.Since(captured).Hours())}
				if itemID != "" {
					row["item_id"] = itemID
				}
				if query != "" {
					row["query"] = query
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			_ = rows.Close()
			view := map[string]any{"max_age": age.String(), "stale_count": len(out), "observations": out}
			return qcPrint(cmd.OutOrStdout(), flags, view, out)
		},
	}
	cmd.Flags().StringVar(&maxAge, "max-age", "24h", "Trust window before an observation is stale, such as 24h or 7d")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
