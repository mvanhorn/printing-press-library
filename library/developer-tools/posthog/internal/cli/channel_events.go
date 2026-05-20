// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Event schema and property analysis commands",
		Long: `Compound commands for event property analysis and schema drift detection.

For raw event access, use: posthog-pp-cli environments events`,
	}
	cmd.AddCommand(newEventsPropertyDriftCmd(flags))
	return cmd
}

// newEventsPropertyDriftCmd catches tracking regressions — properties that
// silently disappeared from an event between two time windows.
func newEventsPropertyDriftCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var eventName string
	var baseline string
	var current string
	var limit int

	cmd := &cobra.Command{
		Use:         "property-drift <event-name>",
		Short:       "Catch properties that silently disappeared from an event between two time windows",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compare event property schemas between a baseline window and the current window. Returns dropped and new properties. Use after deploys to catch silent tracking regressions.`,
		Example: `  posthog-pp-cli events property-drift pageview --project 12345
  posthog-pp-cli events property-drift purchase --project 12345 --baseline 14d --current 1d
  posthog-pp-cli events property-drift checkout --project 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Support positional arg as fallback for backwards compatibility.
			if eventName == "" && len(args) > 0 {
				eventName = args[0]
			}
			if eventName == "" {
				return usageErr(fmt.Errorf("--event is required"))
			}

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

			// Parse time windows.
			// --baseline controls the WIDTH of the comparison window (e.g. 14d = 14-day window).
			baselineWidth := 7 * 24 * time.Hour // default 7-day baseline window
			if baseline != "" {
				t, parseErr := parseSinceDuration(baseline)
				if parseErr != nil {
					return fmt.Errorf("invalid --baseline value %q: %w", baseline, parseErr)
				}
				baselineWidth = time.Since(t)
			}
			baselineEnd := time.Now().AddDate(0, 0, -7) // baseline window ends 7 days ago
			baselineStart := baselineEnd.Add(-baselineWidth)
			currentEnd := time.Now()
			currentStart := currentEnd.AddDate(0, 0, -2) // last 2 days

			if current != "" {
				t, parseErr := parseSinceDuration(current)
				if parseErr != nil {
					return fmt.Errorf("invalid --current value %q: %w", current, parseErr)
				}
				currentStart = t
			}

			basePath := fmt.Sprintf("/api/projects/%s/events/", projectID)

			// Fetch baseline samples.
			baselineParams := map[string]string{
				"event":  eventName,
				"limit":  strconv.Itoa(limit),
				"after":  baselineStart.Format(time.RFC3339),
				"before": baselineEnd.Format(time.RFC3339),
			}
			baselineData, err := c.Get(basePath, baselineParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Fetch current samples.
			currentParams := map[string]string{
				"event":  eventName,
				"limit":  strconv.Itoa(limit),
				"after":  currentStart.Format(time.RFC3339),
				"before": currentEnd.Format(time.RFC3339),
			}
			currentData, err := c.Get(basePath, currentParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Collect property key sets.
			extractProps := func(data json.RawMessage) (map[string]int, int) {
				propCounts := map[string]int{}
				var events []struct {
					Properties map[string]json.RawMessage `json:"properties"`
				}
				if json.Unmarshal(data, &events) != nil {
					var envelope struct {
						Results []struct {
							Properties map[string]json.RawMessage `json:"properties"`
						} `json:"results"`
					}
					if json.Unmarshal(data, &envelope) == nil {
						events = envelope.Results
					}
				}
				for _, e := range events {
					for k := range e.Properties {
						propCounts[k]++
					}
				}
				return propCounts, len(events)
			}

			baseProps, baseCount := extractProps(baselineData)
			currentProps, currentCount := extractProps(currentData)

			type propDiff struct {
				Property      string `json:"property"`
				Status        string `json:"status"` // "dropped" | "new" | "stable"
				BaselineCount int    `json:"baseline_occurrences"`
				CurrentCount  int    `json:"current_occurrences"`
			}

			var diffs []propDiff
			// Find dropped properties (in baseline, not in current).
			for k, baseCount2 := range baseProps {
				curCount := currentProps[k]
				if curCount == 0 {
					diffs = append(diffs, propDiff{
						Property:      k,
						Status:        "dropped",
						BaselineCount: baseCount2,
						CurrentCount:  0,
					})
				}
			}
			// Find new properties (in current, not in baseline).
			for k, curCount2 := range currentProps {
				if baseProps[k] == 0 {
					diffs = append(diffs, propDiff{
						Property:      k,
						Status:        "new",
						BaselineCount: 0,
						CurrentCount:  curCount2,
					})
				}
			}

			sort.Slice(diffs, func(i, j int) bool {
				if diffs[i].Status != diffs[j].Status {
					return diffs[i].Status < diffs[j].Status // dropped before new
				}
				return diffs[i].Property < diffs[j].Property
			})

			droppedCount := 0
			newCount := 0
			for _, d := range diffs {
				if d.Status == "dropped" {
					droppedCount++
				} else {
					newCount++
				}
			}

			type result struct {
				ProjectID        string     `json:"project_id"`
				EventName        string     `json:"event_name"`
				BaselineSamples  int        `json:"baseline_samples"`
				CurrentSamples   int        `json:"current_samples"`
				BaselineProps    int        `json:"baseline_property_count"`
				CurrentProps     int        `json:"current_property_count"`
				DroppedCount     int        `json:"dropped_count"`
				NewCount         int        `json:"new_count"`
				Diffs            []propDiff `json:"diffs"`
				GeneratedAt      string     `json:"generated_at"`
			}
			out := result{
				ProjectID:       projectID,
				EventName:       eventName,
				BaselineSamples: baseCount,
				CurrentSamples:  currentCount,
				BaselineProps:   len(baseProps),
				CurrentProps:    len(currentProps),
				DroppedCount:    droppedCount,
				NewCount:        newCount,
				Diffs:           diffs,
				GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
			}
			if out.Diffs == nil {
				out.Diffs = []propDiff{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Property drift for event %q (project %s)\n\n", eventName, projectID)
			fmt.Fprintf(w, "Baseline: %d samples, %d unique properties\n", baseCount, len(baseProps))
			fmt.Fprintf(w, "Current:  %d samples, %d unique properties\n\n", currentCount, len(currentProps))

			if len(diffs) == 0 {
				fmt.Fprintln(w, "No property drift detected. Schema looks stable.")
				return nil
			}

			fmt.Fprintf(w, "Detected %d dropped and %d new properties:\n\n", droppedCount, newCount)
			fmt.Fprintf(w, "  %-10s  %-8s  %-8s  %s\n", "STATUS", "BASELINE", "CURRENT", "PROPERTY")
			for _, d := range diffs {
				fmt.Fprintf(w, "  %-10s  %-8d  %-8d  %s\n", d.Status, d.BaselineCount, d.CurrentCount, d.Property)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&eventName, "event", "", "Event name to analyze for property drift (e.g. pageview, purchase)")
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().StringVar(&baseline, "baseline", "14d", "Baseline comparison window (e.g. 14d, 7d)")
	cmd.Flags().StringVar(&current, "current", "2d", "Current comparison window (e.g. 2d, 1d)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of event samples to fetch per comparison window")
	// event-name accepted as positional arg (Use: "property-drift <event-name>") or via --event
	return cmd
}
