// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/store"

	"github.com/spf13/cobra"
)

// degradedSnapshotPath stores the previous degraded-set for diffing.
func degradedSnapshotPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "openrouter-pp-cli", "providers-prev.json")
}

func newProvidersDegradedCmd(flags *rootFlags) *cobra.Command {
	var llm bool

	cmd := &cobra.Command{
		Use:         "degraded",
		Short:       "Show currently-degraded provider endpoints (status != 0) with set-diff vs prior snapshot",
		Example:     "  openrouter-pp-cli providers degraded --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "{\"degraded\":[],\"added\":[],\"removed\":[]}")
				return nil
			}
			dbPath := defaultDBPath("openrouter-pp-cli")
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("open store: %w", err))
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT provider_name, model_id, status, COALESCE(uptime_last_30m, -1) FROM endpoints WHERE status != 0`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			type degraded struct {
				Provider string  `json:"provider"`
				Model    string  `json:"model"`
				Status   int     `json:"status"`
				Uptime30 float64 `json:"uptime_last_30m"`
			}
			cur := []degraded{}
			curKeys := map[string]bool{}
			for rows.Next() {
				var d degraded
				var up float64
				if err := rows.Scan(&d.Provider, &d.Model, &d.Status, &up); err != nil {
					continue
				}
				d.Uptime30 = up
				cur = append(cur, d)
				curKeys[d.Provider+"|"+d.Model] = true
			}
			sort.Slice(cur, func(i, j int) bool {
				if cur[i].Provider != cur[j].Provider {
					return cur[i].Provider < cur[j].Provider
				}
				return cur[i].Model < cur[j].Model
			})

			// Read prev snapshot.
			prevKeys := map[string]bool{}
			snapPath := degradedSnapshotPath()
			if raw, err := os.ReadFile(snapPath); err == nil {
				var prev []degraded
				if json.Unmarshal(raw, &prev) == nil {
					for _, p := range prev {
						prevKeys[p.Provider+"|"+p.Model] = true
					}
				}
			}
			added := []string{}
			removed := []string{}
			for k := range curKeys {
				if !prevKeys[k] {
					added = append(added, k)
				}
			}
			for k := range prevKeys {
				if !curKeys[k] {
					removed = append(removed, k)
				}
			}
			sort.Strings(added)
			sort.Strings(removed)

			// Persist current snapshot.
			_ = os.MkdirAll(filepath.Dir(snapPath), 0o755)
			if buf, err := json.Marshal(cur); err == nil {
				_ = os.WriteFile(snapPath, buf, 0o644)
			}

			result := map[string]any{
				"degraded": cur,
				"added":    added,
				"removed":  removed,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if llm {
				if len(cur) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no degraded endpoints")
					return nil
				}
				for _, d := range cur {
					fmt.Fprintf(cmd.OutOrStdout(), "provider=%s model=%s status=%d uptime_last_30m=%.2f\n",
						d.Provider, d.Model, d.Status, d.Uptime30)
				}
				return nil
			}
			if len(cur) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no degraded endpoints")
				return nil
			}
			rs := make([][]string, 0, len(cur))
			for _, d := range cur {
				rs = append(rs, []string{d.Provider, d.Model, fmt.Sprintf("%d", d.Status), fmt.Sprintf("%.2f", d.Uptime30)})
			}
			return flags.printTable(cmd, []string{"PROVIDER", "MODEL", "STATUS", "UPTIME_30M"}, rs)
		},
	}
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output")
	return cmd
}
