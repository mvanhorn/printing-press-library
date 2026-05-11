package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newCompaniesSimilarCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "similar [slug]",
		Short:       "Rank peers of a YC company by tag overlap, industry match, and batch proximity.",
		Long:        "Local Jaccard score on tag set + same-industry bonus + batch-proximity bonus. Reads the synced companies table; needs 'sync' to have run first.",
		Example:     "  yc-companies-pp-cli companies similar stripe --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			hits, err := yclocal.Similar(cmd.Context(), st.DB(), args[0], limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, hits)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no similar companies — try a slug with more tags)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-15s %-25s %5s  %s\n", "SLUG", "NAME", "BATCH", "INDUSTRY", "SCORE", "SHARED TAGS")
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-15s %-25s %5.3f  %s\n", trunc(h.Slug, 25), trunc(h.Name, 25), trunc(h.Batch, 15), trunc(h.Industry, 25), h.Score, strings.Join(h.SharedTags, ", "))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum peers to return.")
	return cmd
}
