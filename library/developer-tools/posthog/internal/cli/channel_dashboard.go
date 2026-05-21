// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDashboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Dashboard health and analysis commands",
		Long: `Compound commands for dashboard health checks and diagnostics.

For full dashboard CRUD, use: posthog-pp-cli projects dashboards`,
	}
	cmd.AddCommand(newDashboardHealthCmd(flags))
	return cmd
}

// newDashboardHealthCmd finds broken dashboards before a stakeholder meeting
// does — stale data, deleted cohorts, archived flags.
func newDashboardHealthCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var staleDays int
	var limit int

	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Find broken dashboards before a stakeholder meeting does",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Score dashboards for health issues: stale (not refreshed), empty (no tiles), error (tile errors), archived insight. Returns healthy/warning/broken per dashboard.`,
		Example: `  posthog-pp-cli dashboard health --project 12345
  posthog-pp-cli dashboard health --project 12345 --stale-days 3
  posthog-pp-cli dashboard health --project 12345 --json`,
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

			var rawItems []json.RawMessage
			nextURL := fmt.Sprintf("/api/projects/%s/dashboards/", projectID)
			params := map[string]string{"limit": strconv.Itoa(limit)}
			for nextURL != "" && len(rawItems) < limit {
				data, err := c.Get(nextURL, params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				params = nil // only send params on the first request
				var resp struct {
					Next    string            `json:"next"`
					Results []json.RawMessage `json:"results"`
				}
				if json.Unmarshal(data, &resp) == nil {
					rawItems = append(rawItems, resp.Results...)
					nextURL = resp.Next
				} else {
					var items []json.RawMessage
					if json.Unmarshal(data, &items) == nil {
						rawItems = append(rawItems, items...)
					}
					break
				}
			}

			staleCutoff := time.Now().AddDate(0, 0, -staleDays)

			type issue struct {
				Type    string `json:"type"`
				Detail  string `json:"detail"`
			}
			type dashHealth struct {
				ID          int     `json:"id"`
				Name        string  `json:"name"`
				IsShared    bool    `json:"is_shared"`
				TileCount   int     `json:"tile_count"`
				LastRefresh string  `json:"last_refresh,omitempty"`
				Score       string  `json:"health_score"` // healthy | warning | broken
				Issues      []issue `json:"issues"`
			}

			var results []dashHealth
			healthyCount := 0
			warningCount := 0
			brokenCount := 0

			for _, raw := range rawItems {
				var dash struct {
					ID          int            `json:"id"`
					Name        string         `json:"name"`
					IsShared    bool           `json:"is_shared"`
					Deleted     bool           `json:"deleted"`
					Tiles       []json.RawMessage `json:"tiles"`
					LastRefresh string         `json:"last_refresh"`
					CreatedAt   string         `json:"created_at"`
				}
				if json.Unmarshal(raw, &dash) != nil {
					continue
				}

				var issues []issue

				// Check: deleted/archived dashboard.
				if dash.Deleted {
					issues = append(issues, issue{Type: "archived", Detail: "dashboard is deleted/archived"})
				}

				// Check: empty dashboard.
				if len(dash.Tiles) == 0 {
					issues = append(issues, issue{Type: "empty", Detail: "dashboard has no tiles"})
				}

				// Check: stale data.
				if dash.LastRefresh != "" {
					t, err := time.Parse(time.RFC3339, dash.LastRefresh)
					if err != nil {
						t, _ = time.Parse("2006-01-02T15:04:05.999999Z", dash.LastRefresh)
					}
					if !t.IsZero() && t.Before(staleCutoff) {
						days := int(time.Since(t).Hours() / 24)
						issues = append(issues, issue{
							Type:   "stale",
							Detail: fmt.Sprintf("last refreshed %d days ago (threshold: %d days)", days, staleDays),
						})
					}
				} else if dash.CreatedAt != "" {
					// Never refreshed — check if dashboard is old.
					t, _ := time.Parse(time.RFC3339, dash.CreatedAt)
					if !t.IsZero() && t.Before(staleCutoff) {
						issues = append(issues, issue{Type: "stale", Detail: "never refreshed"})
					}
				}

				// Check tiles for errors and archived insights.
				tileErrorCount := 0
				archivedInsightCount := 0
				for _, tileRaw := range dash.Tiles {
					var tile map[string]any
					if json.Unmarshal(tileRaw, &tile) != nil {
						continue
					}
					// Check insight deletion.
					if insight, ok := tile["insight"].(map[string]any); ok {
						if del, ok := insight["deleted"].(bool); ok && del {
							archivedInsightCount++
						}
						if archived, ok := insight["archived"].(bool); ok && archived {
							archivedInsightCount++
						}
					}
					// Check for error state on tile.
					if errMsg, ok := tile["error_description"].(string); ok && errMsg != "" {
						tileErrorCount++
					}
				}
				if tileErrorCount > 0 {
					issues = append(issues, issue{
						Type:   "error",
						Detail: fmt.Sprintf("%d tile(s) have errors", tileErrorCount),
					})
				}
				if archivedInsightCount > 0 {
					issues = append(issues, issue{
						Type:   "archived_insight",
						Detail: fmt.Sprintf("%d tile(s) reference deleted/archived insights", archivedInsightCount),
					})
				}

				score := "healthy"
				if len(issues) > 0 {
					// Any error or archived makes it broken; stale/empty is a warning.
					for _, iss := range issues {
						if iss.Type == "error" || iss.Type == "archived" || iss.Type == "archived_insight" {
							score = "broken"
							break
						}
					}
					if score != "broken" {
						score = "warning"
					}
				}

				switch score {
				case "healthy":
					healthyCount++
				case "warning":
					warningCount++
				case "broken":
					brokenCount++
				}

				if issues == nil {
					issues = []issue{}
				}

				results = append(results, dashHealth{
					ID:          dash.ID,
					Name:        dash.Name,
					IsShared:    dash.IsShared,
					TileCount:   len(dash.Tiles),
					LastRefresh: dash.LastRefresh,
					Score:       score,
					Issues:      issues,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				order := map[string]int{"broken": 0, "warning": 1, "healthy": 2}
				if results[i].Score != results[j].Score {
					return order[results[i].Score] < order[results[j].Score]
				}
				return results[i].ID < results[j].ID
			})

			type summary struct {
				ProjectID    string       `json:"project_id"`
				TotalChecked int          `json:"total_checked"`
				Healthy      int          `json:"healthy"`
				Warning      int          `json:"warning"`
				Broken       int          `json:"broken"`
				Dashboards   []dashHealth `json:"dashboards"`
				GeneratedAt  string       `json:"generated_at"`
			}
			out := summary{
				ProjectID:    projectID,
				TotalChecked: len(results),
				Healthy:      healthyCount,
				Warning:      warningCount,
				Broken:       brokenCount,
				Dashboards:   results,
				GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
			}
			if out.Dashboards == nil {
				out.Dashboards = []dashHealth{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Dashboard health check (project %s)\n\n", projectID)
			fmt.Fprintf(w, "Checked %d dashboards: %d healthy, %d warning, %d broken\n\n",
				out.TotalChecked, healthyCount, warningCount, brokenCount)

			if brokenCount == 0 && warningCount == 0 {
				fmt.Fprintln(w, "All dashboards are healthy.")
				return nil
			}

			for _, d := range results {
				if d.Score == "healthy" {
					continue
				}
				fmt.Fprintf(w, "  [%s] %s (ID: %d, tiles: %d)\n", strings.ToUpper(d.Score), d.Name, d.ID, d.TileCount)
				for _, iss := range d.Issues {
					fmt.Fprintf(w, "       • %s: %s\n", iss.Type, iss.Detail)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().IntVar(&staleDays, "stale-days", 2, "Number of days without a refresh before flagging as stale (default 2)")
	cmd.Flags().IntVar(&limit, "limit", 250, "Maximum number of dashboards to include in the health check")
	return cmd
}
