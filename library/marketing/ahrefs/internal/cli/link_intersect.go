// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const allBacklinksCompositeSelect = "url_from,domain_rating_source,first_seen,traffic_domain"

type linkIntersectCandidate struct {
	refdomain     string
	bestDR        float64
	competitors   map[string]bool
	exampleURL    string
	firstSeen     string
	trafficDomain int
}

type linkIntersectResult struct {
	Refdomain          string   `json:"refdomain"`
	DomainRatingSource float64  `json:"domain_rating_source"`
	CompetitorsLinking []string `json:"competitors_linking"`
	CompetitorsCount   int      `json:"competitors_count"`
	ExampleURL         string   `json:"example_url"`
	FirstSeen          string   `json:"first_seen,omitempty"`
	TrafficDomain      int      `json:"traffic_domain,omitempty"`
}

func newLinkIntersectCmd(flags *rootFlags) *cobra.Command {
	var flagTarget string
	var flagCompetitors []string
	var flagMinCompetitors int
	var flagMinDR float64
	var flagLimit int
	var flagMode string

	cmd := &cobra.Command{
		Use:         "link-intersect",
		Short:       "Find referring domains linking competitors but not you",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  ahrefs-pp-cli link-intersect --target bestself.co --competitor intelligentchange.com
  ahrefs-pp-cli link-intersect --target bestself.co --competitor intelligentchange.com --competitor papier.com --min-competitors 2 --min-dr 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTarget == "" && !flags.dryRun {
				return fmt.Errorf("required flag %q not set", "target")
			}
			if len(flagCompetitors) == 0 && !flags.dryRun {
				return fmt.Errorf("required flag %q not set", "competitor")
			}
			if flagMinCompetitors < 1 {
				return fmt.Errorf("--min-competitors must be at least 1")
			}
			validateCompositeMode(cmd, flagMode)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			targetRows, targetProv, err := fetchCompositeRows[backlinkCompositeRow](cmd, c, flags, "/site-explorer/all-backlinks", allBacklinksCompositeParams(flagTarget, flagMode, flagLimit))
			if err != nil {
				return classifyAPIError(err)
			}
			targetHosts := map[string]bool{}
			for _, row := range targetRows {
				host := refdomainFromURL(row.URLFrom)
				if host != "" {
					targetHosts[host] = true
				}
			}

			candidates := map[string]*linkIntersectCandidate{}
			provs := []DataProvenance{targetProv}
			for _, competitor := range flagCompetitors {
				rows, prov, err := fetchCompositeRows[backlinkCompositeRow](cmd, c, flags, "/site-explorer/all-backlinks", allBacklinksCompositeParams(competitor, flagMode, flagLimit))
				if err != nil {
					return classifyAPIError(err)
				}
				provs = append(provs, prov)
				for _, row := range rows {
					host := refdomainFromURL(row.URLFrom)
					if host == "" || targetHosts[host] {
						continue
					}
					dr := 0.0
					if row.DomainRatingSource != nil {
						dr = *row.DomainRatingSource
					}
					if dr < flagMinDR {
						continue
					}
					candidate := candidates[host]
					if candidate == nil {
						candidate = &linkIntersectCandidate{
							refdomain:     host,
							competitors:   map[string]bool{},
							exampleURL:    row.URLFrom,
							firstSeen:     row.FirstSeen,
							trafficDomain: row.TrafficDomain,
						}
						candidates[host] = candidate
					}
					candidate.competitors[competitor] = true
					if dr > candidate.bestDR {
						candidate.bestDR = dr
						candidate.exampleURL = row.URLFrom
						candidate.firstSeen = row.FirstSeen
						candidate.trafficDomain = row.TrafficDomain
					}
				}
			}
			if flags.dryRun {
				return nil
			}

			results := make([]linkIntersectResult, 0, len(candidates))
			for _, candidate := range candidates {
				if len(candidate.competitors) < flagMinCompetitors {
					continue
				}
				competitors := make([]string, 0, len(candidate.competitors))
				for competitor := range candidate.competitors {
					competitors = append(competitors, competitor)
				}
				sort.Strings(competitors)
				results = append(results, linkIntersectResult{
					Refdomain:          candidate.refdomain,
					DomainRatingSource: candidate.bestDR,
					CompetitorsLinking: competitors,
					CompetitorsCount:   len(competitors),
					ExampleURL:         candidate.exampleURL,
					FirstSeen:          candidate.firstSeen,
					TrafficDomain:      candidate.trafficDomain,
				})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].DomainRatingSource == results[j].DomainRatingSource {
					return results[i].CompetitorsCount > results[j].CompetitorsCount
				}
				return results[i].DomainRatingSource > results[j].DomainRatingSource
			})
			results = limitCompositeRows(results, flagLimit)
			return printCompositeOutputWithCompact(cmd, results, compactLinkIntersectResults(results), len(results), mergeCompositeProvenance(provs...), flags)
		},
	}
	cmd.Flags().StringVar(&flagTarget, "target", "", "Your target domain or URL.")
	cmd.Flags().StringArrayVar(&flagCompetitors, "competitor", nil, "Competitor domain or URL. Repeat for multiple competitors.")
	cmd.Flags().IntVar(&flagMinCompetitors, "min-competitors", 1, "Minimum number of competitors a referring domain must link to.")
	cmd.Flags().Float64Var(&flagMinDR, "min-dr", 0, "Minimum source domain rating.")
	cmd.Flags().IntVar(&flagLimit, "limit", 1000, "Maximum rows to request per target and return after sorting.")
	cmd.Flags().StringVar(&flagMode, "mode", "subdomains", "The scope of the search based on the target you entered. (one of: exact, prefix, domain, subdomains)")
	return cmd
}

func allBacklinksCompositeParams(target, mode string, limit int) map[string]string {
	params := map[string]string{
		"select":      allBacklinksCompositeSelect,
		"target":      target,
		"mode":        mode,
		"aggregation": "1_per_domain",
	}
	if limit != 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	return params
}

func refdomainFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + raw)
		if err != nil {
			return ""
		}
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}
	return strings.TrimPrefix(host, "www.")
}

func compactLinkIntersectResults(rows []linkIntersectResult) []map[string]any {
	compact := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		compact = append(compact, map[string]any{
			"refdomain":            row.Refdomain,
			"domain_rating_source": row.DomainRatingSource,
			"competitors_count":    row.CompetitorsCount,
			"example_url":          row.ExampleURL,
		})
	}
	return compact
}
