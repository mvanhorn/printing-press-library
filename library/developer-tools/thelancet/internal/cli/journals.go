// Hand-authored Lancet journal-registry command. Not generated.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

func newJournalsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "journals",
		Aliases:     []string{"journal-list"},
		Short:       "List The Lancet family of journals (slug, ISSN, name)",
		Long:        "List every journal in the curated Lancet family registry. Use a slug with\nother commands' --journal flag to scope queries to that title.",
		Example:     "  thelancet-pp-cli journals\n  thelancet-pp-cli journals --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			js := lancet.Journals()
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), js, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-38s %-11s %s\n", "SLUG", "ISSN", "NAME")
			for _, j := range js {
				fmt.Fprintf(cmd.OutOrStdout(), "%-38s %-11s %s\n", j.Slug, j.ISSN, j.Display)
			}
			return nil
		},
	}
	return cmd
}
