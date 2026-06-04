// Copyright 2026 riteshtiwari and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newPersonsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "persons",
		Short: "Persons analytics and at-risk detection commands",
		Long: `Compound commands for persons analysis and churn risk detection.

For full persons CRUD, use: posthog-pp-cli projects persons`,
	}
	cmd.AddCommand(newPersonsAtRiskCmd(flags))
	return cmd
}

// newPersonsAtRiskCmd surfaces users in a cohort that are going quiet and
// recently hit errors — before they churn.
func newPersonsAtRiskCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var cohortID string
	var silentDays int
	var limit int

	cmd := &cobra.Command{
		Use:         "at-risk",
		Short:       "Surface persons going quiet who recently hit errors — before they churn",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long:        `Find persons silent for N+ days, sorted by risk score. Use in weekly retention reviews to prioritize outreach before churn.`,
		Example: `  posthog-pp-cli persons at-risk --project 12345
  posthog-pp-cli persons at-risk --project 12345 --cohort 456 --silent-days 14
  posthog-pp-cli persons at-risk --project 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if projectID == "" {
				orgData, err2 := c.Get("/api/organizations/", nil)
				if err2 == nil {
					var orgs struct {
						Results []struct {
							ID string `json:"id"`
						} `json:"results"`
					}
					if json.Unmarshal(orgData, &orgs) == nil && len(orgs.Results) > 0 {
						orgID := orgs.Results[0].ID
						projData, err3 := c.Get(fmt.Sprintf("/api/organizations/%s/projects/", orgID), nil)
						if err3 == nil {
							var projs struct {
								Results []struct {
									ID int `json:"id"`
								} `json:"results"`
							}
							if json.Unmarshal(projData, &projs) == nil && len(projs.Results) > 0 {
								projectID = strconv.Itoa(projs.Results[0].ID)
							}
						}
					}
				}
			}
			if projectID == "" {
				return fmt.Errorf("--project is required")
			}

			params := map[string]string{
				"limit": strconv.Itoa(limit),
			}
			if cohortID != "" {
				params["cohort"] = cohortID
			}

			var rawPersons []json.RawMessage
			nextURL := fmt.Sprintf("/api/projects/%s/persons/", projectID)
			pageParams := params
			for nextURL != "" && len(rawPersons) < limit {
				data, fetchErr := c.Get(nextURL, pageParams)
				if fetchErr != nil {
					if len(rawPersons) == 0 {
						return classifyAPIError(fetchErr, flags)
					}
					break
				}
				pageParams = nil
				var resp struct {
					Next    string            `json:"next"`
					Results []json.RawMessage `json:"results"`
				}
				if json.Unmarshal(data, &resp) == nil && resp.Results != nil {
					rawPersons = append(rawPersons, resp.Results...)
					nextURL = resp.Next
				} else {
					var direct []json.RawMessage
					json.Unmarshal(data, &direct) //nolint:errcheck
					rawPersons = append(rawPersons, direct...)
					break
				}
			}

			// Fetch recent error signals: query $exception events to find which distinct_ids hit errors.
			errorDistinctIDs := map[string]bool{}
			errorData, errErr := c.Get(fmt.Sprintf("/api/projects/%s/events/", projectID),
				map[string]string{"event": "$exception", "limit": "1000"})
			if errErr == nil {
				var eventsResp struct {
					Results []struct {
						DistinctID string `json:"distinct_id"`
					} `json:"results"`
				}
				if json.Unmarshal(errorData, &eventsResp) == nil {
					for _, e := range eventsResp.Results {
						if e.DistinctID != "" {
							errorDistinctIDs[e.DistinctID] = true
						}
					}
				}
			}

			silentCutoff := time.Now().AddDate(0, 0, -silentDays)

			type atRiskPerson struct {
				ID         any    `json:"id"`
				DistinctID string `json:"distinct_id"`
				Email      string `json:"email,omitempty"`
				Name       string `json:"name,omitempty"`
				LastSeen   string `json:"last_seen,omitempty"`
				DaysSilent int    `json:"days_silent"`
				HitErrors  bool   `json:"recently_hit_errors"`
				RiskScore  int    `json:"risk_score"`
			}

			var atRisk []atRiskPerson
			for _, raw := range rawPersons {
				var p struct {
					ID          any            `json:"id"`
					DistinctIDs []string       `json:"distinct_ids"`
					Properties  map[string]any `json:"properties"`
					CreatedAt   string         `json:"created_at"`
				}
				if json.Unmarshal(raw, &p) != nil {
					continue
				}

				email := ""
				name := ""
				if e, ok := p.Properties["email"]; ok {
					email = fmt.Sprintf("%v", e)
				}
				for _, k := range []string{"name", "full_name", "display_name"} {
					if v, ok := p.Properties[k]; ok {
						name = fmt.Sprintf("%v", v)
						break
					}
				}

				lastSeenStr := ""
				for _, k := range []string{"last_seen", "$last_seen", "updated_at"} {
					if v, ok := p.Properties[k]; ok {
						lastSeenStr = fmt.Sprintf("%v", v)
						break
					}
				}

				daysSilent := 0
				isSilent := false
				if lastSeenStr != "" {
					t, err := time.Parse(time.RFC3339, lastSeenStr)
					if err != nil {
						t, err = time.Parse("2006-01-02T15:04:05.999999Z", lastSeenStr)
					}
					if err == nil && t.Before(silentCutoff) {
						isSilent = true
						daysSilent = int(time.Since(t).Hours() / 24)
					}
				} else {
					// No last_seen — treat as silent since account creation.
					if p.CreatedAt != "" {
						t, err := time.Parse(time.RFC3339, p.CreatedAt)
						if err == nil && t.Before(silentCutoff) {
							isSilent = true
							daysSilent = int(time.Since(t).Hours() / 24)
						}
					}
				}

				if !isSilent {
					continue
				}

				// Check if any of this person's distinct_ids appear in recent error signals.
				hitErrors := false
				for _, did := range p.DistinctIDs {
					if errorDistinctIDs[did] {
						hitErrors = true
						break
					}
				}

				riskScore := daysSilent
				if hitErrors {
					riskScore += 50
				}

				atRisk = append(atRisk, atRiskPerson{
					ID: p.ID,
					DistinctID: func() string {
						if len(p.DistinctIDs) > 0 {
							return p.DistinctIDs[0]
						}
						return ""
					}(),
					Email:      email,
					Name:       name,
					LastSeen:   lastSeenStr,
					DaysSilent: daysSilent,
					HitErrors:  hitErrors,
					RiskScore:  riskScore,
				})
			}

			sort.Slice(atRisk, func(i, j int) bool {
				return atRisk[i].RiskScore > atRisk[j].RiskScore
			})

			type result struct {
				ProjectID    string         `json:"project_id"`
				CohortID     string         `json:"cohort_id,omitempty"`
				SilentDays   int            `json:"silent_days_threshold"`
				TotalChecked int            `json:"total_persons_checked"`
				AtRiskCount  int            `json:"at_risk_count"`
				AtRisk       []atRiskPerson `json:"at_risk_persons"`
				GeneratedAt  string         `json:"generated_at"`
			}
			out := result{
				ProjectID:    projectID,
				CohortID:     cohortID,
				SilentDays:   silentDays,
				TotalChecked: len(rawPersons),
				AtRiskCount:  len(atRisk),
				AtRisk:       atRisk,
				GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
			}
			if out.AtRisk == nil {
				out.AtRisk = []atRiskPerson{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "At-risk persons (project %s", projectID)
			if cohortID != "" {
				fmt.Fprintf(w, ", cohort %s", cohortID)
			}
			fmt.Fprintf(w, ")\n")
			fmt.Fprintf(w, "Silent threshold: %d days | Checked: %d persons\n\n", silentDays, len(rawPersons))

			if len(atRisk) == 0 {
				fmt.Fprintln(w, "No at-risk persons found.")
				return nil
			}

			fmt.Fprintf(w, "Found %d at-risk person(s):\n\n", len(atRisk))
			fmt.Fprintf(w, "  %-6s  %-8s  %-6s  %s\n", "SCORE", "DAYS_SILENT", "ERRORS", "EMAIL / ID")
			for _, p := range atRisk {
				label := p.Email
				if label == "" {
					label = fmt.Sprintf("%v", p.ID)
				}
				fmt.Fprintf(w, "  %-6d  %-8d  %-6v  %s\n", p.RiskScore, p.DaysSilent, p.HitErrors, label)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().StringVar(&cohortID, "cohort", "", "Filter to persons in this cohort ID")
	cmd.Flags().IntVar(&silentDays, "silent-days", 14, "Days without any event before treating person as at-risk, default 14")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of persons to retrieve and evaluate for risk scoring")
	return cmd
}
