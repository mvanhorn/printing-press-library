// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

func newNovelAffiliationGrowthCmd(flags *rootFlags) *cobra.Command {
	var journal string
	var years int
	var threshold int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "affiliation-growth",
		Short: "Find institutions gaining publication velocity in Lancet journals",
		Long: "Compare institutional publication counts between the most recent N years and\n" +
			"the equal-length window before it, surfacing rising research centers. Reads\n" +
			"the local mirror; run 'thelancet-pp-cli refresh' first.",
		Example:     "  thelancet-pp-cli affiliation-growth --journal lancet-neurology --years 5 --json\n  thelancet-pp-cli affiliation-growth --years 4 --threshold 3",
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
			if years < 1 {
				years = 5
			}
			st, err := ensureLancetStore(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := lancet.AffiliationGrowth(cmd.Context(), st.DB(), issn, years, threshold, limit)
			if err != nil {
				return fmt.Errorf("computing affiliation growth: %w", err)
			}
			if rows == nil {
				rows = []lancet.InstGrowth{}
			}
			if done, err := emitLancet(cmd, flags, rows); done || err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no institutions met the threshold (try a smaller --threshold or run refresh)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s %8s %8s %8s\n", "INSTITUTION", "RECENT", "PRIOR", "GROWTH")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40.40s %8d %8d %+8d\n", r.Institution, r.RecentCount, r.PriorCount, r.Growth)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&journal, "journal", "", "Scope to a Lancet journal slug, or omit for all")
	cmd.Flags().IntVar(&years, "years", 5, "Window length in years (recent vs prior window of the same length)")
	cmd.Flags().IntVar(&threshold, "threshold", 2, "Minimum recent-window publication count to include an institution")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum institutions to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}
