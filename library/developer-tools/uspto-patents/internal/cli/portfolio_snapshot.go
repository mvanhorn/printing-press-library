package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// portfolioStats holds aggregate stats for an assignee's patent portfolio.
type portfolioStats struct {
	Assignee        string            `json:"assignee"`
	TotalCount      int               `json:"totalCount"`
	PendingCount    int               `json:"pendingCount"`
	GrantedCount    int               `json:"grantedCount"`
	AbandonedCount  int               `json:"abandonedCount"`
	OtherCount      int               `json:"otherCount"`
	FilingYearHist  map[string]int    `json:"filingYearHistogram"`
	TopArtUnits     []artUnitCount    `json:"topArtUnits"`
}

type artUnitCount struct {
	ArtUnit string `json:"artUnit"`
	Count   int    `json:"count"`
}

func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Portfolio analytics for patent assignees",
	}
	cmd.AddCommand(newPortfolioSnapshotCmd(flags))
	cmd.AddCommand(newPortfolioDiffCmd(flags))
	return cmd
}

func newPortfolioSnapshotCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "snapshot <assignee>",
		Short: "Aggregate stats for an assignee's patent portfolio",
		Long: `Searches for all applications filed by the given assignee and computes
aggregate statistics: total count, pending/granted/abandoned breakdown,
filing year histogram, and top 5 art units.`,
		Example: strings.Trim(`
  uspto-patents-pp-cli portfolio snapshot "Google LLC"
  uspto-patents-pp-cli portfolio snapshot "Apple Inc." --json
  uspto-patents-pp-cli portfolio snapshot "Microsoft" --limit 100`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			assignee := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch all applications for this assignee with pagination
			var allApps []map[string]interface{}
			offset := 0
			pageSize := 25
			if limit > 0 && limit < pageSize {
				pageSize = limit
			}

			for {
				body := map[string]interface{}{
					"q": fmt.Sprintf("applicationMetaData.firstApplicantName:\"%s\"", assignee),
					"pagination": map[string]interface{}{
						"offset": offset,
						"limit":  pageSize,
					},
				}
				data, _, err := c.Post("/api/v1/patent/applications/search", body)
				if err != nil {
					return classifyAPIError(err, flags)
				}

				apps := extractSearchApps(data)
				if len(apps) == 0 {
					break
				}
				allApps = append(allApps, apps...)

				if limit > 0 && len(allApps) >= limit {
					allApps = allApps[:limit]
					break
				}

				offset += len(apps)
				if len(apps) < pageSize {
					break // last page
				}
			}

			if len(allApps) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no applications found for assignee %q\n", assignee)
				return nil
			}

			// Compute aggregate stats
			stats := computePortfolioStats(assignee, allApps)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Portfolio Snapshot: %s\n\n", assignee)
			fmt.Fprintf(w, "  Total Applications: %d\n", stats.TotalCount)
			fmt.Fprintf(w, "  Pending:            %d\n", stats.PendingCount)
			fmt.Fprintf(w, "  Granted:            %d\n", stats.GrantedCount)
			fmt.Fprintf(w, "  Abandoned:          %d\n", stats.AbandonedCount)
			if stats.OtherCount > 0 {
				fmt.Fprintf(w, "  Other:              %d\n", stats.OtherCount)
			}

			if len(stats.TopArtUnits) > 0 {
				fmt.Fprintf(w, "\n  Top Art Units:\n")
				for _, au := range stats.TopArtUnits {
					fmt.Fprintf(w, "    %s: %d\n", au.ArtUnit, au.Count)
				}
			}

			if len(stats.FilingYearHist) > 0 {
				fmt.Fprintf(w, "\n  Filing Years:\n")
				years := make([]string, 0, len(stats.FilingYearHist))
				for y := range stats.FilingYearHist {
					years = append(years, y)
				}
				sort.Strings(years)
				for _, y := range years {
					fmt.Fprintf(w, "    %s: %d\n", y, stats.FilingYearHist[y])
				}
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of applications to fetch (0 = all)")
	return cmd
}

// extractSearchApps extracts application objects from the search response.
func extractSearchApps(data json.RawMessage) []map[string]interface{} {
	// Try direct array
	var items []map[string]interface{}
	if json.Unmarshal(data, &items) == nil {
		return items
	}

	// Try wrapped response
	var wrapper map[string]json.RawMessage
	if json.Unmarshal(data, &wrapper) == nil {
		for _, key := range []string{"patentFileWrapperDataBag", "results", "data", "items", "patentApplications", "applications"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
					return items
				}
			}
		}
	}

	return nil
}

// computePortfolioStats computes aggregate statistics from a list of applications.
func computePortfolioStats(assignee string, apps []map[string]interface{}) portfolioStats {
	stats := portfolioStats{
		Assignee:       assignee,
		TotalCount:     len(apps),
		FilingYearHist: map[string]int{},
	}

	artUnitCounts := map[string]int{}

	for _, app := range apps {
		// Status classification
		status := strings.ToLower(extractStringField(app, "applicationStatusDescriptionText", "applicationStatus", "status", "applicationStatusCategory"))
		switch {
		case strings.Contains(status, "patent") || strings.Contains(status, "granted") || strings.Contains(status, "issued"):
			stats.GrantedCount++
		case strings.Contains(status, "pending") || strings.Contains(status, "docketed") || strings.Contains(status, "examination"):
			stats.PendingCount++
		case strings.Contains(status, "abandon"):
			stats.AbandonedCount++
		default:
			stats.OtherCount++
		}

		// Filing year
		filingDate := extractStringField(app, "filingDate", "applicationFilingDate")
		if len(filingDate) >= 4 {
			year := filingDate[:4]
			stats.FilingYearHist[year]++
		}

		// Art unit
		artUnit := extractStringField(app, "groupArtUnitNumber", "artUnit", "groupArtUnit")
		if artUnit != "" {
			artUnitCounts[artUnit]++
		}
	}

	// Top 5 art units
	type auPair struct {
		unit  string
		count int
	}
	var auSlice []auPair
	for u, c := range artUnitCounts {
		auSlice = append(auSlice, auPair{u, c})
	}
	sort.Slice(auSlice, func(i, j int) bool {
		return auSlice[i].count > auSlice[j].count
	})
	topN := 5
	if len(auSlice) < topN {
		topN = len(auSlice)
	}
	for _, au := range auSlice[:topN] {
		stats.TopArtUnits = append(stats.TopArtUnits, artUnitCount{ArtUnit: au.unit, Count: au.count})
	}

	return stats
}

// extractStringField tries multiple field names and returns the first non-empty string value.
func extractStringField(obj map[string]interface{}, fields ...string) string {
	for _, f := range fields {
		if v, ok := obj[f]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	// Check nested applicationMetaData
	if meta, ok := obj["applicationMetaData"]; ok {
		if metaMap, ok := meta.(map[string]interface{}); ok {
			for _, f := range fields {
				if v, ok := metaMap[f]; ok && v != nil {
					s := fmt.Sprintf("%v", v)
					if s != "" && s != "<nil>" {
						return s
					}
				}
			}
		}
	}
	return ""
}
