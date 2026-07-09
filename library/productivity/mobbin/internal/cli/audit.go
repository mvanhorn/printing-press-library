// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newAuditCmd(flags *rootFlags) *cobra.Command {
	var platform, industry, since, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "audit <flow-type>",
		Short:       "Audit local flows by action, industry, platform, and time window.",
		Example:     "  mobbin-pp-cli audit onboarding --platform web --industry b2b-saas --since 60d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseSince(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			db, err := openStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "no flows in local store; run `mobbin-pp-cli sync` first to populate it")
				return flags.printJSON(cmd, []map[string]any{})
			}
			defer db.Close()
			cutoff := time.Now().Add(-d).UTC().Format(time.RFC3339)
			where := []string{"flows.flow_actions LIKE " + sqlQuote("%"+args[0]+"%"), "flows.captured_at >= " + sqlQuote(cutoff)}
			if platform != "" {
				where = append(where, "flows.platform="+sqlQuote(platform))
			}
			if industry != "" {
				where = append(where, "apps.app_categories LIKE "+sqlQuote("%"+industry+"%"))
			}
			q := `SELECT apps.app_name AS app, flows.name AS flow, flows.step_count, flows.captured_at
FROM flows LEFT JOIN apps ON apps.id=flows.app_id
WHERE ` + strings.Join(where, " AND ") + ` ORDER BY flows.captured_at DESC LIMIT ` + fmt.Sprint(limit)
			rows, err := db.RawQuery(cmd.Context(), q)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "web", "Platform to filter")
	cmd.Flags().StringVar(&industry, "industry", "", "App category, e.g. b2b-saas")
	cmd.Flags().StringVar(&since, "since", "60d", "Window duration, e.g. 60d or 1440h")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum flows to return")
	return cmd
}
