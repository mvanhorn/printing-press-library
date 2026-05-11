package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Cross-batch and cross-industry aggregates over the local companies table.",
		Long:        "GROUP BY pivots: count, average team_size, % hiring, % top, % acquired/active/public/inactive per cell.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newStatsByBatchCmd(flags))
	cmd.AddCommand(newStatsByIndustryCmd(flags))
	return cmd
}

func newStatsByBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		industry string
		tag      string
		region   string
	)
	cmd := &cobra.Command{
		Use:         "by-batch",
		Short:       "Aggregates grouped by batch, optionally filtered by industry/tag/region.",
		Example:     "  yc-companies-pp-cli stats by-batch --industry fintech --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := yclocal.Stats(cmd.Context(), st.DB(), yclocal.StatsQuery{
				GroupBy:  "batch",
				Industry: industry,
				Tag:      tag,
				Region:   region,
			})
			if err != nil {
				return err
			}
			return printStatsRows(cmd, flags, rows, "BATCH")
		},
	}
	cmd.Flags().StringVar(&industry, "industry", "", "Filter to this industry (e.g. Fintech, Healthcare).")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter to this tag (substring match against tags JSON).")
	cmd.Flags().StringVar(&region, "region", "", "Filter to this region (substring match against regions JSON).")
	return cmd
}

func newStatsByIndustryCmd(flags *rootFlags) *cobra.Command {
	var (
		batch  string
		tag    string
		region string
	)
	cmd := &cobra.Command{
		Use:         "by-industry",
		Short:       "Aggregates grouped by industry, optionally filtered by batch/tag/region.",
		Example:     "  yc-companies-pp-cli stats by-industry --batch \"Winter 2025\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := yclocal.Stats(cmd.Context(), st.DB(), yclocal.StatsQuery{
				GroupBy: "industry",
				Batch:   batch,
				Tag:     tag,
				Region:  region,
			})
			if err != nil {
				return err
			}
			return printStatsRows(cmd, flags, rows, "INDUSTRY")
		},
	}
	cmd.Flags().StringVar(&batch, "batch", "", "Filter to this batch (e.g. 'Winter 2025').")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter to this tag.")
	cmd.Flags().StringVar(&region, "region", "", "Filter to this region.")
	return cmd
}

func printStatsRows(cmd *cobra.Command, flags *rootFlags, rows []yclocal.StatsCell, keyHeader string) error {
	if flags.asJSON {
		return flags.printJSON(cmd, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no rows match — try 'sync' first or relax filters)")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-22s %6s %12s %10s %8s %10s %10s\n", keyHeader, "COUNT", "AVG_TEAM", "HIRING_%", "TOP_%", "ACQ_%", "PUB_%")
	for _, r := range rows {
		fmt.Fprintf(cmd.OutOrStdout(), "%-22s %6d %12.1f %10.1f %8.1f %10.1f %10.1f\n", trunc(r.Key, 22), r.Count, r.AvgTeamSize, r.PctHiring, r.PctTop, r.PctAcquired, r.PctPublic)
	}
	return nil
}
