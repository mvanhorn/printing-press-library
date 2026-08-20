// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

func newNovelRankAuthorsCmd(flags *rootFlags) *cobra.Command {
	var institution string
	var journal string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "rank-authors",
		Short: "Rank authors by citation impact within a Lancet journal or institution",
		Long: "Rank researchers by total citations across the local store, optionally scoped\n" +
			"to a Lancet journal (--journal) and/or an institution (--institution). Reads\n" +
			"the local mirror; run 'thelancet-pp-cli refresh' first.",
		Example:     "  thelancet-pp-cli rank-authors --journal lancet-oncology --json\n  thelancet-pp-cli rank-authors --institution 'Harvard' --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			issn, err := resolveJournalISSN(journal)
			if err != nil {
				_ = cmd.Usage()
				return err
			}
			st, err := ensureLancetStore(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := lancet.RankAuthors(cmd.Context(), st.DB(), issn, institution, limit)
			if err != nil {
				return fmt.Errorf("ranking authors: %w", err)
			}
			if rows == nil {
				rows = []lancet.AuthorRank{}
			}
			if done, err := emitLancet(cmd, flags, rows); done || err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no authors found (try a wider scope or run refresh)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-32s %6s %10s %8s\n", "AUTHOR", "WORKS", "CITATIONS", "AVG")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-32.32s %6d %10d %8.1f\n", r.AuthorName, r.Works, r.TotalCitations, r.AvgCitations)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&institution, "institution", "", "Filter to authors affiliated with this institution (substring match)")
	cmd.Flags().StringVar(&journal, "journal", "", "Scope to a Lancet journal slug (e.g. lancet-oncology), or omit for all")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum authors to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}
