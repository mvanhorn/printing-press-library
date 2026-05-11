package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newBatchesShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show [batch]",
		Short:       "One-shot batch summary: count, top industries/tags, status mix, hiring share, median team size.",
		Long:        "Project a single batch into a card with company count, top 5 industries, top 10 tags, % hiring, % top company, status breakdown, and median team size. Accepts short batch names like 'w25' or 's24'.",
		Example:     "  yc-companies-pp-cli batches show w25\n  yc-companies-pp-cli batches show \"Winter 2025\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			card, err := yclocal.BatchSummary(cmd.Context(), st.DB(), args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, card)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Batch: %s\n", card.Batch)
			fmt.Fprintf(cmd.OutOrStdout(), "Companies: %d\n", card.CompanyCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Hiring: %.1f%%   Top company: %.1f%%\n", card.PctHiring, card.PctTop)
			fmt.Fprintf(cmd.OutOrStdout(), "Acquired: %.1f%%   Public: %.1f%%   Inactive: %.1f%%\n", card.PctAcquired, card.PctPublic, card.PctInactive)
			fmt.Fprintf(cmd.OutOrStdout(), "Median team size: %d\n\n", card.MedianTeamSize)
			fmt.Fprintln(cmd.OutOrStdout(), "Status breakdown:")
			keys := make([]string, 0, len(card.StatusBreakdown))
			for k := range card.StatusBreakdown {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %d\n", k, card.StatusBreakdown[k])
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nTop industries:")
			for _, kc := range card.TopIndustries {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d\n", trunc(kc.Name, 30), kc.Count)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nTop tags:")
			for _, kc := range card.TopTags {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d\n", trunc(kc.Name, 30), kc.Count)
			}
			return nil
		},
	}
	return cmd
}
