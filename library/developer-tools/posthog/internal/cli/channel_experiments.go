// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newExperimentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiments",
		Short: "Experiment analysis and pre-check commands",
		Long: `Compound commands for experiment readiness and significance analysis.

For full experiment CRUD, use: posthog-pp-cli projects experiments`,
	}
	cmd.AddCommand(newExperimentsPreCheckCmd(flags))
	return cmd
}

// newExperimentsPreCheckCmd estimates if a running experiment will reach
// statistical significance within the current sprint.
func newExperimentsPreCheckCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var sprintDays int

	cmd := &cobra.Command{
		Use:         "pre-check [experiment-id]",
		Short:       "Know today whether an experiment will reach significance this sprint",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Project experiment enrollment velocity to estimate whether significance will be reached within the sprint. Returns verdict per experiment: on_track, needs_traffic, already_significant, or not_running.`,
		Example: `  posthog-pp-cli experiments pre-check --project 12345
  posthog-pp-cli experiments pre-check 789 --project 12345
  posthog-pp-cli experiments pre-check --project 12345 --sprint-days 14 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if projectID == "" {
				orgData, err2 := c.Get("/api/organizations/", nil)
				if err2 == nil {
					var orgs struct {
						Results []struct{ ID string `json:"id"` } `json:"results"`
					}
					if json.Unmarshal(orgData, &orgs) == nil && len(orgs.Results) > 0 {
						orgID := orgs.Results[0].ID
						projData, err3 := c.Get(fmt.Sprintf("/api/organizations/%s/projects/", orgID), nil)
						if err3 == nil {
							var projs struct {
								Results []struct{ ID int `json:"id"` } `json:"results"`
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

			if len(args) > 0 {
				// Fetch single experiment by ID.
				expData, err := c.Get(fmt.Sprintf("/api/projects/%s/experiments/%s/", projectID, args[0]), nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return renderExperimentsPreCheck(cmd, flags, projectID, sprintDays, []json.RawMessage{expData})
			}

			var rawItems []json.RawMessage
			nextURL := fmt.Sprintf("/api/projects/%s/experiments/", projectID)
			pageParams := map[string]string{"limit": "100"}
			for nextURL != "" {
				data, fetchErr := c.Get(nextURL, pageParams)
				if fetchErr != nil {
					if len(rawItems) == 0 {
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
					rawItems = append(rawItems, resp.Results...)
					nextURL = resp.Next
				} else {
					var direct []json.RawMessage
					json.Unmarshal(data, &direct) //nolint:errcheck
					rawItems = append(rawItems, direct...)
					break
				}
			}

			return renderExperimentsPreCheck(cmd, flags, projectID, sprintDays, rawItems)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().IntVar(&sprintDays, "sprint-days", 14, "Number of days remaining in the current sprint for projection")
	return cmd
}

func renderExperimentsPreCheck(cmd *cobra.Command, flags *rootFlags, projectID string, sprintDays int, rawItems []json.RawMessage) error {
	type expCheck struct {
		ID                 int     `json:"id"`
		Name               string  `json:"name"`
		Status             string  `json:"status"`
		StartDate          string  `json:"start_date,omitempty"`
		EndDate            string  `json:"end_date,omitempty"`
		DaysRunning        int     `json:"days_running"`
		DaysRemaining      int     `json:"days_remaining_in_sprint"`
		Participants       int     `json:"participants"`
		DailyRate          float64 `json:"daily_enrollment_rate"`
		SignificanceTarget float64 `json:"significance_target"`
		CurrentSignificance float64 `json:"current_significance,omitempty"`
		Verdict            string  `json:"verdict"` // on_track | needs_traffic | already_significant | not_running
		Recommendation     string  `json:"recommendation"`
	}

	var checks []expCheck
	now := time.Now()

	for _, raw := range rawItems {
		var exp struct {
			ID             int     `json:"id"`
			Name           string  `json:"name"`
			Status         string  `json:"status"`
			StartDate      string  `json:"start_date"`
			EndDate        string  `json:"end_date"`
			Significance   float64 `json:"significance"`
			Variants []struct{} `json:"variants"`
		}
		if json.Unmarshal(raw, &exp) != nil {
			continue
		}

		// Only analyze running experiments.
		status := strings.ToLower(exp.Status)
		if status != "running" {
			checks = append(checks, expCheck{
				ID: exp.ID, Name: exp.Name, Status: exp.Status,
				Verdict: "not_running", Recommendation: "experiment is not currently running",
			})
			continue
		}

		startDate := now.AddDate(0, 0, -7) // default if no start date
		if exp.StartDate != "" {
			if t, err := time.Parse(time.RFC3339, exp.StartDate); err == nil {
				startDate = t
			} else if t, err := time.Parse("2006-01-02", exp.StartDate); err == nil {
				startDate = t
			}
		}

		daysRunning := int(math.Max(1, now.Sub(startDate).Hours()/24))

		// Extract participant count from the raw JSON more broadly.
		var rawObj map[string]any
		json.Unmarshal(raw, &rawObj)
		participantCount := 0
		for _, k := range []string{"exposure_count", "participant_count", "sample_size"} {
			if v, ok := rawObj[k]; ok {
				participantCount = int(toFloat(v))
				break
			}
		}

		dailyRate := float64(participantCount) / float64(daysRunning)
		projectedTotal := participantCount + int(dailyRate*float64(sprintDays))

		sigTarget := exp.Significance
		if sigTarget == 0 {
			sigTarget = 0.95
		}

		// Current significance from the experiment object.
		// Use dedicated current-result fields; avoid "significance" which holds the target threshold.
		currentSig := 0.0
		isPValue := false
		for _, k := range []string{"current_significance", "significance_level"} {
			if v, ok := rawObj[k]; ok {
				currentSig = toFloat(v)
				break
			}
		}
		if currentSig == 0.0 {
			if v, ok := rawObj["p_value"]; ok {
				currentSig = toFloat(v)
				isPValue = true
			}
		}

		isSignificant := currentSig >= sigTarget
		if isPValue && currentSig > 0 {
			isSignificant = currentSig <= (1 - sigTarget)
		}

		verdict := "on_track"
		recommendation := fmt.Sprintf("projected %d total participants by sprint end", projectedTotal)

		switch {
		case isSignificant:
			verdict = "already_significant"
			recommendation = fmt.Sprintf("significance %.2f reached — ready to ship", currentSig)
		case projectedTotal < 100:
			verdict = "needs_traffic"
			recommendation = fmt.Sprintf("only ~%d participants projected by sprint end — increase traffic or extend sprint", projectedTotal)
		}

		checks = append(checks, expCheck{
			ID:                  exp.ID,
			Name:                exp.Name,
			Status:              exp.Status,
			StartDate:           exp.StartDate,
			EndDate:             exp.EndDate,
			DaysRunning:         daysRunning,
			DaysRemaining:       sprintDays,
			Participants:        participantCount,
			DailyRate:           dailyRate,
			SignificanceTarget:  sigTarget,
			CurrentSignificance: currentSig,
			Verdict:             verdict,
			Recommendation:      recommendation,
		})
	}

	sort.Slice(checks, func(i, j int) bool {
		order := map[string]int{"needs_traffic": 0, "on_track": 1, "already_significant": 2, "not_running": 3}
		return order[checks[i].Verdict] < order[checks[j].Verdict]
	})

	type result struct {
		ProjectID   string     `json:"project_id"`
		SprintDays  int        `json:"sprint_days"`
		TotalChecked int       `json:"total_experiments_checked"`
		Experiments []expCheck `json:"experiments"`
		GeneratedAt string     `json:"generated_at"`
	}
	out := result{
		ProjectID:    projectID,
		SprintDays:   sprintDays,
		TotalChecked: len(checks),
		Experiments:  checks,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if out.Experiments == nil {
		out.Experiments = []expCheck{}
	}

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, out)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Experiment pre-check (project %s, %d-day sprint)\n\n", projectID, sprintDays)

	if len(checks) == 0 {
		fmt.Fprintln(w, "No experiments found.")
		return nil
	}

	fmt.Fprintf(w, "  %-6s  %-12s  %-6s  %-8s  %s\n", "ID", "VERDICT", "DAYS", "ENROLL", "NAME")
	for _, e := range checks {
		fmt.Fprintf(w, "  %-6d  %-12s  %-6d  %-8d  %s\n",
			e.ID, e.Verdict, e.DaysRunning, e.Participants, e.Name)
		fmt.Fprintf(w, "         %s\n", e.Recommendation)
	}
	return nil
}
