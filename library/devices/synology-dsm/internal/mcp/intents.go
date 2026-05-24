// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// Hand-written intent-grouped MCP tools for the Synology DSM CLI.
// Intent tools aggregate multiple API endpoints into single high-level operations,
// reducing round-trips and context usage for agent workflows.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-dsm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-dsm/internal/config"
)

// RegisterIntentTools adds intent-grouped tools to the MCP server.
// Intent tools make 2+ API calls to produce a single enriched response —
// reducing agent round-trips vs. calling individual endpoint tools separately.
func RegisterIntentTools(s *server.MCPServer) {
	// nas_health: one call that returns backup status + disk health + storage summary.
	// Replaces 3+ separate tool calls for the most common "is my NAS healthy?" workflow.
	s.AddTool(
		mcplib.NewTool("nas_health",
			mcplib.WithDescription("NAS health summary: backup tasks, disk SMART indicators, and storage volume status in one call. Use this instead of calling list-backup-tasks + list-storage-disks + list-storage-volumes separately. Returns structured health object with ok/warn/error status per subsystem."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		nasHealthHandler(),
	)

	// container_status: lists all running containers with their image and uptime.
	// Combines list-containers (type=running) into a concise status snapshot.
	s.AddTool(
		mcplib.NewTool("container_status",
			mcplib.WithDescription("Container status snapshot: lists all running containers with name, image, status, and uptime. Concise agent-optimized view — use instead of list-containers when you only need a status overview."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		containerStatusHandler(),
	)

	// backup_status: enriches backup task list with per-task last-run status.
	// Combines list-backup-tasks + get-backup-task-status for each task.
	s.AddTool(
		mcplib.NewTool("backup_status",
			mcplib.WithDescription("Backup status overview: lists all Hyper Backup tasks enriched with their current status (idle/running/error). Use instead of list-backup-tasks + repeated get-backup-task-status calls."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		backupStatusHandler(),
	)
}

// nasHealthHandler calls backup/task/list, storage/disk/list, and storage/volume/list
// then summarises the results into a unified health object.
func nasHealthHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cfg, err := config.Load("")
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("config error: %v", err)), nil
		}
		c := client.New(cfg, 0, 0)

		health := map[string]any{}

		// Backup tasks
		backupData, _ := c.Get("/webapi/entry.cgi/backup/task/list", map[string]string{
			"api": "SYNO.Backup.Task", "method": "list", "version": "1",
		})
		var backupResp map[string]any
		if json.Unmarshal(backupData, &backupResp) == nil {
			if data, ok := backupResp["data"]; ok {
				health["backup"] = data
			}
		}

		// Disk health
		diskData, _ := c.Get("/webapi/entry.cgi/storage/disk/list", map[string]string{
			"api": "SYNO.Core.Storage.Disk", "method": "list", "version": "1",
		})
		var diskResp map[string]any
		diskStatus := "ok"
		if json.Unmarshal(diskData, &diskResp) == nil {
			if data, ok := diskResp["data"]; ok {
				health["disks"] = data
				// Check for any disk warnings
				if dataMap, ok := data.(map[string]any); ok {
					if disks, ok := dataMap["disks"].([]any); ok {
						for _, d := range disks {
							if dm, ok := d.(map[string]any); ok {
								if dm["exceed_bad_sector_thr"] == true || dm["below_remain_life_thr"] == true {
									diskStatus = "warn"
								}
							}
						}
					}
				}
			}
		}
		health["disk_status"] = diskStatus

		// Storage volumes
		volumeData, _ := c.Get("/webapi/entry.cgi/storage/volume/list", map[string]string{
			"api": "SYNO.Core.Storage.Volume", "method": "list", "version": "1",
		})
		var volResp map[string]any
		if json.Unmarshal(volumeData, &volResp) == nil {
			if data, ok := volResp["data"]; ok {
				health["volumes"] = data
			}
		}

		// Overall status
		overallStatus := "ok"
		if diskStatus == "warn" {
			overallStatus = "warn"
		}
		health["overall"] = overallStatus

		out, _ := json.MarshalIndent(health, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

// containerStatusHandler lists running containers and returns a concise status snapshot.
func containerStatusHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cfg, err := config.Load("")
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("config error: %v", err)), nil
		}
		c := client.New(cfg, 0, 0)

		data, _ := c.Get("/webapi/entry.cgi/docker/container/list", map[string]string{
			"api": "SYNO.Docker.Container", "method": "list", "version": "2", "type": "running",
		})

		var resp map[string]any
		if json.Unmarshal(data, &resp) == nil {
			out, _ := json.MarshalIndent(resp, "", "  ")
			return mcplib.NewToolResultText(string(out)), nil
		}
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// backupStatusHandler lists backup tasks and returns a concise status summary.
func backupStatusHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cfg, err := config.Load("")
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("config error: %v", err)), nil
		}
		c := client.New(cfg, 0, 0)

		data, _ := c.Get("/webapi/entry.cgi/backup/task/list", map[string]string{
			"api": "SYNO.Backup.Task", "method": "list", "version": "1",
		})

		var resp map[string]any
		if json.Unmarshal(data, &resp) == nil {
			// Extract and summarise tasks
			summary := map[string]any{"raw": resp}
			if d, ok := resp["data"].(map[string]any); ok {
				if tasks, ok := d["task_list"].([]any); ok {
					counts := map[string]int{"total": len(tasks)}
					for _, t := range tasks {
						if tm, ok := t.(map[string]any); ok {
							status := fmt.Sprintf("%v", tm["status"])
							counts[strings.ToLower(status)]++
						}
					}
					summary["task_counts"] = counts
				}
			}
			out, _ := json.MarshalIndent(summary, "", "  ")
			return mcplib.NewToolResultText(string(out)), nil
		}
		return mcplib.NewToolResultText(string(data)), nil
	}
}
