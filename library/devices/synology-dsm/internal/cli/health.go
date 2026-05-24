// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// Hand-written insight command: NAS health summary across backup, disk, and storage subsystems.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

// newHealthCmd returns a command that aggregates backup task status,
// disk SMART indicators, and storage volume usage into a single NAS
// health summary. Makes 3 API calls so agents need only one command
// to assess overall NAS health.
func newHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "NAS health summary: backup tasks, disk SMART indicators, and storage volumes",
		Long: `Aggregates backup task status, disk SMART health flags, and storage volume
free space into a single summary. Equivalent to running list-backup-tasks,
list-storage-disks, and list-storage-volumes in sequence.

Each subsystem is rated ok / warn / error:
  backup  — any task in error state → error; running → ok
  disks   — exceed_bad_sector_thr or below_remain_life_thr → warn
  volumes — any volume degraded → error; usage >80% → warn

Agents should call this before triggering backup or storage operations.`,
		Example: `  # Human-readable summary
  synology-dsm-pp-cli health

  # JSON for agent consumption
  synology-dsm-pp-cli health --json

  # Only show subsystems with warnings or errors
  synology-dsm-pp-cli health --json --select overall,backup_status,disk_status,volume_status`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			summary := map[string]any{}
			overallStatus := "ok"

			// ── 1. Backup tasks ──────────────────────────────────────────────────
			backupRaw, _ := c.Get("/webapi/entry.cgi/backup/task/list", map[string]string{
				"api": "SYNO.Backup.Task", "method": "list", "version": "1",
			})
			backupStatus := "ok"
			var backupTasks []map[string]any
			var backupResp map[string]any
			if json.Unmarshal(backupRaw, &backupResp) == nil {
				if d, ok := backupResp["data"].(map[string]any); ok {
					if tasks, ok := d["task_list"].([]any); ok {
						for _, t := range tasks {
							if tm, ok := t.(map[string]any); ok {
								backupTasks = append(backupTasks, tm)
								st := fmt.Sprintf("%v", tm["status"])
								if st == "error" || st == "err" {
									backupStatus = "error"
									overallStatus = "error"
								}
							}
						}
					}
				}
			}
			summary["backup_status"] = backupStatus
			summary["backup_task_count"] = len(backupTasks)

			// ── 2. Disk SMART health ─────────────────────────────────────────────
			diskData, _ := c.Get("/webapi/entry.cgi/storage/disk/list", map[string]string{
				"api": "SYNO.Core.Storage.Disk", "method": "list", "version": "1",
			})
			diskStatus := "ok"
			diskWarnCount := 0
			var diskWarnings []string
			var diskResp map[string]any
			if json.Unmarshal(diskData, &diskResp) == nil {
				if d, ok := diskResp["data"].(map[string]any); ok {
					if disks, ok := d["disks"].([]any); ok {
						for _, disk := range disks {
							dm, ok := disk.(map[string]any)
							if !ok {
								continue
							}
							diskName := fmt.Sprintf("%v", dm["name"])
							if dm["exceed_bad_sector_thr"] == true {
								diskWarnCount++
								diskWarnings = append(diskWarnings, fmt.Sprintf("%s: bad sector threshold exceeded", diskName))
								diskStatus = "warn"
								if overallStatus == "ok" {
									overallStatus = "warn"
								}
							}
							if dm["below_remain_life_thr"] == true {
								diskWarnCount++
								diskWarnings = append(diskWarnings, fmt.Sprintf("%s: remaining life below threshold", diskName))
								diskStatus = "warn"
								if overallStatus == "ok" {
									overallStatus = "warn"
								}
							}
						}
					}
				}
			}
			sort.Strings(diskWarnings)
			summary["disk_status"] = diskStatus
			summary["disk_warn_count"] = diskWarnCount
			if len(diskWarnings) > 0 {
				summary["disk_warnings"] = diskWarnings
			}

			// ── 3. Storage volume ────────────────────────────────────────────────
			volData, _ := c.Get("/webapi/entry.cgi/storage/volume/get", map[string]string{
				"api": "SYNO.Core.Storage.Volume", "method": "get", "version": "1",
				"volume_path": "/volume1",
			})
			volStatus := "ok"
			var volDetails []map[string]any
			var volResp map[string]any
			if json.Unmarshal(volData, &volResp) == nil {
				if d, ok := volResp["data"].(map[string]any); ok {
					if vm, ok := d["volume"].(map[string]any); ok {
						volDetails = append(volDetails, vm)
						st := fmt.Sprintf("%v", vm["status"])
						if st == "degraded" || st == "crashed" {
							volStatus = "error"
							overallStatus = "error"
						}
						t := parseByteStr(vm["size_total_byte"])
						f := parseByteStr(vm["size_free_byte"])
						if t > 0 {
							u := t - f
							usedPct := (u / t) * 100
							vm["usage_pct"] = fmt.Sprintf("%.1f%%", usedPct)
							if usedPct > 80 && volStatus == "ok" {
								volStatus = "warn"
								if overallStatus == "ok" {
									overallStatus = "warn"
								}
							}
						}
					}
				}
			}
			summary["volume_status"] = volStatus
			summary["volume_count"] = len(volDetails)

			summary["overall"] = overallStatus

			// Output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := mustMarshal(summary)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			statusIcon := func(s string) string {
				switch s {
				case "ok":
					return green("✓")
				case "warn":
					return yellow("⚠")
				default:
					return red("✗")
				}
			}
			fmt.Fprintf(w, "  %s Overall: %s\n", statusIcon(overallStatus), overallStatus)
			fmt.Fprintf(w, "  %s Backup: %s (%d tasks)\n", statusIcon(backupStatus), backupStatus, len(backupTasks))
			fmt.Fprintf(w, "  %s Disks:  %s", statusIcon(diskStatus), diskStatus)
			if diskWarnCount > 0 {
				fmt.Fprintf(w, " (%d warning(s))", diskWarnCount)
			}
			fmt.Fprintln(w)
			for _, warn := range diskWarnings {
				fmt.Fprintf(w, "    · %s\n", warn)
			}
			fmt.Fprintf(w, "  %s Volumes: %s (%d volume(s))\n", statusIcon(volStatus), volStatus, len(volDetails))

			return nil
		},
	}
	return cmd
}

// mustMarshal marshals v to JSON, returning an empty object on error.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: marshal error: %v\n", err)
		return json.RawMessage("{}")
	}
	return b
}
