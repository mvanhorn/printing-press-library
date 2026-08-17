// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/store"
)

type usagePoint struct {
	At             string  `json:"at"`
	BrowserMinutes float64 `json:"browser_minutes"`
	ProxyBytes     float64 `json:"proxy_bytes"`
	DeltaMinutes   float64 `json:"delta_minutes,omitempty"`
}

type usageTrendView struct {
	ProjectID      string       `json:"project_id"`
	Since          string       `json:"since"`
	Points         []usagePoint `json:"points"`
	CurrentMinutes float64      `json:"current_browser_minutes"`
	CurrentBytes   float64      `json:"current_proxy_bytes"`
	DeltaMinutes   float64      `json:"delta_browser_minutes"`
}

func newNovelUsageTrendCmd(flags *rootFlags) *cobra.Command {
	var flagProject string
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Track per-project browserMinutes and proxyBytes over sync history and spot quota creep before the bill arrives.",
		Long: `Use this command to see how project usage quotas have changed over sync history.
Do NOT use it for activity summaries; use 'projects digest' instead.`,
		Example:     "  browserbase-pp-cli usage trend --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "usage trend")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagProject == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project is required (project ID)"))
			}
			since := 30 * 24 * time.Hour
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q is invalid: %w (use e.g. 7d, 30d, 1w)", flagSince, err))
				}
				since = parsed
			}

			if dbPath == "" {
				dbPath = defaultDBPath("browserbase-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: browserbase-pp-cli sync --resources projects --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), usageTrendView{ProjectID: flagProject, Since: flagSince, Points: []usagePoint{}}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "projects") {
				hintIfStale(cmd, db, "projects", flags.maxAge)
			}

			cutoff := time.Now().UTC().Add(-since)

			// Usage rows live in the typed "usage" table: synced_at holds the
			// snapshot timestamp, projects_id scopes to the project, and the
			// payload (browserMinutes/proxyBytes) is in data.
			rows, err := db.DB().QueryContext(ctx, `
				SELECT synced_at, json_extract(data, '$.browserMinutes'), json_extract(data, '$.proxyBytes')
				FROM usage
				WHERE projects_id = ? AND synced_at >= ?`, flagProject, cutoff.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying usage snapshots: %w", err)
			}
			type rawUsage struct {
				at      string
				minutes float64
				bytes   float64
			}
			rawRows := make([]rawUsage, 0)
			for rows.Next() {
				var at string
				var minutes, bytes sql.NullFloat64
				if err := rows.Scan(&at, &minutes, &bytes); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning usage row: %w", err)
				}
				rawRows = append(rawRows, rawUsage{at: at, minutes: minutes.Float64, bytes: bytes.Float64})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating usage rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			points := make([]usagePoint, 0, len(rawRows))
			for _, r := range rawRows {
				if r.at == "" {
					continue
				}
				t := cliutil.ParseStoredTime(r.at)
				if t.IsZero() || t.Before(cutoff) {
					continue
				}
				points = append(points, usagePoint{At: t.UTC().Format(time.RFC3339), BrowserMinutes: r.minutes, ProxyBytes: r.bytes})
			}

			// Self-populating baseline: if no snapshot exists in the window,
			// capture the live project usage now so the trend has a starting
			// point (the usage endpoint is a current-total snapshot, so each
			// subsequent `usage trend` run accumulates the series).
			if len(points) == 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				usagePath := replacePathParam("/v1/projects/{id}/usage", "id", flagProject)
				data, err := c.Get(ctx, usagePath, nil)
				if err == nil && len(data) > 0 {
					var usage struct {
						BrowserMinutes float64 `json:"browserMinutes"`
						ProxyBytes     float64 `json:"proxyBytes"`
					}
					if json.Unmarshal(data, &usage) == nil {
						// Store a snapshot via UpsertUsage (typed usage table).
						// The payload carries an id (the project id) so the
						// store's extractObjectID resolves it, and projects_id
						// scopes the trend query.
						snapshot := map[string]any{
							"id":             flagProject,
							"projects_id":    flagProject,
							"browserMinutes": usage.BrowserMinutes,
							"proxyBytes":     usage.ProxyBytes,
						}
						if snapJSON, marshalErr := json.Marshal(snapshot); marshalErr == nil {
							_ = db.UpsertUsage(snapJSON)
						}
						points = append(points, usagePoint{
							At:             time.Now().UTC().Format(time.RFC3339),
							BrowserMinutes: usage.BrowserMinutes,
							ProxyBytes:     usage.ProxyBytes,
						})
					}
				}
			}
			sort.Slice(points, func(i, j int) bool { return points[i].At < points[j].At })
			for i := 1; i < len(points); i++ {
				points[i].DeltaMinutes = points[i].BrowserMinutes - points[i-1].BrowserMinutes
			}

			view := usageTrendView{
				ProjectID: flagProject,
				Since:     flagSince,
				Points:    points,
			}
			if len(points) > 0 {
				view.CurrentMinutes = points[len(points)-1].BrowserMinutes
				view.CurrentBytes = points[len(points)-1].ProxyBytes
				view.DeltaMinutes = points[len(points)-1].BrowserMinutes - points[0].BrowserMinutes
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(points) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No usage snapshots for project %s in the last %s.\n", flagProject, since)
				return nil
			}
			for _, p := range points {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.1f min\t%.1f MB\t%+.1f\n", p.At, p.BrowserMinutes, p.ProxyBytes/1024/1024, p.DeltaMinutes)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "total delta: %+.1f browser minutes\n", view.DeltaMinutes)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Project ID to track")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Look back window (e.g. 7d, 30d, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the CLI data dir)")
	return cmd
}
