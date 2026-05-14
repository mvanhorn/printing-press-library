package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newQuotaCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "quota",
		Short:       "Show today's YouTube Data API quota usage (budget: 10,000 units/day)",
		Example:     "  yt-studio-pp-cli quota --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			total, byEndpoint, err := ytstore.QuotaUsedToday(ctx, db)
			if err != nil {
				return err
			}
			budget := 10000
			res := map[string]any{
				"budget":      budget,
				"used":        total,
				"remaining":   budget - total,
				"by_endpoint": byEndpoint,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Quota used today: %d / %d units (remaining: %d)\n", total, budget, budget-total)
			for ep, u := range byEndpoint {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s  %d\n", ep, u)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}
