// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newScenariosListAllTeamsCmd registers `scenarios list-all` as a sibling of
// the standard `scenarios list` (which requires a single teamId). The hyphenated
// command name avoids clashing with the generated list endpoint while
// preserving the `--all-teams` semantic the absorb manifest committed to.
func newScenariosListAllTeamsCmd(flags *rootFlags) *cobra.Command {
	var flagActive bool
	var flagStale time.Duration
	var flagFolder string

	cmd := &cobra.Command{
		Use:     "list-all",
		Aliases: []string{"all-teams", "across-teams"},
		Short:   "Union scenarios across every team the token can see (Make's API is one-team-per-call; the local mirror is not)",
		Example: strings.Trim(`
  make-pp-cli scenarios list-all --active --stale 720h --json --select rows.id,rows.name,rows.team,rows.lastEdit
  make-pp-cli scenarios list-all --folder "Marketing Statistics Automations"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			teamIDs, err := allVisibleTeamIDs(ctx, c)
			if err != nil {
				return err
			}
			staleCutoff := time.Time{}
			if flagStale > 0 {
				staleCutoff = time.Now().Add(-flagStale)
			}

			type row struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				TeamID     int64  `json:"teamId"`
				FolderID   int64  `json:"folderId,omitempty"`
				IsActive   bool   `json:"isActive"`
				IsPaused   bool   `json:"isPaused"`
				LastEdit   string `json:"lastEdit,omitempty"`
				NextExec   string `json:"nextExec,omitempty"`
				DlqCount   int    `json:"dlqCount"`
				FolderName string `json:"folderName,omitempty"`
			}
			var rows []row
			folderNames := map[int64]string{}
			for _, tid := range teamIDs {
				if folders, err := c.Get(ctx, "/scenarios-folders", map[string]string{"teamId": fmt.Sprintf("%d", tid)}); err == nil {
					var wrap struct {
						ScenariosFolders []map[string]any `json:"scenariosFolders"`
					}
					if json.Unmarshal(folders, &wrap) == nil {
						for _, f := range wrap.ScenariosFolders {
							fid := int64(asFloat(f["id"]))
							if fid != 0 {
								folderNames[fid] = stringOf(f["name"])
							}
						}
					}
				}
				scenarios, err := listScenarios(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list scenarios for team %d failed: %v\n", tid, err)
					continue
				}
				for _, s := range scenarios {
					sid := int64(asFloat(s["id"]))
					if sid == 0 {
						continue
					}
					isActive, _ := s["isActive"].(bool)
					if flagActive && !isActive {
						continue
					}
					lastEditStr, _ := s["lastEdit"].(string)
					if !staleCutoff.IsZero() && lastEditStr != "" {
						if t, err := time.Parse(time.RFC3339Nano, lastEditStr); err == nil && t.After(staleCutoff) {
							continue
						}
					}
					folderID := int64(asFloat(s["folderId"]))
					folderName := folderNames[folderID]
					if flagFolder != "" && !strings.EqualFold(folderName, flagFolder) {
						continue
					}
					isPaused, _ := s["isPaused"].(bool)
					rows = append(rows, row{
						ID:         sid,
						Name:       stringOf(s["name"]),
						TeamID:     tid,
						FolderID:   folderID,
						IsActive:   isActive,
						IsPaused:   isPaused,
						LastEdit:   lastEditStr,
						NextExec:   stringOf(s["nextExec"]),
						DlqCount:   int(asFloat(s["dlqCount"])),
						FolderName: folderName,
					})
				}
			}
			out := map[string]any{
				"teamsScanned":   len(teamIDs),
				"totalScenarios": len(rows),
				"rows":           rows,
			}
			b, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().BoolVar(&flagActive, "active", false, "Only include scenarios where isActive=true")
	cmd.Flags().DurationVar(&flagStale, "stale", 0, "Only include scenarios whose lastEdit is older than this duration (e.g. 720h for 30 days)")
	cmd.Flags().StringVar(&flagFolder, "folder", "", "Filter to scenarios in a folder by name (case-insensitive)")
	return cmd
}
