// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/store"
)

type digestDay struct {
	Day        string `json:"day"`
	Sessions   int    `json:"sessions"`
	AgentRuns  int    `json:"agent_runs"`
	Downloads  int    `json:"downloads"`
	FailedSess int    `json:"failed_sessions,omitempty"`
}

type projectDigestView struct {
	ProjectID   string      `json:"project_id"`
	Since       string      `json:"since"`
	Days        []digestDay `json:"days"`
	TotalSess   int         `json:"total_sessions"`
	TotalRuns   int         `json:"total_agent_runs"`
	TotalDl     int         `json:"total_downloads"`
	TotalFailed int         `json:"total_failed_sessions"`
}

func newNovelProjectsDigestCmd(flags *rootFlags) *cobra.Command {
	var flagProject string
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "See everything that ran in a project this week — sessions, agent runs, and downloads grouped by day — from the local store.",
		Long: `Use this command for a per-project weekly activity report (sessions, agent runs, downloads).
Do NOT use it for usage-quota or cost metrics; use 'usage trend' instead.`,
		Example:     "  browserbase-pp-cli projects digest --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "projects digest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagProject == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project is required (project ID)"))
			}
			since := 7 * 24 * time.Hour
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q is invalid: %w (use e.g. 7d, 24h, 1w)", flagSince, err))
				}
				since = parsed
			}

			if dbPath == "" {
				dbPath = defaultDBPath("browserbase-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: browserbase-pp-cli sync --resources sessions,agents,downloads --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), projectDigestView{ProjectID: flagProject, Since: flagSince, Days: []digestDay{}}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "") {
				hintIfStale(cmd, db, "", flags.maxAge)
			}

			cutoff := time.Now().UTC().Add(-since)

			// Drain-first: scan each resource set into plain structs, close rows,
			// then aggregate in memory.
			type rawRow struct {
				resourceType string
				createdAt    string
				status       string
			}
			rawRows := make([]rawRow, 0)

			// Sessions for this project.
			sessRows, err := db.DB().QueryContext(ctx, `
				SELECT resource_type, json_extract(data, '$.createdAt'), json_extract(data, '$.status')
				FROM resources
				WHERE resource_type = 'sessions' AND json_extract(data, '$.projectId') = ?`, flagProject)
			if err != nil {
				return fmt.Errorf("querying sessions: %w", err)
			}
			for sessRows.Next() {
				var r rawRow
				var created, status sql.NullString
				if err := sessRows.Scan(&r.resourceType, &created, &status); err != nil {
					_ = sessRows.Close()
					return fmt.Errorf("scanning session row: %w", err)
				}
				r.createdAt = created.String
				r.status = status.String
				rawRows = append(rawRows, r)
			}
			if err := sessRows.Err(); err != nil {
				_ = sessRows.Close()
				return fmt.Errorf("iterating session rows: %w", err)
			}
			if err := sessRows.Close(); err != nil {
				return fmt.Errorf("closing session rows: %w", err)
			}

			// Agent runs: resource_type 'agents-runs' (hyphenated, as sync writes
			// it). Runs carry no projectId, so count all runs in the window; the
			// digest is scoped to the project via the sessions/agents join when
			// possible, otherwise reported as a project-wide count.
			runRows, err := db.DB().QueryContext(ctx, `
				SELECT resource_type, json_extract(data, '$.createdAt'), json_extract(data, '$.status')
				FROM resources
				WHERE resource_type = 'agents-runs'`)
			if err != nil {
				return fmt.Errorf("querying agent runs: %w", err)
			}
			for runRows.Next() {
				var r rawRow
				var created, status sql.NullString
				if err := runRows.Scan(&r.resourceType, &created, &status); err != nil {
					_ = runRows.Close()
					return fmt.Errorf("scanning agent run row: %w", err)
				}
				r.createdAt = created.String
				r.status = status.String
				rawRows = append(rawRows, r)
			}
			if err := runRows.Err(); err != nil {
				_ = runRows.Close()
				return fmt.Errorf("iterating agent run rows: %w", err)
			}
			if err := runRows.Close(); err != nil {
				return fmt.Errorf("closing agent run rows: %w", err)
			}

			// Downloads: the typed downloads table has session_id + created_at.
			// Join to this project's sessions (which carry projectId) so we only
			// count downloads belonging to the requested project.
			dlRows, err := db.DB().QueryContext(ctx, `
				SELECT 'downloads', d.created_at, ''
				FROM downloads d
				JOIN resources s ON json_extract(s.data, '$.id') = d.session_id
				WHERE s.resource_type = 'sessions' AND json_extract(s.data, '$.projectId') = ?`, flagProject)
			if err != nil {
				return fmt.Errorf("querying downloads: %w", err)
			}
			for dlRows.Next() {
				var r rawRow
				var created, status sql.NullString
				if err := dlRows.Scan(&r.resourceType, &created, &status); err != nil {
					_ = dlRows.Close()
					return fmt.Errorf("scanning download row: %w", err)
				}
				r.createdAt = created.String
				r.status = status.String
				rawRows = append(rawRows, r)
			}
			if err := dlRows.Err(); err != nil {
				_ = dlRows.Close()
				return fmt.Errorf("iterating download rows: %w", err)
			}
			if err := dlRows.Close(); err != nil {
				return fmt.Errorf("closing download rows: %w", err)
			}

			// Aggregate by day.
			byDay := map[string]*digestDay{}
			var totalSess, totalRuns, totalDl, totalFailed int
			for _, r := range rawRows {
				if r.createdAt == "" {
					continue
				}
				t := cliutil.ParseStoredTime(r.createdAt)
				if t.IsZero() || t.Before(cutoff) {
					continue
				}
				day := t.UTC().Format("2006-01-02")
				d := byDay[day]
				if d == nil {
					d = &digestDay{Day: day}
					byDay[day] = d
				}
				switch r.resourceType {
				case "sessions":
					d.Sessions++
					totalSess++
					if strings.EqualFold(r.status, "ERROR") || strings.EqualFold(r.status, "FAILED") {
						d.FailedSess++
						totalFailed++
					}
				case "agent_runs", "agents_runs", "agents_agent_runs", "agents-runs":
					d.AgentRuns++
					totalRuns++
				case "downloads":
					d.Downloads++
					totalDl++
				}
			}

			days := make([]digestDay, 0, len(byDay))
			for _, d := range byDay {
				days = append(days, *d)
			}
			sort.Slice(days, func(i, j int) bool { return days[i].Day < days[j].Day })

			view := projectDigestView{
				ProjectID:   flagProject,
				Since:       flagSince,
				Days:        days,
				TotalSess:   totalSess,
				TotalRuns:   totalRuns,
				TotalDl:     totalDl,
				TotalFailed: totalFailed,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(days) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No activity for project %s in the last %s.\n", flagProject, since)
				return nil
			}
			for _, d := range days {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d sessions\t%d agent runs\t%d downloads", d.Day, d.Sessions, d.AgentRuns, d.Downloads)
				if d.FailedSess > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\t%d failed", d.FailedSess)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "total: %d sessions, %d agent runs, %d downloads (%d failed)\n", totalSess, totalRuns, totalDl, totalFailed)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Project ID to summarize")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look back window (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the CLI data dir)")
	return cmd
}
