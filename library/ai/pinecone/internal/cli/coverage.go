// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// coverage: join synced backups x backup-schedules x restore-jobs x indexes
// into a per-index protection matrix.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type coverageRow struct {
	Index       string `json:"index"`
	Host        string `json:"host,omitempty"`
	BackedUp    bool   `json:"backed_up"`
	LastBackup  string `json:"last_backup,omitempty"`
	HasSchedule bool   `json:"has_schedule"`
	ScheduleID  string `json:"schedule_id,omitempty"`
	Restorable  bool   `json:"restorable"`
	Protected   bool   `json:"protected"`
}

type coverageEnvelope struct {
	GeneratedAt string        `json:"generated_at"`
	Indexes     []coverageRow `json:"indexes"`
	Protected   int           `json:"protected_count"`
	Total       int           `json:"total_count"`
	Note        string        `json:"note,omitempty"`
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report backup/restore protection coverage across your indexes",
		Long: `Report backup/restore protection coverage across your indexes.

Use this command for the backup/restore protection report joined across backups, schedules, and restore jobs.
Do NOT use this command for per-job status inspection; use 'analytics --type backups' or 'tail --resource backup-schedules'.`,
		Example:     `  pinecone-pp-cli coverage --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			resolvedDB, err := defaultNovelDB(dbPath)
			if err != nil {
				return err
			}
			if missingMirrorHint(cmd.ErrOrStderr(), resolvedDB) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), coverageEnvelope{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Indexes: []coverageRow{}, Protected: 0, Total: 0}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No local data; run 'pinecone-pp-cli sync --resources indexes,backups,backup-schedules,restore-jobs' first.")
				return nil
			}
			s, db, err := openNovelDB(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			// Gather indexes from the synced resources table.
			rows, err := db.QueryContext(ctx,
				`SELECT data FROM resources WHERE resource_type = 'indexes'`)
			if err != nil {
				return fmt.Errorf("querying indexes: %w", err)
			}
			type idx struct {
				Name string `json:"name"`
				Host string `json:"host"`
			}
			var indexes []idx
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning index: %w", err)
				}
				var i idx
				_ = json.Unmarshal([]byte(data), &i)
				if i.Name != "" {
					indexes = append(indexes, i)
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating indexes: %w", err)
			}
			_ = rows.Close()

			// Backups: map index -> last backup time.
			backupByIndex := map[string]string{}
			rows, err = db.QueryContext(ctx,
				`SELECT data FROM resources WHERE resource_type = 'backups'`)
			if err != nil {
				return fmt.Errorf("querying backups: %w", err)
			}
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning backup: %w", err)
				}
				var b struct {
					IndexName string `json:"index_name"`
					Status    string `json:"status"`
					CreatedAt string `json:"created_at"`
				}
				_ = json.Unmarshal([]byte(data), &b)
				if b.IndexName != "" && (b.Status == "Ready" || b.Status == "Completed" || b.CreatedAt != "") {
					if t, ok := backupByIndex[b.IndexName]; !ok || b.CreatedAt > t {
						backupByIndex[b.IndexName] = b.CreatedAt
					}
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating backups: %w", err)
			}
			_ = rows.Close()

			// Backup schedules: map index -> schedule id.
			scheduleByIndex := map[string]string{}
			rows, err = db.QueryContext(ctx,
				`SELECT data FROM resources WHERE resource_type = 'backup-schedules'`)
			if err != nil {
				return fmt.Errorf("querying schedules: %w", err)
			}
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning schedule: %w", err)
				}
				var sc struct {
					IndexName string `json:"index_name"`
					ID        string `json:"id"`
				}
				_ = json.Unmarshal([]byte(data), &sc)
				if sc.IndexName != "" && sc.ID != "" {
					scheduleByIndex[sc.IndexName] = sc.ID
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating schedules: %w", err)
			}
			_ = rows.Close()

			// Restore jobs: which indexes have ever been restored.
			restored := map[string]bool{}
			rows, err = db.QueryContext(ctx,
				`SELECT data FROM resources WHERE resource_type = 'restore-jobs'`)
			if err != nil {
				return fmt.Errorf("querying restore jobs: %w", err)
			}
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning restore job: %w", err)
				}
				var rj struct {
					IndexName string `json:"index_name"`
				}
				_ = json.Unmarshal([]byte(data), &rj)
				if rj.IndexName != "" {
					restored[rj.IndexName] = true
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating restore jobs: %w", err)
			}
			_ = rows.Close()

			out := make([]coverageRow, 0, len(indexes))
			protected := 0
			for _, i := range indexes {
				row := coverageRow{
					Index:       i.Name,
					Host:        i.Host,
					BackedUp:    backupByIndex[i.Name] != "",
					LastBackup:  backupByIndex[i.Name],
					HasSchedule: scheduleByIndex[i.Name] != "",
					ScheduleID:  scheduleByIndex[i.Name],
					Restorable:  restored[i.Name],
				}
				row.Protected = row.BackedUp || row.HasSchedule
				if row.Protected {
					protected++
				}
				out = append(out, row)
			}
			env := coverageEnvelope{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Indexes:     out,
				Protected:   protected,
				Total:       len(out),
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No indexes found in local store. Run 'pinecone-pp-cli sync --resources indexes' first.")
				return nil
			}
			for _, r := range out {
				flag := "UNPROTECTED"
				if r.Protected {
					flag = "protected"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-12s backup=%s schedule=%s\n", r.Index, flag, r.LastBackup, r.ScheduleID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d/%d indexes protected\n", protected, len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: platform data dir)")
	return cmd
}
