// Copyright 2026 mathias-michel. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search the local synced TED notices store",
		Long: `Search the local SQLite store using FTS5 full-text search.

Requires notices to be synced first:
  eu-tenders-pp-cli sync --country DEU --cpv 72000000 --since 2024-01-01

Examples:
  eu-tenders-pp-cli search "cloud software"
  eu-tenders-pp-cli search "hospital construction" --limit 50 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := args[0]

			st, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()

			count, err := st.Count()
			if err == nil && count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: eu-tenders-pp-cli sync --country DEU --cpv 72000000 --since 2024-01-01\n")
				return nil
			}

			notices, err := st.Search(query, limit)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(notices)
			}

			if len(notices) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No results for %q\n", query)
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tBUYER\tTITLE\tCOUNTRY\tDATE\tVALUE")
			for _, n := range notices {
				val := ""
				if n.EstimatedValue > 0 {
					val = fmt.Sprintf("€%.0f", n.EstimatedValue)
				} else if n.ContractValue > 0 {
					val = fmt.Sprintf("€%.0f", n.ContractValue)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					n.ID,
					truncate(n.BuyerName, 30),
					truncate(n.Title, 40),
					n.BuyerCountry,
					n.PublicationDate,
					val,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")

	return cmd
}
