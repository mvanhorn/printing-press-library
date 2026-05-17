// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/store"

	"github.com/spf13/cobra"
)

func newBulkCmd(rflags *rootFlags) *cobra.Command {
	var (
		typeFilter string
		roomFilter string
		all        bool
		action     string
	)
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Fan-out a control command across every device matching a filter",
		Long: `Send one control action to every Dreo device matching --type and/or --room.
Opens one WebSocket connection and fires N frames in parallel.

Actions: on, off, sleep, turbo, auto
Filters: --type (model prefix or category, e.g. "HTF", "tower-fan", "purifier"),
         --room (case-insensitive substring match),
         --all  (no filter)

Examples:
  dreo-pp-cli bulk --type tower-fan --action off
  dreo-pp-cli bulk --room bedroom --action sleep
  dreo-pp-cli bulk --all --action off`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if action == "" {
				if len(args) >= 1 {
					action = args[0]
				} else {
					return cmd.Help()
				}
			}
			if !all && typeFilter == "" && roomFilter == "" {
				return usageErr(errors.New("bulk: provide --type and/or --room, or pass --all"))
			}
			params, err := bulkActionParams(action)
			if err != nil {
				return usageErr(err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			devs, err := listCachedOrFetch(ctx, rflags, false)
			if err != nil {
				return err
			}
			matched := filterDevices(devs, typeFilter, roomFilter)
			if len(matched) == 0 {
				if rflags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"matched": 0,
						"action":  action,
					}, rflags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No devices matched the filter.\n")
				return nil
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would bulk %s across %d devices\n", action, len(matched))
				return nil
			}
			if dryRunOK(rflags) {
				out := map[string]any{
					"action":  action,
					"params":  params,
					"matched": len(matched),
					"devices": deviceSummaries(matched),
					"dry_run": true,
				}
				if rflags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would send %v to %d devices:\n", params, len(matched))
				for _, d := range matched {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s (%s)\n", d.Name, d.Sn, d.Model)
				}
				return nil
			}

			wsConn, err := connectWS(ctx, rflags)
			if err != nil {
				return apiErr(fmt.Errorf("bulk: open WS: %w", err))
			}
			defer wsConn.Close()

			var wg sync.WaitGroup
			results := make([]map[string]any, len(matched))
			for i, d := range matched {
				wg.Add(1)
				go func(i int, d store.Device) {
					defer wg.Done()
					err := wsConn.Send(d.Sn, params)
					r := map[string]any{
						"sn":   d.Sn,
						"name": d.Name,
						"ok":   err == nil,
					}
					if err != nil {
						r["error"] = err.Error()
					}
					results[i] = r
				}(i, d)
			}
			wg.Wait()

			okCount := 0
			for _, r := range results {
				if r["ok"] == true {
					okCount++
				}
			}

			out := map[string]any{
				"action":  action,
				"params":  params,
				"matched": len(matched),
				"sent":    okCount,
				"results": results,
			}
			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bulk %s: %d/%d devices succeeded\n", action, okCount, len(matched))
			for _, r := range results {
				name, _ := r["name"].(string)
				if r["ok"] == true {
					fmt.Fprintf(cmd.OutOrStdout(), "  ok    %s\n", name)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  FAIL  %s: %v\n", name, r["error"])
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by model prefix or category (HTF, tower-fan, purifier, humidifier, heater, ac)")
	cmd.Flags().StringVar(&roomFilter, "room", "", "Filter by room name (case-insensitive substring)")
	cmd.Flags().BoolVar(&all, "all", false, "Apply to all devices (no filter)")
	cmd.Flags().StringVar(&action, "action", "", "Action: on|off|sleep|turbo|auto")
	return cmd
}

func bulkActionParams(action string) (map[string]any, error) {
	switch strings.ToLower(action) {
	case "on":
		return map[string]any{"poweron": true}, nil
	case "off":
		return map[string]any{"poweron": false}, nil
	case "sleep":
		return map[string]any{"poweron": true, "windmode": 3, "windtype": 3}, nil
	case "turbo":
		return map[string]any{"poweron": true, "windmode": 5, "windtype": 5}, nil
	case "auto":
		return map[string]any{"poweron": true, "windmode": 4, "windtype": 4}, nil
	}
	return nil, fmt.Errorf("bulk: unknown action %q (use on|off|sleep|turbo|auto)", action)
}

func filterDevices(devs []store.Device, typeFilter, roomFilter string) []store.Device {
	out := make([]store.Device, 0, len(devs))
	tf := strings.TrimSpace(typeFilter)
	rf := strings.ToLower(strings.TrimSpace(roomFilter))
	for _, d := range devs {
		if tf != "" && !matchesDeviceType(d, tf) {
			continue
		}
		if rf != "" && !strings.Contains(strings.ToLower(d.Room), rf) {
			continue
		}
		out = append(out, d)
	}
	return out
}

func deviceSummaries(devs []store.Device) []map[string]any {
	out := make([]map[string]any, 0, len(devs))
	for _, d := range devs {
		out = append(out, map[string]any{
			"sn":    d.Sn,
			"name":  d.Name,
			"model": d.Model,
			"room":  d.Room,
		})
	}
	return out
}
