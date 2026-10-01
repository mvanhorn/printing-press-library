// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/asoview/internal/asoview"
	"github.com/spf13/cobra"
)

func newNovelOptionsCmd(f *rootFlags) *cobra.Command {
	var date, slot, party string
	var quantity int
	cmd := &cobra.Command{Use: "options <id>", Short: "Read advertised or dated fee bands with exact source units", Long: "Dated ticket bands require a slot; a single slot is selected automatically. Multiple slots return an explicit selection requirement. --party computes a labeled band subtotal, never a confirmed checkout total.", Example: "  asoview-pp-cli options ticket0000049223 --date 2026-10-02 --agent\n  asoview-pp-cli options pln3000044589 --date 2026-10-03 --agent", Annotations: sourceAnnotations("id=ticket0000049223;--date=2026-10-02;--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "options")
		}
		id, err := oneSourceID(args)
		if err != nil {
			return err
		}
		if party != "" {
			n, e := asoview.PartyQuantity(party)
			if e != nil {
				return sourceFailure(e)
			}
			if cmd.Flags().Changed("quantity") && quantity != n {
				return usageErr(fmt.Errorf("--quantity must equal the --party total (%d)", n))
			}
		}
		c, ctx, cancel, err := sourceClient(cmd, f)
		if err != nil {
			return err
		}
		defer cancel()
		out, err := c.Options(ctx, id, date, slot, quantity, party)
		if err != nil {
			return sourceFailure(err)
		}
		return sourceOutput(cmd, f, c, out)
	}}
	cmd.Flags().StringVar(&date, "date", "", "Fetch date-specific bands in YYYY-MM-DD instead of advertised bands")
	cmd.Flags().StringVar(&slot, "slot", "", "Select the source slot ID or its start time")
	cmd.Flags().StringVar(&party, "party", "", "Dated option-id:quantity pairs for a labeled unconfirmed subtotal")
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Units for selected slot stock check, bounded to 1..50")
	return cmd
}
