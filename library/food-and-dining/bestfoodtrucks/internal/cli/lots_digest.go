// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

type LotDigestEntry struct {
	SEOName string                `json:"seoName"`
	Success bool                  `json:"success"`
	Error   string                `json:"error,omitempty"`
	Lot     *GqlLotScheduleResult `json:"lot,omitempty"`
	Digest  string                `json:"digest,omitempty"`
}

func newNovelLotsDigestCmd(flags *rootFlags) *cobra.Command {
	var flagLots string
	var days int

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Combines multiple lots' schedules into one view in a single command instead of visiting each lot's page separately.",
		Long:  "Combines multiple lots' schedules into one view in a single command instead of visiting each lot's page separately.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli lots digest --lots playa-district,at-t-los-angeles
  bestfoodtrucks-pp-cli lots digest --lots playa-district,at-t-los-angeles --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagLots == "" && len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--lots is required"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "lots digest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			lotsCSV := flagLots
			if flagLots == "" && len(args) > 0 {
				lotsCSV = args[0]
			}

			parts := strings.Split(lotsCSV, ",")
			var cleanLots []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cleanLots = append(cleanLots, p)
				}
			}

			if len(cleanLots) == 0 {
				return usageErr(fmt.Errorf("no valid lot slugs provided"))
			}

			// Parallel fetch with bounded concurrency or direct sync group
			results := make([]LotDigestEntry, len(cleanLots))
			var wg sync.WaitGroup

			client := graphqlclient.New(flags.timeout)
			query := `
				query GetLotSchedule($seoName: String!, $days: Int!) {
					lot(seoName: $seoName) {
						id
						name
						locationSchedule(days: $days) {
							id
							dateAlias
							locations {
								id
								startTime
								endTime
								workStatusHuman
								allowOrders
								customerUrl
								truck {
									id
									name
								}
							}
						}
					}
				}
			`

			for i, lotSlug := range cleanLots {
				wg.Add(1)
				go func(idx int, slug string) {
					defer wg.Done()

					entry := LotDigestEntry{
						SEOName: slug,
						Success: true,
					}

					var res struct {
						Lot *GqlLotScheduleResult `json:"lot"`
					}
					err := client.Query(ctx, query, map[string]any{"seoName": slug, "days": days}, &res)
					if err != nil {
						entry.Success = false
						entry.Error = err.Error()
					} else if res.Lot == nil {
						entry.Success = false
						entry.Error = "lot not found"
					} else {
						entry.Lot = res.Lot
						entry.Digest = formatLotDigest(res.Lot.Name, slug, res.Lot.LocationSchedule)
					}

					results[idx] = entry
				}(i, lotSlug)
			}

			wg.Wait()

			if flags.asJSON {
				type lotsEnvelope struct {
					Lots []LotDigestEntry `json:"lots"`
				}
				return printJSONFiltered(cmd.OutOrStdout(), lotsEnvelope{Lots: results}, flags)
			}

			w := cmd.OutOrStdout()
			for i, res := range results {
				if i > 0 {
					fmt.Fprintln(w, strings.Repeat("-", 40))
				}
				if !res.Success {
					fmt.Fprintf(w, "Lot %q could not be loaded: %s\n", res.SEOName, res.Error)
				} else {
					fmt.Fprint(w, res.Digest)
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flagLots, "lots", "", "CSV list of lot seoNames")
	cmd.Flags().IntVar(&days, "days", 5, "Number of days of schedule to fetch")
	return cmd
}
