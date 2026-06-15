package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newLocationsCmd(g *globalOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "locations",
		Short: "List MasterPark parking locations (Lot B / Lot G)",
		Example: `  masterpark-pp-cli locations
  masterpark-pp-cli locations --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := g.ctx()
			defer cancel()
			locs, err := g.newClient().Locations(ctx)
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(locs)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tCODE_ID")
			for _, l := range locs {
				fmt.Fprintf(w, "%s\t%s\n", l.Name, l.CodeID)
			}
			return w.Flush()
		},
	}
}
