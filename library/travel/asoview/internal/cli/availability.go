// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelAvailabilityCmd(f *rootFlags) *cobra.Command {
	var date, month string
	var quantity int
	cmd := &cobra.Command{Use: "availability <id>", Short: "Inspect public dated stock and source entry windows", Long: "Read public stock without holding it. Date validity, admission windows and reserved time slots remain distinct; quantity checks do not verify age eligibility or a checkout quote.", Example: "  asoview-pp-cli availability ticket0000049223 --date 2026-10-02 --quantity 2 --agent\n  asoview-pp-cli availability pln3000044589 --month 2026-10 --agent", Annotations: sourceAnnotations("id=ticket0000049223;--date=2026-10-02;--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "availability")
		}
		id, err := oneSourceID(args)
		if err != nil {
			return err
		}
		if err = validateDateMode(date, month); err != nil {
			return err
		}
		c, ctx, cancel, err := sourceClient(cmd, f)
		if err != nil {
			return err
		}
		defer cancel()
		out, err := c.Availability(ctx, id, date, month, quantity)
		if err != nil {
			return sourceFailure(err)
		}
		return sourceOutput(cmd, f, c, out)
	}}
	cmd.Flags().StringVar(&date, "date", "", "Source date in YYYY-MM-DD; requests one day's slots")
	cmd.Flags().StringVar(&month, "month", "", "Calendar month YYYY-MM; requests one month's days only")
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Units requested for slot quantity check, bounded to 1..50")
	return cmd
}
