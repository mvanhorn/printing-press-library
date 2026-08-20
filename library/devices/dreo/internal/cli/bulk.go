// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

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

			// PATCH(greptile/641): sleep/turbo/auto send fan-specific
			// windmode/windtype frames that non-fan devices silently ignore.
			// Partition matched devices and refuse the action when every
			// match is a non-fan, so users get a loud failure instead of
			// a fake success.
			applicable, skipped := partitionForAction(matched, action)
			if len(applicable) == 0 {
				return usageErr(fmt.Errorf("bulk: action %q is fan-only; %d matched device(s) are non-fan (use --type fan, or pick on|off)", action, len(skipped)))
			}

			if cliutil.IsVerifyEnv() {
				if len(skipped) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "would bulk %s across %d devices (%d non-fan skipped)\n", action, len(applicable), len(skipped))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "would bulk %s across %d devices\n", action, len(applicable))
				}
				return nil
			}
			if dryRunOK(rflags) {
				out := map[string]any{
					"action":      action,
					"params":      params,
					"matched":     len(matched),
					"applicable":  len(applicable),
					"skipped":     len(skipped),
					"devices":     deviceSummaries(applicable),
					"skipped_for": deviceSummaries(skipped),
					"dry_run":     true,
				}
				if rflags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would send %v to %d devices:\n", params, len(applicable))
				for _, d := range applicable {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s (%s)\n", d.Name, d.Sn, d.Model)
				}
				if len(skipped) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipped (action %q is fan-only):\n", action)
					for _, d := range skipped {
						fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s (%s)\n", d.Name, d.Sn, d.Model)
					}
				}
				return nil
			}

			wsConn, err := connectWS(ctx, rflags)
			if err != nil {
				return apiErr(fmt.Errorf("bulk: open WS: %w", err))
			}
			defer wsConn.Close()

			var wg sync.WaitGroup
			sentResults := make([]map[string]any, len(applicable))
			for i, d := range applicable {
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
					sentResults[i] = r
				}(i, d)
			}
			wg.Wait()

			results := make([]map[string]any, 0, len(applicable)+len(skipped))
			results = append(results, sentResults...)
			for _, d := range skipped {
				results = append(results, map[string]any{
					"sn":      d.Sn,
					"name":    d.Name,
					"ok":      false,
					"skipped": true,
					"reason":  fmt.Sprintf("action %q is fan-only; %s is not a fan", action, d.Model),
				})
			}

			okCount := 0
			for _, r := range sentResults {
				if r["ok"] == true {
					okCount++
				}
			}

			out := map[string]any{
				"action":  action,
				"params":  params,
				"matched": len(matched),
				"sent":    okCount,
				"skipped": len(skipped),
				"results": results,
			}
			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
			}
			if len(skipped) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Bulk %s: %d/%d devices succeeded (%d non-fan skipped)\n", action, okCount, len(applicable), len(skipped))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Bulk %s: %d/%d devices succeeded\n", action, okCount, len(applicable))
			}
			for _, r := range results {
				name, _ := r["name"].(string)
				switch {
				case r["skipped"] == true:
					fmt.Fprintf(cmd.OutOrStdout(), "  skip  %s: %v\n", name, r["reason"])
				case r["ok"] == true:
					fmt.Fprintf(cmd.OutOrStdout(), "  ok    %s\n", name)
				default:
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

// fanOnlyBulkActions encode windmode/windtype frames that only fan
// firmware acts on; sending them to heaters/purifiers/humidifiers/ACs
// is a no-op the WebSocket accepts without error.
var fanOnlyBulkActions = map[string]bool{"sleep": true, "turbo": true, "auto": true}

// isFanModel reports whether a Dreo model code identifies a fan, which
// is the only family that responds to the windmode/windtype frames used
// by sleep/turbo/auto.
func isFanModel(model string) bool {
	m := strings.ToUpper(model)
	return strings.HasPrefix(m, "HTF") || strings.HasPrefix(m, "HPF") ||
		strings.HasPrefix(m, "HCF") || strings.HasPrefix(m, "HSH")
}

// partitionForAction splits matched devices into ones the action will
// actually affect and ones it would silently no-op against. For non-fan-
// only actions every device is applicable.
func partitionForAction(matched []store.Device, action string) (applicable, skipped []store.Device) {
	if !fanOnlyBulkActions[strings.ToLower(action)] {
		return matched, nil
	}
	for _, d := range matched {
		if isFanModel(d.Model) {
			applicable = append(applicable, d)
		} else {
			skipped = append(skipped, d)
		}
	}
	return applicable, skipped
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
