// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

var sqlLimitRegex = regexp.MustCompile(`(?i)\blimit\s+\d+`)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "sql [query]",
		Short:       "Run a SELECT against the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  fedex-pp-cli sql "SELECT service_type, count(*) FROM shipments GROUP BY service_type"
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if !isSelectQuery(query) {
				return usageErr(fmt.Errorf("only SELECT queries are allowed"))
			}
			if limit > 0 && !sqlLimitRegex.MatchString(query) {
				query = strings.TrimRight(query, "; \t\n") + fmt.Sprintf(" LIMIT %d", limit)
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := st.DB().QueryContext(context.Background(), query)
			if err != nil {
				return err
			}
			defer rows.Close()
			result, err := rowsToMaps(rows)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "LIMIT to add when query has none")
	return cmd
}

func isSelectQuery(q string) bool {
	trimmed := strings.TrimSpace(q)
	if trimmed == "" {
		return false
	}
	first := strings.ToUpper(strings.SplitN(trimmed, " ", 2)[0])
	first = strings.TrimRight(first, ";")
	return first == "SELECT" || first == "WITH"
}
