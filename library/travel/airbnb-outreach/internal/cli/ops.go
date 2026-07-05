// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/spf13/cobra"
)

func newOpsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ops",
		Short: "Inspect and self-heal the GraphQL operation-hash registry",
		Long: `Airbnb rotates its persisted-query hashes on every deploy. 'ops refresh'
re-harvests the current hashes from Airbnb's own JS bundles so the CLI keeps
working without a code update. 'ops list' shows what's known.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOpsRefreshCmd(flags))
	cmd.AddCommand(newOpsListCmd(flags))
	return cmd
}

func newOpsRefreshCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Re-harvest current operation hashes from Airbnb's JS bundles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newAirbnbClient(flags)
			harvested, err := airbnb.Harvest(c, airbnb.DefaultHarvestRoutes)
			if err != nil {
				return apiErr(fmt.Errorf("harvesting operation hashes: %w", err))
			}
			n, err := airbnb.SaveOverrides(harvested)
			if err != nil {
				return err
			}
			result := map[string]any{"status": "refreshed", "harvested": len(harvested), "total_known": n}
			if flags.asJSON {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s Harvested %d operations (%d total known).\n", green("✓"), len(harvested), n)
			return nil
		},
	}
}

func newOpsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List known operations and whether each hash is bundled or refreshed",
		Example: "  airbnb-outreach-pp-cli ops list --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := airbnb.LoadRegistry()
			ops := reg.Operations()
			type row struct {
				Operation string `json:"operation"`
				Source    string `json:"source"`
				Hash      string `json:"hash"`
			}
			rows := make([]row, 0, len(ops))
			for _, op := range ops {
				rows = append(rows, row{Operation: op, Source: reg.Source(op), Hash: reg.Hash(op)})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Operation < rows[j].Operation })
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("OPERATION\tSOURCE\tHASH"))
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Operation, r.Source, truncate(r.Hash, 16))
			}
			return tw.Flush()
		},
	}
}
