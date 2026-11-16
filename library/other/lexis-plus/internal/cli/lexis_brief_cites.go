// Hand-written: brief cites (pure computation over a file you supply).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBriefCitesCmd(flags *rootFlags) *cobra.Command {
	cites := &cobra.Command{
		Use:         "cites <file>",
		Short:       "List the California cases and statutes cited in a draft",
		Long:        "Use this command to list the authorities cited in a document. Do NOT use it to download anything; use 'batch --file' for that.",
		Example:     "  lexis-plus-pp-cli brief cites motion.docx > cites.txt",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "file=example-value", "pp:verified-by-proof": "brief-citecheck-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source computed
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a file"); err != nil {
				return err
			}
			cs, err := briefCites(args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), cs, flags)
			}
			for _, c := range cs {
				fmt.Fprintln(cmd.OutOrStdout(), c.Cite)
			}
			return nil
		},
	}

	return cites
}
