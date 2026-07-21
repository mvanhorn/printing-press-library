// Copyright 2026 beetz12. Licensed under Apache-2.0.
// `recommend` - weighted fit scoring over care.com caregiver search results.
// Hand-authored; safe across regen.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type careScored struct {
	careCaregiverSummary
	Score   int      `json:"fit_score"`
	Reasons []string `json:"reasons"`
}

// scoreCaregiver produces a 0-100 fit score plus human reasons. targetRate is
// the family's ideal hourly rate (0 = ignore rate).
func scoreCaregiver(s careCaregiverSummary, targetRate float64) (int, []string) {
	var score float64
	var reasons []string

	// Reviews (up to 25): log-scaled so 20+ reviews saturates.
	if s.TotalReviews > 0 {
		rv := math.Min(1, math.Log10(float64(s.TotalReviews)+1)/math.Log10(21))
		score += rv * 25
		reasons = append(reasons, fmt.Sprintf("%d reviews", s.TotalReviews))
	}
	// Rating (up to 25).
	if s.AvgRating > 0 {
		score += (s.AvgRating / 5.0) * 25
		reasons = append(reasons, fmt.Sprintf("%.1f stars", s.AvgRating))
	}
	// Experience (up to 20), saturating at 10 years.
	if s.YearsExperience > 0 {
		exp := math.Min(1, float64(s.YearsExperience)/10.0)
		score += exp * 20
		reasons = append(reasons, fmt.Sprintf("%dy exp", s.YearsExperience))
	}
	// Background check / CareCheck (10).
	if s.HasCareCheck || containsFold(s.Badges, "BACKGROUND_CHECK") {
		score += 10
		reasons = append(reasons, "background-checked")
	}
	// Proven hires (up to 10).
	if s.HiredTimes > 0 {
		score += math.Min(1, float64(s.HiredTimes)/5.0) * 10
		reasons = append(reasons, fmt.Sprintf("hired %dx", s.HiredTimes))
	}
	// Rate fit (up to 10): full marks at or under target, tapering above.
	if targetRate > 0 && s.HourlyRate > 0 {
		if s.HourlyRate <= targetRate {
			score += 10
			reasons = append(reasons, fmt.Sprintf("$%.0f/hr (within budget)", s.HourlyRate))
		} else {
			over := (s.HourlyRate - targetRate) / targetRate
			score += math.Max(0, 10*(1-over))
			reasons = append(reasons, fmt.Sprintf("$%.0f/hr (over $%.0f budget)", s.HourlyRate, targetRate))
		}
	} else if s.HourlyRate > 0 {
		reasons = append(reasons, fmt.Sprintf("$%.0f/hr", s.HourlyRate))
	}
	if s.IsFavorite {
		reasons = append(reasons, "favorited")
	}
	return int(math.Round(score)), reasons
}

func containsFold(xs []string, want string) bool {
	for _, x := range xs {
		if strings.EqualFold(x, want) {
			return true
		}
	}
	return false
}

func newCareRecommendCmd(flags *rootFlags) *cobra.Command {
	var zip, careType, sortOrder string
	var limit, scan, minExp int
	var targetRate, maxRate float64

	cmd := &cobra.Command{
		Use:   "recommend",
		Short: "Rank caregivers by a weighted fit score and explain why",
		Long:  "Scores caregivers near a zip on reviews, rating, experience, background checks, proven hires, and rate-fit, then returns the top matches with the reasons behind each score. Use --target-rate to weight affordability.",
		Example: strings.Trim(`
  care-pp-cli recommend --zip 90210 --target-rate 22
  care-pp-cli recommend --zip 90210 --scan 40 --limit 5 --min-exp 3 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if zip == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--zip is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would score caregivers near %s (target $%.0f/hr)\n", zip, targetRate)
				return nil
			}
			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()

			if scan < limit {
				scan = limit * 4
			}
			if scan < 10 {
				scan = 10
			}
			if scan > 100 {
				scan = 100
			}
			vars := map[string]any{"input": map[string]any{
				"careType": careType,
				"filters": map[string]any{
					"postalCode":      zip,
					"searchPageSize":  scan,
					"searchSortOrder": sortOrder,
				},
			}}
			data, err := careGraphQL(ctx, flags, careQSearchProvidersOp, careQSearchProviders, vars)
			if err != nil {
				return err
			}
			var wrap struct {
				Search struct {
					Message string `json:"message"`
					Conn    struct {
						TotalHits int `json:"totalHits"`
						Edges     []struct {
							Node careCaregiverNode `json:"node"`
						} `json:"edges"`
					} `json:"searchProvidersConnection"`
				} `json:"searchProvidersChildCare"`
			}
			if err := json.Unmarshal(data, &wrap); err != nil {
				return fmt.Errorf("parsing caregiver results: %w", err)
			}
			// Surface a search-level error variant instead of returning an empty
			// recommendation list (which would look like a zip with no caregivers).
			if wrap.Search.Message != "" && wrap.Search.Conn.TotalHits == 0 {
				return fmt.Errorf("care.com search error: %s", wrap.Search.Message)
			}

			scored := make([]careScored, 0, len(wrap.Search.Conn.Edges))
			for _, e := range wrap.Search.Conn.Edges {
				if e.Node.Typename != "Caregiver" {
					continue
				}
				s := e.Node.summary()
				if minExp > 0 && s.YearsExperience < minExp {
					continue
				}
				if maxRate > 0 && s.HourlyRate > maxRate {
					continue
				}
				sc, reasons := scoreCaregiver(s, targetRate)
				scored = append(scored, careScored{careCaregiverSummary: s, Score: sc, Reasons: reasons})
			}
			sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
			if limit > 0 && len(scored) > limit {
				scored = scored[:limit]
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				if len(scored) == 0 {
					fmt.Fprintf(w, "No caregivers matched near %s.\n", zip)
					return nil
				}
				fmt.Fprintf(w, "%-4s %-20s %-14s %s\n", "SCORE", "NAME", "CITY", "WHY")
				for _, c := range scored {
					fmt.Fprintf(w, "%-4d %-20s %-14s %s\n", c.Score, truncate(c.Name, 20), truncate(c.City, 14), strings.Join(c.Reasons, ", "))
				}
				fmt.Fprintf(w, "\nTop %d of %d scored near %s.\n", len(scored), len(scored), zip)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), scored, flags)
		},
	}
	cmd.Flags().StringVar(&zip, "zip", "", "postal code to search near (required)")
	cmd.Flags().StringVar(&careType, "care-type", "SITTER", "care.com careType (SITTER for babysitters/nannies)")
	cmd.Flags().StringVar(&sortOrder, "sort", "SORT_ORDER_RECOMMENDED_DESCENDING", "care.com search sort order")
	cmd.Flags().IntVar(&limit, "limit", 10, "how many top matches to return")
	cmd.Flags().IntVar(&scan, "scan", 0, "how many candidates to score before ranking (default limit×4)")
	cmd.Flags().IntVar(&minExp, "min-exp", 0, "minimum years of experience")
	cmd.Flags().Float64Var(&targetRate, "target-rate", 0, "ideal hourly rate for affordability weighting (USD)")
	cmd.Flags().Float64Var(&maxRate, "max-rate", 0, "exclude caregivers above this hourly rate (USD)")
	return cmd
}
