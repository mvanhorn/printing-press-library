// Copyright 2026 beetz12. Licensed under Apache-2.0.
// Friendly caregiver-search command over care.com's SearchProvidersChildCare
// GraphQL operation. Hand-authored; safe across regen.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// flexStr unmarshals a JSON value that may be a string or a number into a string.
type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	*f = flexStr(strings.Trim(string(b), `"`))
	return nil
}

// careGraphQL posts an operation to /api/graphql and returns the inner GraphQL
// `data` object, surfacing transport and GraphQL-level errors.
func careGraphQL(ctx context.Context, flags *rootFlags, opName, query string, variables any) (json.RawMessage, error) {
	return careGraphQLH(ctx, flags, opName, query, variables, nil)
}

// careGraphQLH is careGraphQL with per-request header overrides. Some care.com
// resolvers gate on `apollographql-client-name` (e.g. the applicant list only
// answers `job-mfe`, not the default `care-web`).
func careGraphQLH(ctx context.Context, flags *rootFlags, opName, query string, variables any, headers map[string]string) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{"operationName": opName, "query": query, "variables": variables}
	var raw json.RawMessage
	var status int
	if len(headers) > 0 {
		raw, status, err = c.PostWithParamsAndHeaders(ctx, "/api/graphql", map[string]string{}, body, headers)
	} else {
		raw, status, err = c.PostWithParams(ctx, "/api/graphql", map[string]string{}, body)
	}
	if err != nil {
		return nil, err
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing care.com response (HTTP %d): %w", status, err)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("care.com GraphQL error: %s", env.Errors[0].Message)
	}
	if len(env.Data) == 0 {
		return nil, fmt.Errorf("empty care.com response (HTTP %d); your session may be stale - run: care-pp-cli auth refresh", status)
	}
	return env.Data, nil
}

// --- response shapes (subset of the CaregiverFragment we care about) ---

type careRate struct {
	HourlyRate struct {
		Amount flexStr `json:"amount"`
	} `json:"hourlyRate"`
	NumberOfChildren int `json:"numberOfChildren"`
}

type careCaregiverNode struct {
	Typename string `json:"__typename"`
	Member   struct {
		ID          string `json:"id"`
		LegacyID    string `json:"legacyId"`
		DisplayName string `json:"displayName"`
		FirstName   string `json:"firstName"`
		LastName    string `json:"lastName"`
		ImageURL    string `json:"imageURL"`
		Address     struct {
			City  string `json:"city"`
			State string `json:"state"`
			Zip   string `json:"zip"`
		} `json:"address"`
	} `json:"member"`
	YearsOfExperience int      `json:"yearsOfExperience"`
	HasCareCheck      bool     `json:"hasCareCheck"`
	Badges            []string `json:"badges"`
	HiredTimes        int      `json:"hiredTimes"`
	IsFavorite        bool     `json:"isFavorite"`
	IsAvailable       bool     `json:"isAvailable"`
	RevieweeMetrics   struct {
		Metrics struct {
			TotalReviews   int `json:"totalReviews"`
			AverageRatings []struct {
				Type  string  `json:"type"`
				Value float64 `json:"value"`
			} `json:"averageRatings"`
		} `json:"metrics"`
	} `json:"revieweeMetrics"`
	Profiles struct {
		CommonCaregiverProfile struct {
			Bio struct {
				ExperienceSummary string `json:"experienceSummary"`
			} `json:"bio"`
		} `json:"commonCaregiverProfile"`
		ChildCareCaregiverProfile struct {
			Rates []careRate `json:"rates"`
		} `json:"childCareCaregiverProfile"`
	} `json:"profiles"`
}

// careCaregiverSummary is the clean, agent-friendly output shape.
type careCaregiverSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	City            string   `json:"city"`
	State           string   `json:"state"`
	YearsExperience int      `json:"years_experience"`
	HourlyRate      float64  `json:"hourly_rate,omitempty"`
	TotalReviews    int      `json:"total_reviews"`
	AvgRating       float64  `json:"avg_rating,omitempty"`
	HasCareCheck    bool     `json:"has_care_check"`
	HiredTimes      int      `json:"hired_times"`
	IsFavorite      bool     `json:"is_favorite"`
	Badges          []string `json:"badges,omitempty"`
	Bio             string   `json:"bio,omitempty"`
	ProfileURL      string   `json:"profile_url"`
}

func (n careCaregiverNode) summary() careCaregiverSummary {
	s := careCaregiverSummary{
		ID:              n.Member.ID,
		Name:            strings.TrimSpace(n.Member.FirstName + " " + firstInitial(n.Member.LastName)),
		City:            n.Member.Address.City,
		State:           n.Member.Address.State,
		YearsExperience: n.YearsOfExperience,
		TotalReviews:    n.RevieweeMetrics.Metrics.TotalReviews,
		HasCareCheck:    n.HasCareCheck,
		HiredTimes:      n.HiredTimes,
		IsFavorite:      n.IsFavorite,
		Badges:          n.Badges,
		Bio:             strings.TrimSpace(n.Profiles.CommonCaregiverProfile.Bio.ExperienceSummary),
	}
	if len(n.Profiles.ChildCareCaregiverProfile.Rates) > 0 {
		if v, err := strconv.ParseFloat(string(n.Profiles.ChildCareCaregiverProfile.Rates[0].HourlyRate.Amount), 64); err == nil {
			s.HourlyRate = v
		}
	}
	for _, r := range n.RevieweeMetrics.Metrics.AverageRatings {
		if strings.EqualFold(r.Type, "OVERALL") || s.AvgRating == 0 {
			s.AvgRating = r.Value
		}
	}
	if n.Member.ID != "" {
		s.ProfileURL = "https://www.care.com/app/vip/" + n.Member.ID
	}
	return s
}

func firstInitial(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + "."
}

func newCareFindCmd(flags *rootFlags) *cobra.Command {
	var zip, careType, sortOrder, jobRef string
	var limit, minExp, minReviews int
	var minRate, maxRate float64

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search care.com caregivers by zip with filters",
		Long:  "Search caregivers near a zip code for a specific job you're hiring for, with local filters for rate, experience, and reviews. Requires --job (name or id) so any caregiver you find can be messaged under it; that job becomes the active job for `outreach send`. Backed by care.com's SearchProvidersChildCare.",
		Example: strings.Trim(`
  care-pp-cli find --job "Summer Sitter" --zip 90210
  care-pp-cli find --job 12345678 --zip 90210 --min-exp 5 --max-rate 25 --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if zip == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would search caregivers near %s (careType=%s)\n", zip, careType)
				return nil
			}
			if zip == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--zip is required"))
			}

			pageSize := limit
			if pageSize < 1 {
				pageSize = 10
			}
			// Over-fetch so local filters still return a full page.
			fetchSize := pageSize
			if minRate > 0 || maxRate > 0 || minExp > 0 || minReviews > 0 {
				fetchSize = pageSize * 4
				if fetchSize > 100 {
					fetchSize = 100
				}
			}

			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()

			// Tie every search to a hiring job so found caregivers can be messaged
			// under it. The resolved job becomes the active job for `outreach send`.
			if strings.TrimSpace(jobRef) == "" {
				return usageErr(fmt.Errorf("--job <name|id> is required (the job you're hiring for); see your jobs: care-pp-cli job list"))
			}
			job, err := careResolveJob(ctx, flags, jobRef)
			if err != nil {
				return err
			}
			if err := careWriteActiveJob(job); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Searching for: %s (job %s)\n", job.Title, job.ID)

			vars := map[string]any{
				"input": map[string]any{
					"careType": careType,
					"filters": map[string]any{
						"postalCode":      zip,
						"searchPageSize":  fetchSize,
						"searchSortOrder": sortOrder,
					},
				},
			}
			data, err := careGraphQL(ctx, flags, careQSearchProvidersOp, careQSearchProviders, vars)
			if err != nil {
				return err
			}
			var wrap struct {
				Search struct {
					Typename string `json:"__typename"`
					Message  string `json:"message"`
					Conn     struct {
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
			if wrap.Search.Message != "" && wrap.Search.Conn.TotalHits == 0 {
				return fmt.Errorf("care.com search error: %s", wrap.Search.Message)
			}

			results := make([]careCaregiverSummary, 0, pageSize)
			for _, e := range wrap.Search.Conn.Edges {
				if e.Node.Typename != "Caregiver" {
					continue
				}
				s := e.Node.summary()
				if minExp > 0 && s.YearsExperience < minExp {
					continue
				}
				if minReviews > 0 && s.TotalReviews < minReviews {
					continue
				}
				if minRate > 0 && (s.HourlyRate == 0 || s.HourlyRate < minRate) {
					continue
				}
				if maxRate > 0 && s.HourlyRate > maxRate {
					continue
				}
				results = append(results, s)
				if len(results) >= pageSize {
					break
				}
			}
			// Stable ranking: more reviews, then higher rating, then more experience.
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].TotalReviews != results[j].TotalReviews {
					return results[i].TotalReviews > results[j].TotalReviews
				}
				if results[i].AvgRating != results[j].AvgRating {
					return results[i].AvgRating > results[j].AvgRating
				}
				return results[i].YearsExperience > results[j].YearsExperience
			})

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return renderCareFindTable(cmd, results, wrap.Search.Conn.TotalHits, zip)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&zip, "zip", "", "postal code to search near (required)")
	cmd.Flags().StringVar(&jobRef, "job", "", "the job you're hiring for, by name or id (required); becomes the active job for messaging")
	cmd.Flags().StringVar(&careType, "care-type", "SITTER", "care.com careType (SITTER for babysitters/nannies)")
	cmd.Flags().StringVar(&sortOrder, "sort", "SORT_ORDER_RECOMMENDED_DESCENDING", "care.com search sort order")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum caregivers to return")
	cmd.Flags().IntVar(&minExp, "min-exp", 0, "minimum years of experience")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 0, "minimum total reviews")
	cmd.Flags().Float64Var(&minRate, "min-rate", 0, "minimum hourly rate (USD)")
	cmd.Flags().Float64Var(&maxRate, "max-rate", 0, "maximum hourly rate (USD)")
	return cmd
}

func renderCareFindTable(cmd *cobra.Command, results []careCaregiverSummary, totalHits int, zip string) error {
	w := cmd.OutOrStdout()
	if len(results) == 0 {
		fmt.Fprintf(w, "No caregivers matched near %s.\n", zip)
		return nil
	}
	fmt.Fprintf(w, "%-20s %-16s %5s %8s %8s %7s  %s\n", "NAME", "CITY", "EXP", "RATE", "REVIEWS", "RATING", "ID")
	for _, r := range results {
		rate := "-"
		if r.HourlyRate > 0 {
			rate = fmt.Sprintf("$%.0f", r.HourlyRate)
		}
		rating := "-"
		if r.AvgRating > 0 {
			rating = fmt.Sprintf("%.1f", r.AvgRating)
		}
		city := r.City
		if r.State != "" {
			city = city + ", " + r.State
		}
		fmt.Fprintf(w, "%-20s %-16s %4dy %8s %8d %7s  %s\n",
			truncate(r.Name, 20), truncate(city, 16), r.YearsExperience, rate, r.TotalReviews, rating, r.ID)
	}
	fmt.Fprintf(w, "\nShowing %d of ~%d caregivers near %s.\n", len(results), totalHits, zip)
	return nil
}
