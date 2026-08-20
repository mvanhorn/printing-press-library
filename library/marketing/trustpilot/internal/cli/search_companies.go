package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <name>",
		Short: "Search Trustpilot for companies by name; returns canonical domain (identifyingName)",
		Example: `  trustpilot-pp-cli search "ThriftBooks" --limit 5 --json
  trustpilot-pp-cli search "Bookshop"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			sess, _, err := loadOrHarvestSession(ctx, db, true)
			if err != nil {
				return err
			}
			hits, err := fetchSearchWithRetry(ctx, db, &sess, query)
			if err != nil {
				return err
			}
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}
			out := make([]map[string]any, 0, len(hits))
			for _, h := range hits {
				if h.IdentifyingName != "" {
					_ = tpkg.UpsertCompany(ctx, db, tpkg.BusinessUnit{
						IdentifyingName: h.IdentifyingName,
						DisplayName:     h.DisplayName,
						TrustScore:      h.TrustScore,
						Stars:           h.Stars,
						NumberOfReviews: h.NumberOfReviews,
						ProfileImageURL: h.LogoURL,
					})
				}
				out = append(out, map[string]any{
					"domain":          h.IdentifyingName,
					"displayName":     h.DisplayName,
					"trustScore":      h.TrustScore,
					"stars":           h.Stars,
					"numberOfReviews": h.NumberOfReviews,
					"logoUrl":         h.LogoURL,
				})
			}
			payload := map[string]any{"query": query, "hits": out}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, h := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tTrustScore %.1f\t%d reviews\n",
					h["domain"], h["displayName"], h["trustScore"], h["numberOfReviews"])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum number of search hits to return")
	return cmd
}
