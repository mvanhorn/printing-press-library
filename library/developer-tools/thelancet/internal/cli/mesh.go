// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

func newNovelMeshCmd(flags *rootFlags) *cobra.Command {
	var org string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "mesh",
		Short: "Map co-authorship pairs within an institution across Lancet papers",
		Long: "Find researchers affiliated with an institution who co-author Lancet papers,\n" +
			"ranked by the number of papers they share. Reads the local mirror; run\n" +
			"'thelancet-pp-cli refresh' first.",
		Example:     "  thelancet-pp-cli mesh --org 'Stanford University' --json\n  thelancet-pp-cli mesh --org 'Oxford' --limit 30",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--org=University College London"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if org == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--org is required (the institution to map)"))
			}
			st, err := ensureLancetStore(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			edges, err := lancet.CoAuthorMesh(cmd.Context(), st.DB(), org, limit)
			if err != nil {
				return fmt.Errorf("building mesh: %w", err)
			}
			if edges == nil {
				edges = []lancet.CoAuthorEdge{}
			}
			if done, err := emitLancet(cmd, flags, edges); done || err != nil {
				return err
			}
			if len(edges) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no co-authorship pairs found for %q (try a distinctive substring, or run refresh)\n", org)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-28s %6s\n", "AUTHOR A", "AUTHOR B", "SHARED")
			for _, e := range edges {
				fmt.Fprintf(cmd.OutOrStdout(), "%-28.28s %-28.28s %6d\n", e.AuthorA, e.AuthorB, e.SharedWorks)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "Institution to map (substring match against OpenAlex display names)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum co-authorship pairs to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}
