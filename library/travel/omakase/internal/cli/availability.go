// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelAvailabilityCmd(flags *rootFlags) *cobra.Command {
	var date string
	var party int
	cmd := &cobra.Command{Use: "availability [id]", Short: "Inspect public seat-query boundary for a planning date, party and restaurant ID"}
	cmd.Flags().StringVar(&date, "date", "", "Planning date YYYY-MM-DD; public query is not evaluated")
	cmd.Flags().IntVar(&party, "party", 0, "Planning party 1..20; supply together with date")
	return configurePlanningDetailCmd(flags, cmd, "availability", &date, &party)
}
