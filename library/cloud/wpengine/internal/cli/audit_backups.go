// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fleet-wide backup staleness audit.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/wpengine/internal/cliutil"
)

type auditBackupRow struct {
	Install       string `json:"install"`
	InstallID     string `json:"install_id"`
	Environment   string `json:"environment,omitempty"`
	Status        string `json:"install_status,omitempty"`
	LastBackupAt  string `json:"last_backup_at,omitempty"`
	AgeDays       int    `json:"age_days"`
	NeverBackedUp bool   `json:"never_backed_up"`
}

// auditBackupsView mirrors the audit versions/domains envelope convention:
// results plus a scanned denominator, so JSON consumers can distinguish a
// clean fleet (scanned > 0, stale empty) from an unsynced or empty mirror
// (scanned == 0). scanned_installs counts installs after the --env filter,
// matching the human summary line.
type auditBackupsView struct {
	Stale           []auditBackupRow `json:"stale"`
	ScannedInstalls int              `json:"scanned_installs"`
}

func newNovelAuditBackupsCmd(flags *rootFlags) *cobra.Command {
	var flagStale string
	var flagEnv string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Find installs whose latest completed backup is older than a threshold you set",
		Long: strings.Trim(`
Use this command for fleet-wide backup staleness across installs: it joins
synced installs and backups in the local mirror, keeps the latest completed
backup per install, and reports installs whose latest completed backup is
older than the threshold (installs with no completed backup at all are always
reported).

Do NOT use it to list backups for a single install; use
'installs backups list' instead.

Requires a synced mirror: run 'wpengine-pp-cli sync' first.
`, "\n"),
		Example: strings.Trim(`
  # Production installs with no completed backup in 7 days
  wpengine-pp-cli audit backups --stale 7d --env production

  # Whole fleet, agent-friendly JSON
  wpengine-pp-cli audit backups --stale 3d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan local mirror for installs with completed backups older than", flagStale)
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			threshold, err := cliutil.ParseDurationLoose(flagStale)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --stale %q: %w (use forms like 7d, 1w, 72h)", flagStale, err))
			}
			db, handled, err := openAuditStore(cmd, flags, dbPath, "backups", `{"stale":[],"scanned_installs":0}`)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			ctx := cmd.Context()

			// Drain installs first.
			type installRow struct {
				id, name, environment, status string
			}
			irows, err := db.DB().QueryContext(ctx, `
				SELECT id, COALESCE(name, ''), COALESCE(environment, ''), COALESCE(status, '')
				FROM installs`)
			if err != nil {
				return fmt.Errorf("querying installs: %w", err)
			}
			installs := make([]installRow, 0)
			if err := drainRows(irows, "install", func(rs *sql.Rows) error {
				var r installRow
				if err := rs.Scan(&r.id, &r.name, &r.environment, &r.status); err != nil {
					return err
				}
				installs = append(installs, r)
				return nil
			}); err != nil {
				return err
			}

			// Then drain backups and compute the latest completed per install.
			brows, err := db.DB().QueryContext(ctx, `SELECT installs_id, data FROM backups`)
			if err != nil {
				return fmt.Errorf("querying backups: %w", err)
			}
			latestCompleted := make(map[string]time.Time)
			if err := drainRows(brows, "backup", func(rs *sql.Rows) error {
				var installID string
				var data []byte
				if err := rs.Scan(&installID, &data); err != nil {
					return err
				}
				var b struct {
					Status       string `json:"status"`
					CompleteTime string `json:"complete_time"`
					CreateTime   string `json:"create_time"`
				}
				if err := json.Unmarshal(data, &b); err != nil {
					return nil // malformed row: skip, don't fail the audit
				}
				if b.Status != "completed" {
					return nil
				}
				ts, ok := parseAPITime(b.CompleteTime)
				if !ok {
					ts, ok = parseAPITime(b.CreateTime)
				}
				if !ok {
					return nil
				}
				if cur, seen := latestCompleted[installID]; !seen || ts.After(cur) {
					latestCompleted[installID] = ts
				}
				return nil
			}); err != nil {
				return err
			}

			now := time.Now().UTC()
			envFilter := strings.ToLower(strings.TrimSpace(flagEnv))
			matching := 0
			results := make([]auditBackupRow, 0)
			for _, ins := range installs {
				if envFilter != "" && !strings.EqualFold(ins.environment, envFilter) {
					continue
				}
				matching++
				last, has := latestCompleted[ins.id]
				if has && now.Sub(last) <= threshold {
					continue
				}
				row := auditBackupRow{
					Install:       ins.name,
					InstallID:     ins.id,
					Environment:   ins.environment,
					Status:        ins.status,
					NeverBackedUp: !has,
				}
				if has {
					row.LastBackupAt = last.UTC().Format(time.RFC3339)
					row.AgeDays = int(now.Sub(last).Hours() / 24)
				} else {
					row.AgeDays = -1
				}
				results = append(results, row)
			}
			// Never-backed-up first, then oldest backup first.
			sort.Slice(results, func(i, j int) bool {
				if results[i].NeverBackedUp != results[j].NeverBackedUp {
					return results[i].NeverBackedUp
				}
				return results[i].LastBackupAt < results[j].LastBackupAt
			})

			if len(results) == 0 && !flags.asJSON && !flags.agent {
				fmt.Fprintf(cmd.OutOrStdout(), "all %d matching installs have a completed backup within %s\n", matching, flagStale)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), auditBackupsView{
				Stale:           results,
				ScannedInstalls: matching,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagStale, "stale", "7d", "Staleness threshold (e.g. 7d, 1w, 72h); installs whose latest completed backup is older are reported")
	cmd.Flags().StringVar(&flagEnv, "env", "", "Filter by environment (production, staging, development); empty = all")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local data dir)")
	return cmd
}
