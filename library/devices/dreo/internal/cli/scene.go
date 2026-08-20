// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"

	"github.com/spf13/cobra"
)

func newSceneCmd(rflags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scene",
		Short: "Save and replay named device-state scenes",
		Long: `Snapshot the current state across selected devices as a named scene,
then replay it later with one command.

Subcommands:
  save <name>   Capture current state for matching devices.
  apply <name>  Replay a scene by fan-out WebSocket frames.
  list          List saved scenes.`,
		RunE: parentNoSubcommandRunE(rflags),
	}
	cmd.AddCommand(newSceneSaveCmd(rflags))
	cmd.AddCommand(newSceneApplyCmd(rflags))
	cmd.AddCommand(newSceneListCmd(rflags))
	return cmd
}

func newSceneSaveCmd(rflags *rootFlags) *cobra.Command {
	var (
		typeFilter string
		roomFilter string
		all        bool
	)
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Snapshot current device state into a named scene",
		Example: `  dreo-pp-cli scene save evening --room bedroom
  dreo-pp-cli scene save night --all`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := args[0]
			if !all && typeFilter == "" && roomFilter == "" {
				return usageErr(errors.New("scene save: provide --type, --room, or --all"))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would save scene %q\n", name)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			devs, err := listCachedOrFetch(ctx, rflags, false)
			if err != nil {
				return err
			}
			matched := filterDevices(devs, typeFilter, roomFilter)
			if len(matched) == 0 {
				return notFoundErr(errors.New("no devices matched filter"))
			}

			if dryRunOK(rflags) {
				out := map[string]any{
					"name":    name,
					"matched": len(matched),
					"devices": deviceSummaries(matched),
					"dry_run": true,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
			}

			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			snapshots := map[string]map[string]any{}
			for _, d := range matched {
				raw, _, err := st.GetDeviceState(ctx, d.Sn)
				if err != nil {
					continue
				}
				var m map[string]any
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				snapshots[d.Sn] = extractSceneFields(m)
			}
			if len(snapshots) == 0 {
				return notFoundErr(errors.New("no cached device_state to snapshot; run `dreo-pp-cli sensors --live` first to populate the state cache"))
			}
			// Partial captures are a real correctness hazard: silently
			// dropping unsynced devices means a later `scene apply` only
			// controls a subset of the matched filter and the user has no
			// way to tell. Warn (stderr) when this happens, name the
			// command that refreshes the cache, and surface the dropped
			// count in JSON output too.
			skipped := len(matched) - len(snapshots)
			if skipped > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d/%d matched device(s) had no cached state and were omitted from scene %q (run `dreo-pp-cli sensors --live` to refresh, then retry)\n",
					skipped, len(matched), name)
			}
			if err := st.SaveScene(ctx, name, snapshots); err != nil {
				return err
			}
			if rflags.asJSON {
				out := map[string]any{
					"saved":   name,
					"devices": len(snapshots),
				}
				if skipped > 0 {
					out["matched"] = len(matched)
					out["skipped_uncached"] = skipped
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved scene %q with %d devices.\n", name, len(snapshots))
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by model prefix or category")
	cmd.Flags().StringVar(&roomFilter, "room", "", "Filter by room")
	cmd.Flags().BoolVar(&all, "all", false, "Capture every device")
	return cmd
}

func newSceneApplyCmd(rflags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "apply <name>",
		Short:       "Replay a saved scene as parallel WebSocket frames",
		Example:     "  dreo-pp-cli scene apply evening",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := args[0]

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			scene, err := st.LoadScene(ctx, name)
			if err != nil {
				return notFoundErr(fmt.Errorf("scene %q: %w", name, err))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would apply scene %q to %d devices\n", name, len(scene))
				return nil
			}
			if dryRunOK(rflags) {
				out := map[string]any{
					"name":      name,
					"devices":   len(scene),
					"snapshots": scene,
					"dry_run":   true,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
			}

			wsConn, err := connectWS(ctx, rflags)
			if err != nil {
				return apiErr(fmt.Errorf("scene apply: open WS: %w", err))
			}
			defer wsConn.Close()

			var wg sync.WaitGroup
			results := make([]map[string]any, 0, len(scene))
			var mu sync.Mutex
			for sn, fields := range scene {
				wg.Add(1)
				go func(sn string, fields map[string]any) {
					defer wg.Done()
					err := wsConn.Send(sn, fields)
					mu.Lock()
					results = append(results, map[string]any{
						"sn": sn,
						"ok": err == nil,
					})
					mu.Unlock()
				}(sn, fields)
			}
			wg.Wait()

			okCount := 0
			for _, r := range results {
				if r["ok"] == true {
					okCount++
				}
			}
			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"name":    name,
					"sent":    okCount,
					"total":   len(scene),
					"results": results,
				}, rflags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Applied scene %q: %d/%d frames sent.\n", name, okCount, len(scene))
			return nil
		},
	}
	return cmd
}

func newSceneListCmd(rflags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved scenes (name, device count, created timestamp)",
		Example: `  dreo-pp-cli scene list
  dreo-pp-cli scene list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			scenes, err := st.ListScenes(ctx)
			if err != nil {
				return err
			}
			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), scenes, rflags)
			}
			if len(scenes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved scenes.")
				return nil
			}
			headers := []string{"NAME", "DEVICES", "SAVED"}
			rows := make([][]string, 0, len(scenes))
			for _, s := range scenes {
				rows = append(rows, []string{
					s.Name,
					fmt.Sprintf("%d", s.Devices),
					s.SavedAt.Format(time.RFC3339),
				})
			}
			return rflags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}

// extractSceneFields keeps the controllable subset of a state snapshot.
// This avoids replaying read-only sensor values like temperature.
func extractSceneFields(state map[string]any) map[string]any {
	wanted := []string{
		"poweron", "windlevel", "windmode", "windtype", "oscmode",
		"shakehorizon", "shakehorizonangle", "htalevel",
		"coolair", "hotair", "eco",
		"targetTemperature", "targetHumidity", "tempunit",
		"foglevel", "mistlevel", "direction",
		"lighton", "lightlevel", "colortemp", "rgbcolor", "rgbmode", "rgblevel",
		"childlockon", "displayon", "voiceon", "muteon", "ledalwayson",
	}
	out := map[string]any{}
	for _, k := range wanted {
		if v, ok := state[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}
