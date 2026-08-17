// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/store"
)

type orphanSessionView struct {
	ID         string  `json:"id"`
	ProjectID  string  `json:"project_id,omitempty"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at,omitempty"`
	AgeMinutes float64 `json:"age_minutes"`
	Stopped    bool    `json:"stopped,omitempty"`
}

type orphansView struct {
	Items        []orphanSessionView `json:"items"`
	TotalMinutes float64             `json:"total_minutes"`
	OrphanCount  int                 `json:"orphan_count"`
	StoppedCount int                 `json:"stopped_count,omitempty"`
	MaxScan      int                 `json:"max_scan_records"`
	Note         string              `json:"note,omitempty"`
}

func newNovelSessionsOrphansCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string
	var flagStop bool
	var flagLimit int
	var flagMaxScan int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Find running sessions that were never released (keepAlive orphans) and the runtime they're burning, then optionally stop them in batch.",
		Long: `Use this command to find running sessions that were never released (keepAlive orphans) and the runtime they're burning.
Do NOT use it for an overall status breakdown of all sessions; use 'projects digest' instead.`,
		Example:     "  browserbase-pp-cli sessions orphans --older-than 15m --stop --json",
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:write-positionals": "0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sessions orphans")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			olderThan := 15 * time.Minute
			if flagOlderThan != "" {
				parsed, err := cliutil.ParseDurationLoose(flagOlderThan)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--older-than %q is invalid: %w (use e.g. 15m, 1h)", flagOlderThan, err))
				}
				olderThan = parsed
			}

			if dbPath == "" {
				dbPath = defaultDBPath("browserbase-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: browserbase-pp-cli sync --resources sessions --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), orphansView{Items: []orphanSessionView{}, MaxScan: flagMaxScan}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "sessions") {
				hintIfStale(cmd, db, "sessions", flags.maxAge)
			}

			rows, err := db.DB().QueryContext(ctx, `
				SELECT id, json_extract(data, '$.status') AS status,
				       json_extract(data, '$.projectId') AS projectId,
				       json_extract(data, '$.createdAt') AS createdAt
				FROM resources
				WHERE resource_type = 'sessions'
				ORDER BY json_extract(data, '$.createdAt') ASC
				LIMIT ?`, flagMaxScan)
			if err != nil {
				return fmt.Errorf("querying sessions: %w", err)
			}
			type rawRow struct {
				id        string
				status    string
				projectID string
				createdAt string
			}
			rawRows := make([]rawRow, 0)
			for rows.Next() {
				var r rawRow
				if err := rows.Scan(&r.id, &r.status, &r.projectID, &r.createdAt); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning session row: %w", err)
				}
				rawRows = append(rawRows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating session rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			now := time.Now().UTC()
			matches := make([]orphanSessionView, 0, len(rawRows))
			var totalMinutes float64
			scanned := len(rawRows)
			for _, r := range rawRows {
				status := strings.ToUpper(strings.TrimSpace(r.status))
				if status != "RUNNING" && status != "PENDING" {
					continue
				}
				if r.createdAt == "" {
					continue
				}
				createdAt := cliutil.ParseStoredTime(r.createdAt)
				if createdAt.IsZero() {
					continue
				}
				age := now.Sub(createdAt)
				if age < olderThan {
					continue
				}
				matches = append(matches, orphanSessionView{
					ID:         r.id,
					ProjectID:  r.projectID,
					Status:     status,
					CreatedAt:  r.createdAt,
					AgeMinutes: age.Minutes(),
				})
				totalMinutes += age.Minutes()
				if flagLimit > 0 && len(matches) >= flagLimit {
					break
				}
			}

			view := orphansView{
				Items:        matches,
				TotalMinutes: totalMinutes,
				OrphanCount:  len(matches),
				MaxScan:      flagMaxScan,
			}
			if len(matches) == 0 && scanned >= flagMaxScan && flagMaxScan > 0 {
				view.Note = fmt.Sprintf("scanned %d sessions without finding orphans older than %s; raise --max-scan-records to widen the search", scanned, olderThan)
			}

			// Optional batch release: stop each orphan via the live API.
			if flagStop && len(matches) > 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				stopped := 0
				for i := range matches {
					relBody := map[string]any{"status": "REQUEST_RELEASE"}
					relPath := replacePathParam("/v1/sessions/{id}", "id", matches[i].ID)
					if _, statusCode, relErr := c.PostWithParams(ctx, relPath, nil, relBody); relErr == nil && statusCode >= 200 && statusCode < 300 {
						matches[i].Stopped = true
						stopped++
					} else if relErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to release session %s: %v\n", matches[i].ID, relErr)
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to release session %s: HTTP %d\n", matches[i].ID, statusCode)
					}
				}
				view.StoppedCount = stopped
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No orphaned sessions found.")
				return nil
			}
			for _, m := range matches {
				stopped := ""
				if m.Stopped {
					stopped = " (stopped)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%.0fm old%s\n", m.ID, m.Status, m.AgeMinutes, stopped)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d orphan(s), %.1f total minutes\n", view.OrphanCount, view.TotalMinutes)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "15m", "Only flag sessions running longer than this (e.g. 15m, 1h)")
	cmd.Flags().BoolVar(&flagStop, "stop", false, "Batch-release flagged sessions via REQUEST_RELEASE")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum orphans to return (0 = all)")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan-records", 5000, "Maximum local session rows to scan")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the CLI data dir)")
	return cmd
}
