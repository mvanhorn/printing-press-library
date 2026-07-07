package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/foxnews/internal/foxnews"
	"github.com/spf13/cobra"
)

func newSectionsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sections",
		Short: "List available Fox News RSS sections",
		Example: `  foxnews-pp-cli sections
  foxnews-pp-cli sections --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows := make([]foxnews.Section, len(foxnews.Sections))
			copy(rows, foxnews.Sections)
			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				meta := map[string]any{
					"source":          "catalog",
					"default_section": "latest",
					"count":           len(rows),
				}
				return printMachineOutput(out, flags, meta, rows)
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tLABEL\tFEED PATH")
			for _, s := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.ID, s.Label, s.Path)
			}
			return w.Flush()
		},
	}
}
