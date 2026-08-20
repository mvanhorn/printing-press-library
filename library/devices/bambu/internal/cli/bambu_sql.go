package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/store"
	"github.com/spf13/cobra"
)

func newBambuSQLCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{Use: "sql", Short: "Run a read-only SQL query against the local Bambu database", Example: "  bambu-pp-cli sql --query 'SELECT observation_type, count(*) AS count FROM observations GROUP BY observation_type' --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
		}
		if query == "" {
			return usageErr(fmt.Errorf("--query is required"))
		}
		if limit <= 0 || limit > 5000 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 5000"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, err := store.OpenReadOnlyContext(ctx, defaultDBPath("bambu-pp-cli"))
		if err != nil {
			return configErr(err)
		}
		defer s.Close()
		_, results, truncated, err := store.BoundedReadOnlyQuery(ctx, s.DB(), query, limit, 60000, flags.timeout)
		if err != nil {
			return usageErr(fmt.Errorf("query local database: %w", err))
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"rows": results, "count": len(results), "truncated": truncated, "row_limit": limit}, flags)
	}}
	cmd.Flags().StringVar(&query, "query", "", "Read-only SQL statement to execute")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum result rows to return")
	return cmd
}
