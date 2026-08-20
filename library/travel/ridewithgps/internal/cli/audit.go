// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `audit` runs catalog-hygiene checks over the locally synced routes mirror.
// Hand-authored transcendence command.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
	"github.com/spf13/cobra"
)

type auditFinding struct {
	RouteID string   `json:"route_id"`
	Name    string   `json:"name"`
	Issues  []string `json:"issues"`
}

type auditView struct {
	ChecksRun []string       `json:"checks_run"`
	StaleDays int            `json:"stale_days"`
	Flagged   []auditFinding `json:"flagged"`
	Counts    map[string]int `json:"counts"`
	Note      string         `json:"note,omitempty"`
}

var auditKnownChecks = map[string]bool{"stale": true, "private": true, "incomplete": true}

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	var checks string
	var staleDays int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Flag routes that are stale, private, or incomplete — catalog hygiene across your library.",
		Long: `Run hygiene checks over your locally synced routes.

Checks (comma-separated via --checks, default all):
  stale       not updated within --stale-days (default 365)
  private     visibility is 'private' (not shareable)
  incomplete  missing name, missing description, or zero distance

Reads the local SQLite mirror only — run 'ridewithgps-pp-cli sync --resources routes'
first. For duplicate detection use 'dedup'.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli audit
  ridewithgps-pp-cli audit --checks stale,private --json
  ridewithgps-pp-cli audit --checks stale --stale-days 180 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit local routes for hygiene issues")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			active := map[string]bool{}
			if strings.TrimSpace(checks) == "" {
				active = map[string]bool{"stale": true, "private": true, "incomplete": true}
			} else {
				for _, c := range strings.Split(checks, ",") {
					c = strings.TrimSpace(strings.ToLower(c))
					if c == "" {
						continue
					}
					if !auditKnownChecks[c] {
						_ = cmd.Usage()
						return usageErr(fmt.Errorf("unknown check %q; valid checks: stale, private, incomplete", c))
					}
					active[c] = true
				}
			}
			if staleDays <= 0 {
				staleDays = 365
			}

			if dbPath == "" {
				dbPath = defaultDBPath("ridewithgps-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ridewithgps-pp-cli sync --resources routes --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "routes", flags.maxAge)

			rows, err := db.DB().QueryContext(ctx, `SELECT id, COALESCE(name,''), COALESCE(description,''),
					COALESCE(distance,0), COALESCE(visibility,''),
					CASE WHEN updated_at IS NOT NULL AND updated_at < datetime('now', ?) THEN 1 ELSE 0 END AS is_stale
				FROM routes`, fmt.Sprintf("-%d days", staleDays))
			if err != nil {
				return fmt.Errorf("querying routes: %w", err)
			}
			defer rows.Close()

			checksRun := make([]string, 0, len(active))
			for c := range active {
				checksRun = append(checksRun, c)
			}
			sort.Strings(checksRun)

			view := auditView{
				ChecksRun: checksRun,
				StaleDays: staleDays,
				Flagged:   make([]auditFinding, 0),
				Counts:    map[string]int{},
			}
			for rows.Next() {
				var id, name, desc, visibility sql.NullString
				var distance sql.NullFloat64
				var isStale sql.NullInt64
				if err := rows.Scan(&id, &name, &desc, &distance, &visibility, &isStale); err != nil {
					continue
				}
				var issues []string
				if active["stale"] && isStale.Int64 == 1 {
					issues = append(issues, "stale")
				}
				if active["private"] && strings.EqualFold(visibility.String, "private") {
					issues = append(issues, "private")
				}
				if active["incomplete"] {
					if strings.TrimSpace(name.String) == "" || strings.TrimSpace(desc.String) == "" || distance.Float64 <= 0 {
						issues = append(issues, "incomplete")
					}
				}
				if len(issues) == 0 {
					continue
				}
				for _, iss := range issues {
					view.Counts[iss]++
				}
				view.Flagged = append(view.Flagged, auditFinding{RouteID: id.String, Name: name.String, Issues: issues})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading routes: %w", err)
			}
			if len(view.Flagged) == 0 {
				view.Note = "no routes flagged by the selected checks"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Checks: %s (stale threshold: %d days)\n", strings.Join(checksRun, ", "), staleDays)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "Route\tName\tIssues\n")
			fmt.Fprintf(tw, "-----\t----\t------\n")
			for _, f := range view.Flagged {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", f.RouteID, truncate(f.Name, 40), strings.Join(f.Issues, ", "))
			}
			_ = tw.Flush()
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d routes flagged.\n", len(view.Flagged))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&checks, "checks", "", "Comma-separated checks: stale, private, incomplete (default: all)")
	cmd.Flags().IntVar(&staleDays, "stale-days", 365, "Days without an update before a route is 'stale'")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}
