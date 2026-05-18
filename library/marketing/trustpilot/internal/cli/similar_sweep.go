package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPSimilarSweepCmd(flags *rootFlags) *cobra.Command {
	var limit, minReviews int
	cmd := &cobra.Command{
		Use:   "similar-sweep <domain>",
		Short: "Fetch the 8 'similar businesses' for a company and rank them by TrustScore",
		Example: `  trustpilot-pp-cli similar-sweep www.thriftbooks.com --limit 8 --json
  trustpilot-pp-cli similar-sweep bookshop.org --min-reviews 1000`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			seed := normalizeDomain(args[0])
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
			bu, err := tpkg.LoadCompany(ctx, db, seed)
			if err != nil || len(bu.SimilarBusinessUnits) == 0 {
				page, err := fetchPageWithRetry(ctx, db, &sess, seed, tpkg.PageFilters{Page: 1, Sort: "recency"})
				if err != nil {
					return err
				}
				bu = page.BusinessUnit
				_ = tpkg.UpsertCompany(ctx, db, bu)
			}
			similar := fetchSimilarCompanies(ctx, db, &sess, bu.SimilarBusinessUnits)
			filtered := make([]map[string]any, 0, len(similar))
			for _, c := range similar {
				if c.NumberOfReviews >= minReviews {
					filtered = append(filtered, map[string]any{
						"domain":          c.IdentifyingName,
						"displayName":     c.DisplayName,
						"trustScore":      c.TrustScore,
						"numberOfReviews": c.NumberOfReviews,
					})
				}
			}
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i]["trustScore"].(float64) > filtered[j]["trustScore"].(float64)
			})
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			payload := map[string]any{"seed": seed, "similar": filtered}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, c := range filtered {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tTrustScore %.1f\t%d reviews\n",
					c["domain"], c["displayName"], c["trustScore"], c["numberOfReviews"])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 8, "Maximum similar businesses to return")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 0, "Minimum total reviews")
	return cmd
}

func fetchSimilarCompanies(ctx context.Context, db *sql.DB, sess *tpkg.Session, sims []tpkg.SimilarBusiness) []tpkg.SimilarBusiness {
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var mu sync.Mutex
	out := make([]tpkg.SimilarBusiness, 0, len(sims))
	for _, sim := range sims {
		sim := sim
		if sim.IdentifyingName == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			localSess := *sess
			page, err := fetchPageWithRetry(ctx, db, &localSess, sim.IdentifyingName, tpkg.PageFilters{Page: 1, Sort: "recency"})
			if err == nil {
				_ = tpkg.UpsertCompany(ctx, db, page.BusinessUnit)
				sim.DisplayName = page.BusinessUnit.DisplayName
				sim.TrustScore = page.BusinessUnit.TrustScore
				sim.NumberOfReviews = page.BusinessUnit.NumberOfReviews
			}
			mu.Lock()
			out = append(out, sim)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}
