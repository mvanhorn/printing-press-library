// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

func newDriftCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drift [project-id]",
		Short: "Detect deployment configuration drift",
		Long: `Compare current live deployment against cached configuration to spot
unexpected changes in environment variables, build settings, or domain config.

Requires synced data for comparison - run 'sync' first to populate local cache,
then run 'drift' to compare live state against the cached baseline.`,
		Example: `  vibecode-pp-cli drift proj_abc123
  vibecode-pp-cli drift proj_abc123 --json`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare live deployment against cached baseline")
				return nil
			}

			projectID := args[0]

			// Get cached deployment from local store
			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var cachedData string
			err = db.DB().QueryRowContext(cmd.Context(), `
				SELECT data FROM resources
				WHERE resource_type IN ('deployments', 'projects_deployments')
				  AND id = ?
			`, projectID).Scan(&cachedData)
			if err != nil {
				return fmt.Errorf("no cached deployment found for %s; run 'sync' first", projectID)
			}

			var cachedDeployment map[string]any
			if err := json.Unmarshal([]byte(cachedData), &cachedDeployment); err != nil {
				return fmt.Errorf("parsing cached deployment: %w", err)
			}

			// Fetch live deployment from API
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			liveData, err := c.Get(cmd.Context(), fmt.Sprintf("/deployments/%s", projectID), nil)
			if err != nil {
				return fmt.Errorf("fetching live deployment: %w", err)
			}

			var liveDeployment map[string]any
			if err := json.Unmarshal(liveData, &liveDeployment); err != nil {
				return fmt.Errorf("parsing live deployment: %w", err)
			}

			// Compare the two
			diffs := compareMaps("", cachedDeployment, liveDeployment)

			type driftResult struct {
				ProjectID  string `json:"project_id"`
				HasDrift   bool   `json:"has_drift"`
				DriftCount int    `json:"drift_count"`
				Diffs      []diff `json:"diffs,omitempty"`
			}

			result := driftResult{
				ProjectID:  projectID,
				HasDrift:   len(diffs) > 0,
				DriftCount: len(diffs),
				Diffs:      diffs,
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}

			if !result.HasDrift {
				fmt.Fprintf(cmd.OutOrStdout(), "No drift detected for %s\n", projectID)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Drift detected for %s (%d changes):\n\n", projectID, result.DriftCount)
			headers := []string{"Field", "Cached Value", "Live Value"}
			var tableRows [][]string
			for _, d := range diffs {
				tableRows = append(tableRows, []string{
					d.Path,
					truncateDrift(fmt.Sprintf("%v", d.CachedValue), 30),
					truncateDrift(fmt.Sprintf("%v", d.LiveValue), 30),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	return cmd
}

type diff struct {
	Path        string `json:"path"`
	CachedValue any    `json:"cached_value"`
	LiveValue   any    `json:"live_value"`
}

// compareMaps recursively compares two maps and returns differences
func compareMaps(prefix string, cached, live map[string]any) []diff {
	var diffs []diff

	// Check all keys in cached
	for k, cachedV := range cached {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}

		liveV, exists := live[k]
		if !exists {
			diffs = append(diffs, diff{Path: path, CachedValue: cachedV, LiveValue: nil})
			continue
		}

		// Recurse for nested maps
		cachedMap, cachedIsMap := cachedV.(map[string]any)
		liveMap, liveIsMap := liveV.(map[string]any)
		if cachedIsMap && liveIsMap {
			diffs = append(diffs, compareMaps(path, cachedMap, liveMap)...)
			continue
		}

		// Compare values
		if !reflect.DeepEqual(cachedV, liveV) {
			diffs = append(diffs, diff{Path: path, CachedValue: cachedV, LiveValue: liveV})
		}
	}

	// Check for new keys in live
	for k, liveV := range live {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if _, exists := cached[k]; !exists {
			diffs = append(diffs, diff{Path: path, CachedValue: nil, LiveValue: liveV})
		}
	}

	return diffs
}

func truncateDrift(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
