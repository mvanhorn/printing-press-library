// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// pp:client-call through eiaFetch in eia_novel_support.go

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelRevisionsCmd(flags *rootFlags) *cobra.Command {
	var route, facet, data, frequency string
	var hours int
	cmd := &cobra.Command{
		Use:         "revisions",
		Short:       "Compare a bounded route/facet snapshot with the prior local observation and report changed period-value-unit keys.",
		Example:     "  eia-energy-pp-cli revisions --route electricity/rto/region-data --facet respondent=CISO --hours 24 --agent",
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:destructive": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return err
			}
			if hours < 1 || hours > 720 {
				return fmt.Errorf("--hours must be 1-720")
			}
			facetName, facetValue, err := parseFacet(facet)
			if err != nil {
				return err
			}
			end := time.Now().UTC().Truncate(time.Hour)
			start := end.Add(-time.Duration(hours-1) * time.Hour)
			params := seriesParams(data, frequency, start.Format("2006-01-02T15"), end.Format("2006-01-02T15"), map[string][]string{facetName: {facetValue}}, 5000)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			page, err := eiaFetch(ctx, flags, route, params)
			if err != nil {
				return err
			}
			if err := requireCompletePage(page, "revision comparison"); err != nil {
				return err
			}
			current := map[string]string{}
			for _, row := range page.Rows {
				key := fmt.Sprintf("%v|%v|%s", row["period"], row["type"], rowUnit(row, data))
				current[key] = fmt.Sprint(row[data])
			}
			queryID := fmt.Sprintf("%s|%s|%s|%s|hours=%d", route, facet, data, frequency, hours)
			sum := sha256.Sum256([]byte(queryID))
			configDir, err := os.UserConfigDir()
			if err != nil {
				return err
			}
			dir := filepath.Join(configDir, "eia-energy-pp-cli", "revisions")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			file := filepath.Join(dir, hex.EncodeToString(sum[:12])+".json")
			previous := map[string]string{}
			// #nosec G304 -- file is derived from UserConfigDir plus a SHA-256 query key.
			if raw, readErr := os.ReadFile(file); readErr == nil {
				_ = json.Unmarshal(raw, &previous)
			} else if !os.IsNotExist(readErr) {
				return readErr
			}
			added, changed, missing := []map[string]any{}, []map[string]any{}, []map[string]any{}
			for key, value := range current {
				old, found := previous[key]
				if !found {
					added = append(added, map[string]any{"key": key, "value": value})
				} else if old != value {
					changed = append(changed, map[string]any{"key": key, "previous": old, "current": value})
				}
			}
			for key, value := range previous {
				if _, found := current[key]; !found {
					missing = append(missing, map[string]any{"key": key, "previous": value})
				}
			}
			raw, err := json.MarshalIndent(current, "", "  ")
			if err != nil {
				return err
			}
			tmp := file + ".tmp"
			if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
				return err
			}
			if err := os.Rename(tmp, file); err != nil {
				return err
			}
			return emitEIA(cmd, flags, "mixed", map[string]any{"query": queryID, "window_hours": hours, "total": page.Total, "returned_rows": len(page.Rows), "first_observation": len(previous) == 0, "added": added, "changed": changed, "missing_from_current_window": missing, "snapshot_path": strings.ReplaceAll(file, configDir, "$CONFIG"), "source_rows": page.Rows, "caveats": eiaCaveats()})
		},
	}
	cmd.Flags().StringVar(&route, "route", "electricity/rto/region-data", "EIA v2 data route")
	cmd.Flags().StringVar(&facet, "facet", "respondent=CISO", "Single facet as NAME=VALUE")
	cmd.Flags().StringVar(&data, "data", "value", "Data column")
	cmd.Flags().StringVar(&frequency, "frequency", "hourly", "EIA frequency")
	cmd.Flags().IntVar(&hours, "hours", 24, "Trailing hourly window (1-720)")
	return cmd
}
