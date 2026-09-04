// Hand-authored transcendence: compare two designers head-to-head.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDesignerCompareCmd(flags *rootFlags) *cobra.Command {
	var maxScanPages int
	cmd := &cobra.Command{
		Use:   "designer-compare <A> <B>",
		Short: "Compare two designers head-to-head (size, type mix, price band, free/POD share)",
		Long: `Profile two designers and render them side by side.

For a single designer profile use 'designer-stats'.`,
		Example:     strings.Trim("\n  creativefabrica-pp-cli designer-compare \"DigiArt\" \"CraftLab\" --agent\n  creativefabrica-pp-cli designer-compare 2880714 123456", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two designer ids or names are required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			a, err := profileDesigner(cmd, flags, args[0], maxScanPages)
			if err != nil {
				return err
			}
			b, err := profileDesigner(cmd, flags, args[1], maxScanPages)
			if err != nil {
				return err
			}
			cmpOut := map[string]any{"a": a, "b": b}
			if flags.asJSON || flags.agent || !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, cmpOut)
			}
			rows := [][]string{
				{"Total products", fmt.Sprintf("%d", a.Total), fmt.Sprintf("%d", b.Total)},
				{"Scanned", fmt.Sprintf("%d", a.Scanned), fmt.Sprintf("%d", b.Scanned)},
				{"Free", fmt.Sprintf("%d", a.FreeCount), fmt.Sprintf("%d", b.FreeCount)},
				{"POD", fmt.Sprintf("%d", a.PodCount), fmt.Sprintf("%d", b.PodCount)},
				{"On sale", fmt.Sprintf("%d", a.OnSaleCount), fmt.Sprintf("%d", b.OnSaleCount)},
				{"Median price", fmt.Sprintf("$%.2f", a.MedianPrice), fmt.Sprintf("$%.2f", b.MedianPrice)},
				{"Top type", topType(a), topType(b)},
			}
			return flags.printTable(cmd, []string{"METRIC", truncate(a.Designer, 22), truncate(b.Designer, 22)}, rows)
		},
	}
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Max catalog pages to scan per designer")
	return cmd
}

func topType(p designerProfile) string {
	if len(p.TypeMix) == 0 {
		return "-"
	}
	return fmt.Sprintf("%s (%d)", p.TypeMix[0].Value, p.TypeMix[0].Count)
}
