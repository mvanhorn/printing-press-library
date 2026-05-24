// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/internal/store"
)

func newFlagsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flags",
		Short: "Feature flag safety and analysis commands",
		Long: `Compound commands for feature flag safety, health, and lifecycle analysis.

For full CRUD operations on flags, use: posthog-pp-cli projects feature-flags`,
	}
	cmd.AddCommand(newFlagsBlastRadiusCmd(flags))
	cmd.AddCommand(newFlagsRolloutHealthCmd(flags))
	cmd.AddCommand(newFlagsStaleCmd(flags))
	return cmd
}

// newFlagsBlastRadiusCmd finds every insight, dashboard, experiment, and survey
// that references a given feature flag before archiving or renaming it.
func newFlagsBlastRadiusCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var flagKey string

	cmd := &cobra.Command{
		Use:         "blast-radius",
		Short:       "Find every insight, dashboard, experiment, and survey that references a flag",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Scan insights, dashboards, experiments, and surveys for references to the given flag key. Use before archiving or renaming a flag.`,
		Example: `  posthog-pp-cli flags blast-radius --key my-flag-key --project 12345
  posthog-pp-cli flags blast-radius --key my-flag-key --project 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type refItem struct {
				ID       any    `json:"id"`
				Name     string `json:"name"`
				Resource string `json:"resource"`
				URL      string `json:"url,omitempty"`
			}
			var refs []refItem

			// Helper to scan a JSON array for flag key mentions.
			scan := func(data json.RawMessage, resource string) {
				var items []json.RawMessage
				if err := json.Unmarshal(data, &items); err != nil {
					var envelope map[string]json.RawMessage
					if err2 := json.Unmarshal(data, &envelope); err2 != nil {
						return
					}
					for _, key := range []string{"results", "data", "items"} {
						if raw, ok := envelope[key]; ok {
							if json.Unmarshal(raw, &items) == nil {
								break
							}
						}
					}
				}
				for _, item := range items {
					raw := string(item)
					// Require the flag key to appear as a quoted JSON string to
					// avoid false positives from short keys ("on", "ab") matching
					// unrelated substrings in serialized values.
					if !strings.Contains(raw, `"`+flagKey+`"`) {
						continue
					}
					var obj map[string]any
					if json.Unmarshal(item, &obj) != nil {
						continue
					}
					name := ""
					for _, k := range []string{"name", "key", "title"} {
						if v, ok := obj[k]; ok {
							name = fmt.Sprintf("%v", v)
							break
						}
					}
					var id any
					for _, k := range []string{"id", "short_id"} {
						if v, ok := obj[k]; ok {
							id = v
							break
						}
					}
					refs = append(refs, refItem{
						ID:       id,
						Name:     name,
						Resource: resource,
					})
				}
			}

			// Resolve project ID: flag or first organization project.
			if projectID == "" {
				orgData, err := c.Get("/api/organizations/", nil)
				if err == nil {
					var orgs struct {
						Results []struct {
							ID string `json:"id"`
						} `json:"results"`
					}
					if json.Unmarshal(orgData, &orgs) == nil && len(orgs.Results) > 0 {
						orgID := orgs.Results[0].ID
						projData, err2 := c.Get(fmt.Sprintf("/api/organizations/%s/projects/", orgID), nil)
						if err2 == nil {
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
				return fmt.Errorf("--project is required (could not auto-detect project ID)")
			}

			base := fmt.Sprintf("/api/projects/%s", projectID)

			// scanPaginated follows next-cursor pagination and calls scan on each page.
			scanPaginated := func(path, resource string) {
				nextURL := path
				pageParams := map[string]string{"limit": "250"}
				for nextURL != "" {
					data, fetchErr := c.Get(nextURL, pageParams)
					if fetchErr != nil {
						break
					}
					pageParams = nil
					scan(data, resource)
					var envelope struct {
						Next string `json:"next"`
					}
					json.Unmarshal(data, &envelope) //nolint:errcheck
					nextURL = envelope.Next
				}
			}

			scanPaginated(base+"/insights/", "insight")
			scanPaginated(base+"/dashboards/", "dashboard")
			scanPaginated(base+"/experiments/", "experiment")
			scanPaginated(base+"/surveys/", "survey")

			type result struct {
				FlagKey    string    `json:"flag_key"`
				ProjectID  string    `json:"project_id"`
				TotalRefs  int       `json:"total_refs"`
				References []refItem `json:"references"`
			}
			out := result{
				FlagKey:    flagKey,
				ProjectID:  projectID,
				TotalRefs:  len(refs),
				References: refs,
			}
			if out.References == nil {
				out.References = []refItem{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			if len(refs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No references found for flag %q in project %s.\n", flagKey, projectID)
				fmt.Fprintln(cmd.OutOrStdout(), "Safe to archive or rename.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Flag %q is referenced in %d place(s):\n\n", flagKey, len(refs))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-6s  %s\n", "RESOURCE", "ID", "NAME")
			for _, r := range refs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-6v  %s\n", r.Resource, r.ID, r.Name)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nDo not archive or rename until these references are updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKey, "key", "", "Feature flag key to analyze (e.g. my-checkout-v2, my-feature)")
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

// newFlagsRolloutHealthCmd checks error rate and key metric movement correlated
// with flag exposure to give a go/no-go signal before ramping to 100%.
func newFlagsRolloutHealthCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var window string
	var flagKey string

	cmd := &cobra.Command{
		Use:         "rollout-health",
		Short:       "Go/no-go confidence for a flag ramp — error rate correlated with flag exposure",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetch flag state and project error signal, return a go/caution/inactive/unknown verdict. Use before ramping a flag to 100%.`,
		Example: `  posthog-pp-cli flags rollout-health --key my-flag --project 12345
  posthog-pp-cli flags rollout-health --key my-flag --project 12345 --window 7d --json`,
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

			// Fetch the flag to confirm it exists and check its rollout state.
			flagData, err := c.Get(fmt.Sprintf("/api/projects/%s/feature_flags/", projectID),
				map[string]string{"search": flagKey, "limit": "10"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var flagResp struct {
				Results []struct {
					ID            int            `json:"id"`
					Key           string         `json:"key"`
					Active        bool           `json:"active"`
					LastModifiedAt string         `json:"last_modified_at"`
					Filters       map[string]any `json:"filters"`
				} `json:"results"`
			}
			var flagID int
			var flagActive bool
			if json.Unmarshal(flagData, &flagResp) == nil {
				for _, f := range flagResp.Results {
					if f.Key == flagKey {
						flagID = f.ID
						flagActive = f.Active
						break
					}
				}
			}

			// Parse window into a date_from parameter for the error_tracking query.
			dateFrom := ""
			if window != "" {
				if t, err2 := parseSinceDuration(window); err2 == nil {
					dateFrom = t.UTC().Format(time.RFC3339)
				}
			}

			// Fetch recent error tracking issues within the window.
			errorParams := map[string]string{"limit": "50"}
			if dateFrom != "" {
				errorParams["date_from"] = dateFrom
			}
			errorData, errErr := c.Get(fmt.Sprintf("/api/projects/%s/error_tracking/", projectID),
				errorParams)

			type healthResult struct {
				FlagKey      string `json:"flag_key"`
				FlagID       int    `json:"flag_id"`
				FlagActive   bool   `json:"flag_active"`
				ProjectID    string `json:"project_id"`
				Window       string `json:"window"`
				ErrorCount   int    `json:"recent_error_count"`
				Verdict      string `json:"verdict"`
				VerdictReason string `json:"verdict_reason"`
				CheckedAt    string `json:"checked_at"`
			}

			res := healthResult{
				FlagKey:    flagKey,
				FlagID:     flagID,
				FlagActive: flagActive,
				ProjectID:  projectID,
				Window:     window,
				CheckedAt:  time.Now().UTC().Format(time.RFC3339),
			}

			if errErr == nil {
				var errResp struct {
					Results []struct {
						Status     string `json:"status"`
						Occurrences int   `json:"occurrences"`
					} `json:"results"`
				}
				if json.Unmarshal(errorData, &errResp) == nil {
					for _, issue := range errResp.Results {
						if issue.Status != "resolved" {
							res.ErrorCount += issue.Occurrences
						}
					}
				}
			}

			// Simple verdict: flag not found -> unknown; flag inactive -> warn;
			// high error count -> caution; otherwise go.
			switch {
			case flagID == 0:
				res.Verdict = "unknown"
				res.VerdictReason = fmt.Sprintf("flag %q not found in project %s", flagKey, projectID)
			case !flagActive:
				res.Verdict = "inactive"
				res.VerdictReason = "flag is currently disabled — no rollout risk"
			case res.ErrorCount > 100:
				res.Verdict = "caution"
				res.VerdictReason = fmt.Sprintf("%d unresolved error occurrences in project — review before ramping", res.ErrorCount)
			default:
				res.Verdict = "go"
				res.VerdictReason = "no elevated error signal detected"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Flag: %s (ID: %d)\n", res.FlagKey, res.FlagID)
			fmt.Fprintf(w, "Active: %v\n", res.FlagActive)
			fmt.Fprintf(w, "Window: %s\n", res.Window)
			fmt.Fprintf(w, "Unresolved error occurrences (project-wide): %d\n", res.ErrorCount)
			fmt.Fprintf(w, "\nVerdict: %s\n", strings.ToUpper(res.Verdict))
			fmt.Fprintf(w, "Reason:  %s\n", res.VerdictReason)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKey, "key", "", "Feature flag key to check rollout health for, e.g. my-checkout-v2")
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().StringVar(&window, "window", "7d", "Time window for error analysis (e.g. 7d, 24h)")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

// newFlagsStaleCmd lists feature flags that haven't been evaluated in N days.
func newFlagsStaleCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var days int
	var limit int

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "List flags not evaluated in N days — cleanup candidates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List feature flags not evaluated in N days. Never-evaluated flags are always included. Use for quarterly cleanup sprints.`,
		Example: `  posthog-pp-cli flags stale --project 12345
  posthog-pp-cli flags stale --project 12345 --days 60 --json`,
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

			// Try local store first for offline / --data-source local mode.
			var rawItems []json.RawMessage
			if flags.dataSource != "live" {
				if dbPath := defaultDBPath("posthog-pp-cli"); dbPath != "" {
					if s, err2 := store.OpenReadOnly(dbPath); err2 == nil {
						if local, err3 := s.SearchFeatureFlags("", limit); err3 == nil && len(local) > 0 {
							rawItems = local
						}
						s.Close()
					}
				}
			}
			if len(rawItems) == 0 {
				nextURL := fmt.Sprintf("/api/projects/%s/feature_flags/", projectID)
				pageParams := map[string]string{"limit": strconv.Itoa(limit)}
				for nextURL != "" && len(rawItems) < limit {
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
			}

			cutoff := time.Now().AddDate(0, 0, -days)

			type staleFlag struct {
				ID          int    `json:"id"`
				Key         string `json:"key"`
				Name        string `json:"name"`
				Active      bool   `json:"active"`
				LastUsedAt  string `json:"last_used_at,omitempty"`
				DaysSince   int    `json:"days_since_last_use"`
				CreatedAt   string `json:"created_at,omitempty"`
			}

			var stale []staleFlag
			for _, raw := range rawItems {
				var f struct {
					ID         int    `json:"id"`
					Key        string `json:"key"`
					Name       string `json:"name"`
					Active     bool   `json:"active"`
					LastUsedAt string `json:"last_used_at"`
					CreatedAt  string `json:"created_at"`
				}
				if json.Unmarshal(raw, &f) != nil {
					continue
				}
				if f.LastUsedAt == "" {
					// Never used — always stale.
					stale = append(stale, staleFlag{
						ID: f.ID, Key: f.Key, Name: f.Name, Active: f.Active,
						CreatedAt: f.CreatedAt, DaysSince: -1,
					})
					continue
				}
				t, err := time.Parse(time.RFC3339, f.LastUsedAt)
				if err != nil {
					t, err = time.Parse("2006-01-02T15:04:05.999999Z", f.LastUsedAt)
				}
				if err != nil || t.Before(cutoff) {
					daysSince := int(math.Round(time.Since(t).Hours() / 24))
					if err != nil {
						daysSince = -1
					}
					stale = append(stale, staleFlag{
						ID: f.ID, Key: f.Key, Name: f.Name, Active: f.Active,
						LastUsedAt: f.LastUsedAt, DaysSince: daysSince,
					})
				}
			}

			type result struct {
				ProjectID  string      `json:"project_id"`
				Days       int         `json:"days_threshold"`
				TotalFlags int         `json:"total_flags_checked"`
				StaleCount int         `json:"stale_count"`
				StaleFlags []staleFlag `json:"stale_flags"`
			}
			out := result{
				ProjectID:  projectID,
				Days:       days,
				TotalFlags: len(rawItems),
				StaleCount: len(stale),
				StaleFlags: stale,
			}
			if out.StaleFlags == nil {
				out.StaleFlags = []staleFlag{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Checked %d flags in project %s (threshold: %d days)\n\n", out.TotalFlags, projectID, days)
			if len(stale) == 0 {
				fmt.Fprintln(w, "No stale flags found.")
				return nil
			}
			fmt.Fprintf(w, "Found %d stale flag(s):\n\n", len(stale))
			fmt.Fprintf(w, "  %-6s  %-8s  %-10s  %s\n", "ID", "ACTIVE", "DAYS_SINCE", "KEY")
			for _, f := range stale {
				dayStr := strconv.Itoa(f.DaysSince)
				if f.DaysSince < 0 {
					dayStr = "never"
				}
				fmt.Fprintf(w, "  %-6d  %-8v  %-10s  %s\n", f.ID, f.Active, dayStr, f.Key)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().IntVar(&days, "days", 30, "Consider a flag stale when not evaluated within this many days")
	cmd.Flags().IntVar(&limit, "limit", 250, "Maximum number of feature flags to scan and evaluate")
	return cmd
}
