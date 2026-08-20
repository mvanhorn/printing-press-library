// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

func newNovelVisibilityGapCmd(flags *rootFlags) *cobra.Command {
	var institution string
	var minWorks int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "visibility-gap",
		Short: "Find authors whose citation impact is out of step with their journals",
		Long: "Compare each author's average citations against the average citations of the\n" +
			"Lancet journals they publish in (a prestige proxy), surfacing authors most\n" +
			"out of step. Optionally scope to an institution. Reads the local mirror; run\n" +
			"'thelancet-pp-cli refresh' first.",
		Example:     "  thelancet-pp-cli visibility-gap --institution 'Oxford' --json\n  thelancet-pp-cli visibility-gap --min-works 3 --limit 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if minWorks < 1 {
				minWorks = 1
			}
			st, err := ensureLancetStore(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := lancet.VisibilityGap(cmd.Context(), st.DB(), institution, minWorks, limit)
			if err != nil {
				return fmt.Errorf("computing visibility gap: %w", err)
			}
			if rows == nil {
				rows = []lancet.VisibilityRow{}
			}
			if done, err := emitLancet(cmd, flags, rows); done || err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no authors met the criteria (try a smaller --min-works or run refresh)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-32s %6s %12s %12s %9s\n", "AUTHOR", "WORKS", "AUTHOR_AVG", "JOURNAL_AVG", "GAP")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-32.32s %6d %12.1f %12.1f %+9.1f\n", r.AuthorName, r.Works, r.AuthorAvgCite, r.JournalAvg, r.Gap)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&institution, "institution", "", "Scope to authors affiliated with this institution (substring match)")
	cmd.Flags().IntVar(&minWorks, "min-works", 2, "Minimum works an author must have to be included")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum authors to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}
