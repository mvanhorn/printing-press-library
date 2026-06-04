// Copyright 2026 riteshtiwari and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

// RegisterNovelTools registers curated typed MCP tools for the 8 novel
// posthog-pp-cli commands. These are registered in addition to the
// cobratree shell-out tools; agents should prefer these typed tools because
// they carry proper input schemas and concise descriptions.
//
// Naming convention: "posthog_<group>_<subcommand>" — distinct from the
// endpoint-mirror tools (which use snake_case resource paths) so agents
// can tell them apart.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterNovelTools(s *server.MCPServer) {
	registerFlagsBlastRadius(s)
	registerFlagsRolloutHealth(s)
	registerFlagsStale(s)
	registerDashboardHealth(s)
	registerExperimentsPreCheck(s)
	registerPersonsAtRisk(s)
	registerEventsPropertyDrift(s)
	registerLLMCostAttribution(s)
}

// ── flags blast-radius ────────────────────────────────────────────────────────

func registerFlagsBlastRadius(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_flags_blast-radius",
			mcplib.WithDescription("Find every insight, dashboard, experiment, and survey that references a feature flag. Use before archiving or renaming a flag to prevent breaking downstream analytics."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("flag_key", mcplib.Required(), mcplib.Description("Feature flag key to scan for (e.g. redesigned-topology).")),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
		),
		handleFlagsBlastRadius,
	)
}

func handleFlagsBlastRadius(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	flagKey, _ := args["flag_key"].(string)
	projectID, _ := args["project_id"].(string)
	if flagKey == "" || projectID == "" {
		return mcplib.NewToolResultError("flag_key and project_id are required"), nil
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	type refItem struct {
		ID       any    `json:"id"`
		Name     string `json:"name"`
		Resource string `json:"resource"`
	}
	var refs []refItem

	scan := func(data json.RawMessage, resource string) {
		var items []json.RawMessage
		if json.Unmarshal(data, &items) != nil {
			var envelope map[string]json.RawMessage
			if json.Unmarshal(data, &envelope) != nil {
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
			if !strings.Contains(string(item), flagKey) {
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
			refs = append(refs, refItem{ID: id, Name: name, Resource: resource})
		}
	}

	base := "/api/projects/" + projectID
	for _, resource := range []string{"insights", "dashboards", "experiments", "surveys"} {
		if data, err := c.Get(base+"/"+resource+"/", map[string]string{"limit": "250"}); err == nil {
			scan(data, strings.TrimSuffix(resource, "s"))
		}
	}

	if refs == nil {
		refs = []refItem{}
	}
	result := map[string]any{
		"flag_key":   flagKey,
		"project_id": projectID,
		"total_refs": len(refs),
		"references": refs,
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── flags rollout-health ──────────────────────────────────────────────────────

func registerFlagsRolloutHealth(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_flags_rollout-health",
			mcplib.WithDescription("Go/no-go confidence signal for a flag ramp. Returns verdict (go/caution/inactive/unknown) based on flag state and project error signal. Use before ramping to 100%."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("flag_key", mcplib.Required(), mcplib.Description("Feature flag key.")),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
		),
		handleFlagsRolloutHealth,
	)
}

func handleFlagsRolloutHealth(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	flagKey, _ := args["flag_key"].(string)
	projectID, _ := args["project_id"].(string)
	if flagKey == "" || projectID == "" {
		return mcplib.NewToolResultError("flag_key and project_id are required"), nil
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	flagData, err := c.Get(fmt.Sprintf("/api/projects/%s/feature_flags/", projectID),
		map[string]string{"search": flagKey, "limit": "10"})
	if err != nil {
		return mcplib.NewToolResultError("fetching flag: " + err.Error()), nil
	}

	var flagResp struct {
		Results []struct {
			ID     int    `json:"id"`
			Key    string `json:"key"`
			Active bool   `json:"active"`
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

	errorCount := 0
	if errData, err2 := c.Get(fmt.Sprintf("/api/projects/%s/error_tracking/", projectID),
		map[string]string{"limit": "50"}); err2 == nil {
		var errResp struct {
			Results []struct {
				Status      string `json:"status"`
				Occurrences int    `json:"occurrences"`
			} `json:"results"`
		}
		if json.Unmarshal(errData, &errResp) == nil {
			for _, issue := range errResp.Results {
				if issue.Status != "resolved" {
					errorCount += issue.Occurrences
				}
			}
		}
	}

	verdict, reason := "go", "no elevated error signal detected"
	switch {
	case flagID == 0:
		verdict, reason = "unknown", fmt.Sprintf("flag %q not found", flagKey)
	case !flagActive:
		verdict, reason = "inactive", "flag is disabled"
	case errorCount > 100:
		verdict, reason = "caution", fmt.Sprintf("%d unresolved error occurrences — review before ramping", errorCount)
	}

	result := map[string]any{
		"flag_key":           flagKey,
		"flag_id":            flagID,
		"flag_active":        flagActive,
		"project_id":         projectID,
		"recent_error_count": errorCount,
		"verdict":            verdict,
		"verdict_reason":     reason,
		"checked_at":         time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── flags stale ───────────────────────────────────────────────────────────────

func registerFlagsStale(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_flags_stale",
			mcplib.WithDescription("List feature flags not evaluated in N days. Returns id, key, active, days_since_last_use. Use for quarterly flag cleanup sprints."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
			mcplib.WithNumber("days", mcplib.Description("Days threshold (default 30). Flags not used in this many days are stale.")),
		),
		handleFlagsStale,
	)
}

func handleFlagsStale(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	projectID, _ := args["project_id"].(string)
	if projectID == "" {
		return mcplib.NewToolResultError("project_id is required"), nil
	}
	days := 30
	if d, ok := args["days"].(float64); ok && d > 0 {
		days = int(d)
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	data, err := c.Get(fmt.Sprintf("/api/projects/%s/feature_flags/", projectID),
		map[string]string{"limit": "250"})
	if err != nil {
		return mcplib.NewToolResultError("fetching flags: " + err.Error()), nil
	}

	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	var rawItems []json.RawMessage
	if json.Unmarshal(data, &resp) == nil {
		rawItems = resp.Results
	} else {
		json.Unmarshal(data, &rawItems)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	type staleFlag struct {
		ID         int    `json:"id"`
		Key        string `json:"key"`
		Active     bool   `json:"active"`
		LastUsedAt string `json:"last_used_at,omitempty"`
		DaysSince  int    `json:"days_since_last_use"`
	}
	var stale []staleFlag
	for _, raw := range rawItems {
		var f struct {
			ID         int    `json:"id"`
			Key        string `json:"key"`
			Active     bool   `json:"active"`
			LastUsedAt string `json:"last_used_at"`
		}
		if json.Unmarshal(raw, &f) != nil {
			continue
		}
		if f.LastUsedAt == "" {
			stale = append(stale, staleFlag{ID: f.ID, Key: f.Key, Active: f.Active, DaysSince: -1})
			continue
		}
		t, err := time.Parse(time.RFC3339, f.LastUsedAt)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05.999999Z", f.LastUsedAt)
		}
		if err != nil || t.Before(cutoff) {
			ds := int(math.Round(time.Since(t).Hours() / 24))
			if err != nil {
				ds = -1
			}
			stale = append(stale, staleFlag{ID: f.ID, Key: f.Key, Active: f.Active, LastUsedAt: f.LastUsedAt, DaysSince: ds})
		}
	}
	if stale == nil {
		stale = []staleFlag{}
	}
	result := map[string]any{
		"project_id":          projectID,
		"days_threshold":      days,
		"total_flags_checked": len(rawItems),
		"stale_count":         len(stale),
		"stale_flags":         stale,
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── dashboard health ──────────────────────────────────────────────────────────

func registerDashboardHealth(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_dashboard_health",
			mcplib.WithDescription("Score each dashboard for health issues: stale (not refreshed in N days), empty (no tiles), error (tile errors), archived insight. Returns per-dashboard verdict: healthy/warning/broken."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
			mcplib.WithNumber("stale_days", mcplib.Description("Days without refresh to flag as stale (default 2).")),
		),
		handleDashboardHealth,
	)
}

func handleDashboardHealth(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	projectID, _ := args["project_id"].(string)
	if projectID == "" {
		return mcplib.NewToolResultError("project_id is required"), nil
	}
	staleDays := 2
	if d, ok := args["stale_days"].(float64); ok && d > 0 {
		staleDays = int(d)
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	data, err := c.Get(fmt.Sprintf("/api/projects/%s/dashboards/", projectID),
		map[string]string{"limit": "250"})
	if err != nil {
		return mcplib.NewToolResultError("fetching dashboards: " + err.Error()), nil
	}

	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	var rawItems []json.RawMessage
	if json.Unmarshal(data, &resp) == nil {
		rawItems = resp.Results
	} else {
		json.Unmarshal(data, &rawItems)
	}

	staleCutoff := time.Now().AddDate(0, 0, -staleDays)

	type issue struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	type dashHealth struct {
		ID     int     `json:"id"`
		Name   string  `json:"name"`
		Score  string  `json:"health_score"`
		Issues []issue `json:"issues"`
	}

	var results []dashHealth
	healthy, warning, broken := 0, 0, 0

	for _, raw := range rawItems {
		var dash struct {
			ID          int               `json:"id"`
			Name        string            `json:"name"`
			Deleted     bool              `json:"deleted"`
			Tiles       []json.RawMessage `json:"tiles"`
			LastRefresh string            `json:"last_refresh"`
			CreatedAt   string            `json:"created_at"`
		}
		if json.Unmarshal(raw, &dash) != nil {
			continue
		}
		var issues []issue
		if dash.Deleted {
			issues = append(issues, issue{"archived", "dashboard is deleted"})
		}
		if len(dash.Tiles) == 0 {
			issues = append(issues, issue{"empty", "no tiles"})
		}
		if dash.LastRefresh != "" {
			if t, err := time.Parse(time.RFC3339, dash.LastRefresh); err == nil && t.Before(staleCutoff) {
				issues = append(issues, issue{"stale", fmt.Sprintf("last refreshed %d days ago", int(time.Since(t).Hours()/24))})
			}
		} else {
			issues = append(issues, issue{"stale", "never refreshed"})
		}
		for _, tileRaw := range dash.Tiles {
			var tile map[string]any
			if json.Unmarshal(tileRaw, &tile) != nil {
				continue
			}
			if insight, ok := tile["insight"].(map[string]any); ok {
				if del, _ := insight["deleted"].(bool); del {
					issues = append(issues, issue{"archived_insight", "tile references deleted insight"})
				}
			}
			if errMsg, ok := tile["error_description"].(string); ok && errMsg != "" {
				issues = append(issues, issue{"error", errMsg})
			}
		}
		score := "healthy"
		for _, iss := range issues {
			if iss.Type == "error" || iss.Type == "archived" || iss.Type == "archived_insight" {
				score = "broken"
				break
			}
		}
		if score == "healthy" && len(issues) > 0 {
			score = "warning"
		}
		switch score {
		case "healthy":
			healthy++
		case "warning":
			warning++
		case "broken":
			broken++
		}
		if issues == nil {
			issues = []issue{}
		}
		results = append(results, dashHealth{ID: dash.ID, Name: dash.Name, Score: score, Issues: issues})
	}

	sort.Slice(results, func(i, j int) bool {
		order := map[string]int{"broken": 0, "warning": 1, "healthy": 2}
		return order[results[i].Score] < order[results[j].Score]
	})
	if results == nil {
		results = []dashHealth{}
	}

	result := map[string]any{
		"project_id":    projectID,
		"total_checked": len(results),
		"healthy":       healthy,
		"warning":       warning,
		"broken":        broken,
		"dashboards":    results,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── experiments pre-check ─────────────────────────────────────────────────────

func registerExperimentsPreCheck(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_experiments_pre-check",
			mcplib.WithDescription("Project whether running experiments will reach statistical significance within the sprint. Returns verdict per experiment: on_track, needs_traffic, already_significant, or not_running."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
			mcplib.WithNumber("sprint_days", mcplib.Description("Days remaining in sprint (default 14).")),
		),
		handleExperimentsPreCheck,
	)
}

func handleExperimentsPreCheck(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	projectID, _ := args["project_id"].(string)
	if projectID == "" {
		return mcplib.NewToolResultError("project_id is required"), nil
	}
	sprintDays := 14
	if d, ok := args["sprint_days"].(float64); ok && d > 0 {
		sprintDays = int(d)
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	data, err := c.Get(fmt.Sprintf("/api/projects/%s/experiments/", projectID),
		map[string]string{"limit": "100"})
	if err != nil {
		return mcplib.NewToolResultError("fetching experiments: " + err.Error()), nil
	}

	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	var rawItems []json.RawMessage
	if json.Unmarshal(data, &resp) == nil {
		rawItems = resp.Results
	} else {
		json.Unmarshal(data, &rawItems)
	}

	type expCheck struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		Status         string `json:"status"`
		DaysRunning    int    `json:"days_running"`
		Participants   int    `json:"participants"`
		Verdict        string `json:"verdict"`
		Recommendation string `json:"recommendation"`
	}

	var checks []expCheck
	now := time.Now()
	for _, raw := range rawItems {
		var exp map[string]any
		if json.Unmarshal(raw, &exp) != nil {
			continue
		}
		id := int(toFloat64(exp["id"]))
		name, _ := exp["name"].(string)
		status, _ := exp["status"].(string)

		if strings.ToLower(status) != "running" && status != "" {
			checks = append(checks, expCheck{ID: id, Name: name, Status: status, Verdict: "not_running", Recommendation: "not running"})
			continue
		}

		startDate := now.AddDate(0, 0, -7)
		if sd, ok := exp["start_date"].(string); ok && sd != "" {
			if t, err := time.Parse(time.RFC3339, sd); err == nil {
				startDate = t
			}
		}
		daysRunning := int(math.Max(1, now.Sub(startDate).Hours()/24))

		participants := 0
		for _, k := range []string{"exposure_count", "participant_count", "sample_size"} {
			if v, ok := exp[k]; ok {
				participants = int(toFloat64(v))
				break
			}
		}

		dailyRate := float64(participants) / float64(daysRunning)
		projected := participants + int(dailyRate*float64(sprintDays))

		currentSig := 0.0
		if v, ok := exp["significance"]; ok {
			currentSig = toFloat64(v)
		}
		sigTarget := 0.95

		verdict := "on_track"
		recommendation := fmt.Sprintf("~%d participants projected by sprint end", projected)
		switch {
		case currentSig >= sigTarget:
			verdict, recommendation = "already_significant", fmt.Sprintf("significance %.2f reached — ready to ship", currentSig)
		case projected < 100:
			verdict, recommendation = "needs_traffic", fmt.Sprintf("only ~%d participants projected — increase traffic or extend sprint", projected)
		}

		checks = append(checks, expCheck{
			ID: id, Name: name, Status: status,
			DaysRunning: daysRunning, Participants: participants,
			Verdict: verdict, Recommendation: recommendation,
		})
	}

	sort.Slice(checks, func(i, j int) bool {
		order := map[string]int{"needs_traffic": 0, "on_track": 1, "already_significant": 2, "not_running": 3}
		return order[checks[i].Verdict] < order[checks[j].Verdict]
	})
	if checks == nil {
		checks = []expCheck{}
	}

	result := map[string]any{
		"project_id":                projectID,
		"sprint_days":               sprintDays,
		"total_experiments_checked": len(checks),
		"experiments":               checks,
		"generated_at":              now.UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── persons at-risk ───────────────────────────────────────────────────────────

func registerPersonsAtRisk(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_persons_at-risk",
			mcplib.WithDescription("Surface persons who are going quiet (no recent activity) and may churn. Returns persons sorted by risk_score (days_silent + error_signal). Use in weekly retention reviews."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
			mcplib.WithNumber("silent_days", mcplib.Description("Days without activity to consider a person silent (default 14).")),
			mcplib.WithString("cohort_id", mcplib.Description("Optional cohort ID to restrict the scan to.")),
		),
		handlePersonsAtRisk,
	)
}

func handlePersonsAtRisk(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	projectID, _ := args["project_id"].(string)
	if projectID == "" {
		return mcplib.NewToolResultError("project_id is required"), nil
	}
	silentDays := 14
	if d, ok := args["silent_days"].(float64); ok && d > 0 {
		silentDays = int(d)
	}
	cohortID, _ := args["cohort_id"].(string)

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	params := map[string]string{"limit": "100"}
	if cohortID != "" {
		params["cohort"] = cohortID
	}
	data, err := c.Get(fmt.Sprintf("/api/projects/%s/persons/", projectID), params)
	if err != nil {
		return mcplib.NewToolResultError("fetching persons: " + err.Error()), nil
	}

	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	var rawPersons []json.RawMessage
	if json.Unmarshal(data, &resp) == nil {
		rawPersons = resp.Results
	} else {
		json.Unmarshal(data, &rawPersons)
	}

	silentCutoff := time.Now().AddDate(0, 0, -silentDays)

	type atRiskPerson struct {
		ID         any    `json:"id"`
		DistinctID string `json:"distinct_id"`
		Email      string `json:"email,omitempty"`
		DaysSilent int    `json:"days_silent"`
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
		if e, ok := p.Properties["email"]; ok {
			email = fmt.Sprintf("%v", e)
		}
		lastSeen := ""
		for _, k := range []string{"last_seen", "$last_seen", "updated_at"} {
			if v, ok := p.Properties[k]; ok {
				lastSeen = fmt.Sprintf("%v", v)
				break
			}
		}
		daysSilent := 0
		isSilent := false
		if lastSeen != "" {
			if t, err := time.Parse(time.RFC3339, lastSeen); err == nil && t.Before(silentCutoff) {
				isSilent = true
				daysSilent = int(time.Since(t).Hours() / 24)
			}
		} else if p.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil && t.Before(silentCutoff) {
				isSilent = true
				daysSilent = int(time.Since(t).Hours() / 24)
			}
		}
		if !isSilent {
			continue
		}
		distinctID := ""
		if len(p.DistinctIDs) > 0 {
			distinctID = p.DistinctIDs[0]
		}
		atRisk = append(atRisk, atRiskPerson{
			ID: p.ID, DistinctID: distinctID, Email: email,
			DaysSilent: daysSilent, RiskScore: daysSilent,
		})
	}

	sort.Slice(atRisk, func(i, j int) bool {
		return atRisk[i].RiskScore > atRisk[j].RiskScore
	})
	if atRisk == nil {
		atRisk = []atRiskPerson{}
	}

	result := map[string]any{
		"project_id":            projectID,
		"silent_days_threshold": silentDays,
		"total_checked":         len(rawPersons),
		"at_risk_count":         len(atRisk),
		"at_risk_persons":       atRisk,
		"generated_at":          time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── events property-drift ─────────────────────────────────────────────────────

func registerEventsPropertyDrift(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_events_property-drift",
			mcplib.WithDescription("Detect event schema changes between two time windows. Returns dropped properties (in baseline, missing now) and new properties (appeared now). Use after deploys to catch silent tracking regressions."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID (integer).")),
			mcplib.WithString("event_name", mcplib.Required(), mcplib.Description("Event name to compare (e.g. $pageview, purchase).")),
		),
		handleEventsPropertyDrift,
	)
}

func handleEventsPropertyDrift(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	projectID, _ := args["project_id"].(string)
	eventName, _ := args["event_name"].(string)
	if projectID == "" || eventName == "" {
		return mcplib.NewToolResultError("project_id and event_name are required"), nil
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	basePath := fmt.Sprintf("/api/projects/%s/events/", projectID)
	now := time.Now()

	extractProps := func(after, before time.Time) map[string]int {
		data, err := c.Get(basePath, map[string]string{
			"event":  eventName,
			"limit":  "100",
			"after":  after.Format(time.RFC3339),
			"before": before.Format(time.RFC3339),
		})
		if err != nil {
			return map[string]int{}
		}
		props := map[string]int{}
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
				props[k]++
			}
		}
		return props
	}

	baselineProps := extractProps(now.AddDate(0, 0, -14), now.AddDate(0, 0, -7))
	currentProps := extractProps(now.AddDate(0, 0, -2), now)

	type diff struct {
		Property      string `json:"property"`
		Status        string `json:"status"`
		BaselineCount int    `json:"baseline_occurrences"`
		CurrentCount  int    `json:"current_occurrences"`
	}

	var diffs []diff
	for k, bc := range baselineProps {
		if currentProps[k] == 0 {
			diffs = append(diffs, diff{k, "dropped", bc, 0})
		}
	}
	for k, cc := range currentProps {
		if baselineProps[k] == 0 {
			diffs = append(diffs, diff{k, "new", 0, cc})
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Status != diffs[j].Status {
			return diffs[i].Status < diffs[j].Status
		}
		return diffs[i].Property < diffs[j].Property
	})

	dropped, newCount := 0, 0
	for _, d := range diffs {
		if d.Status == "dropped" {
			dropped++
		} else {
			newCount++
		}
	}
	if diffs == nil {
		diffs = []diff{}
	}

	result := map[string]any{
		"project_id":              projectID,
		"event_name":              eventName,
		"baseline_property_count": len(baselineProps),
		"current_property_count":  len(currentProps),
		"dropped_count":           dropped,
		"new_count":               newCount,
		"diffs":                   diffs,
		"generated_at":            now.UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// ── llm cost-attribution ──────────────────────────────────────────────────────

func registerLLMCostAttribution(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_llm_cost-attribution",
			mcplib.WithDescription("Break down LLM spend by feature flag variant using PostHog LLM Analytics trace reviews. Returns total_cost_usd, trace_count, and avg_cost_per_trace per variant. Requires LLM Observability to be instrumented."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithString("project_id", mcplib.Required(), mcplib.Description("PostHog project ID / environment ID (integer).")),
			mcplib.WithString("flag_key", mcplib.Description("Optional feature flag key to group trace variants by.")),
		),
		handleLLMCostAttribution,
	)
}

func handleLLMCostAttribution(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	projectID, _ := args["project_id"].(string)
	if projectID == "" {
		return mcplib.NewToolResultError("project_id is required"), nil
	}
	flagKey, _ := args["flag_key"].(string)

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	data, err := c.Get(
		fmt.Sprintf("/api/environments/%s/llm_analytics/trace_reviews/", projectID),
		map[string]string{"limit": "500"},
	)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("fetching LLM traces: %v\nEnsure LLM Observability is enabled.", err)), nil
	}

	var traces []map[string]any
	if json.Unmarshal(data, &traces) != nil {
		var envelope struct {
			Results []map[string]any `json:"results"`
		}
		if json.Unmarshal(data, &envelope) == nil {
			traces = envelope.Results
		}
	}

	type variant struct {
		Variant           string  `json:"variant"`
		TraceCount        int     `json:"trace_count"`
		TotalInputTokens  int     `json:"total_input_tokens"`
		TotalOutputTokens int     `json:"total_output_tokens"`
		TotalCostUSD      float64 `json:"total_cost_usd"`
		AvgCostPerTrace   float64 `json:"avg_cost_per_trace"`
	}

	variantMap := map[string]*variant{}
	for _, trace := range traces {
		vKey := "unknown"
		if flagKey != "" {
			for _, field := range []string{"$feature/" + flagKey, "feature_flag_variant", "variant"} {
				if v, ok := trace[field]; ok {
					vKey = fmt.Sprintf("%v", v)
					break
				}
			}
			if vKey == "unknown" {
				if props, ok := trace["properties"].(map[string]any); ok {
					for _, field := range []string{"$feature/" + flagKey, "feature_flag_variant"} {
						if v, ok := props[field]; ok {
							vKey = fmt.Sprintf("%v", v)
							break
						}
					}
				}
			}
		}
		if _, ok := variantMap[vKey]; !ok {
			variantMap[vKey] = &variant{Variant: vKey}
		}
		v := variantMap[vKey]
		v.TraceCount++
		if tok, ok := trace["input_tokens"]; ok {
			v.TotalInputTokens += int(toFloat64(tok))
		}
		if tok, ok := trace["output_tokens"]; ok {
			v.TotalOutputTokens += int(toFloat64(tok))
		}
		if cost, ok := trace["total_cost"]; ok {
			v.TotalCostUSD += toFloat64(cost)
		}
	}

	var variants []variant
	for _, v := range variantMap {
		if v.TraceCount > 0 {
			v.AvgCostPerTrace = v.TotalCostUSD / float64(v.TraceCount)
		}
		variants = append(variants, *v)
	}
	sort.Slice(variants, func(i, j int) bool {
		return variants[i].TotalCostUSD > variants[j].TotalCostUSD
	})
	if variants == nil {
		variants = []variant{}
	}

	result := map[string]any{
		"project_id":   projectID,
		"flag_key":     flagKey,
		"total_traces": len(traces),
		"variants":     variants,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// toFloat64 safely converts any numeric-ish value to float64.
func toFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}
