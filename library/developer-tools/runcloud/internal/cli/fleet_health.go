// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/runcloud/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/runcloud/internal/cliutil"
)

type fleetHealthRow struct {
	ServerID      string  `json:"server_id"`
	ServerName    string  `json:"server_name,omitempty"`
	Uptime        string  `json:"uptime,omitempty"`
	Load          string  `json:"load,omitempty"`
	DiskUsedPct   float64 `json:"disk_used_pct,omitempty"`
	MemoryUsedPct float64 `json:"memory_used_pct,omitempty"`
	Error         string  `json:"error,omitempty"`
}

func newFleetHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Fetch uptime, load, disk, and memory for every server in one table",
		Example:     `  runcloud-pp-cli fleet health`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would fan out /servers/{id}/health/latest across the fleet)")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []fleetHealthRow{}, flags)
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []fleetHealthRow{}, flags)
			}

			meta, err := loadServerMeta(db)
			if err != nil {
				return err
			}
			ids := make([]string, 0, len(meta))
			for id := range meta {
				ids = append(ids, id)
			}
			// Curtail under live-dogfood: bounded sample, real API calls.
			if cliutil.IsDogfoodEnv() && len(ids) > 3 {
				ids = ids[:3]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results, errs := cliutil.FanoutRun(ctx, ids,
				func(id string) string { return id },
				func(ctx context.Context, id string) (fleetHealthRow, error) {
					return fetchServerHealth(c, id, meta[id])
				},
			)

			out := make([]fleetHealthRow, 0, len(results)+len(errs))
			for _, r := range results {
				out = append(out, r.Value)
			}
			for _, e := range errs {
				out = append(out, fleetHealthRow{
					ServerID:   e.Source,
					ServerName: meta[e.Source],
					Error:      e.Err.Error(),
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func fetchServerHealth(c *client.Client, id, name string) (fleetHealthRow, error) {
	path := fmt.Sprintf("/servers/%s/health/latest", id)
	raw, err := c.Get(path, nil)
	if err != nil {
		return fleetHealthRow{ServerID: id, ServerName: name}, err
	}
	row := fleetHealthRow{ServerID: id, ServerName: name}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		// Common health-payload shapes; tolerate missing keys.
		body := obj
		if d, ok := obj["data"].(map[string]any); ok {
			body = d
		}
		if v, ok := body["uptime"].(string); ok {
			row.Uptime = v
		} else if v, ok := body["uptime"].(float64); ok {
			row.Uptime = fmt.Sprintf("%g", v)
		}
		if v, ok := body["load"].(string); ok {
			row.Load = v
		} else if v, ok := body["load"].(float64); ok {
			row.Load = fmt.Sprintf("%g", v)
		}
		row.DiskUsedPct = floatField(body, "diskUsedPct", "disk_used_pct", "disk.used_pct")
		row.MemoryUsedPct = floatField(body, "memoryUsedPct", "memory_used_pct", "memory.used_pct")
	}
	return row, nil
}

func floatField(obj map[string]any, keys ...string) float64 {
	for _, k := range keys {
		// Dotted lookup support
		v := lookupDottedAny(obj, k)
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		}
	}
	return 0
}

// Silence unused-import warning for sql when the file evolves.
var _ = sql.ErrNoRows
