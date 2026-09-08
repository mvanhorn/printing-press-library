package mcp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cli"
	"github.com/spf13/cobra"
)

func keywordPlannerContext() map[string]any {
	home, _ := os.UserHomeDir()
	database := strings.TrimSpace(os.Getenv("KEYWORD_PLANNER_DB"))
	if database == "" {
		database = filepath.Join(home, ".local", "share", "keyword-planner", "snapshots.db")
	}
	return map[string]any{
		"api":         "keyword-planner",
		"description": "Collect Keyword Planner evidence and inspect immutable snapshots offline.",
		"api_version": "v25", "discovery_revision": "20260831", "transport": "REST",
		"auth":             cli.PlannerAuthMetadata(),
		"portfolio_db":     database,
		"tool_surface":     "The runtime command mirror invokes the same curated CLI commands and evidence policy. Raw endpoint mirrors are disabled.",
		"remote_methods":   []string{"generateKeywordIdeas", "generateKeywordHistoricalMetrics"},
		"remote_mutations": false,
		"commands":         plannerContextCommands(cli.RootCmd()),
		"query_tips": []string{
			"Use ideas with six to eight subject-level seeds and explicit language and geo resource IDs.",
			"Use historical for chosen terms. Ideas pages are collected automatically; historical requests use batches.",
			"Collection output limits do not change collection scope. Explicit request budgets yield incomplete snapshots when exhausted.",
			"Use portfolio commands offline. Monthly gaps and absent money fields remain unavailable.",
			"Use doctor offline by default; its explicit live mode checks authentication and both Planner methods.",
			"Search volume and advertiser bids are vendor estimates, not YouTube demand, RPM, or revenue.",
		},
	}
}

func plannerContextCommands(root *cobra.Command) []map[string]any {
	out := make([]map[string]any, 0)
	for _, cmd := range root.Commands() {
		if cmd.Hidden || cmd.Annotations["mcp:hidden"] == "true" {
			continue
		}
		if cmd.Runnable() {
			out = append(out, map[string]any{"command": cmd.CommandPath(), "description": cmd.Short})
		}
		out = append(out, plannerContextCommands(cmd)...)
	}
	return out
}
