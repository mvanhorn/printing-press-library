// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelBlueprintSyncCmd(flags *rootFlags) *cobra.Command {
	var flagRepo string
	var flagTeam string
	var flagAllTeams bool
	var flagKeepMetadata bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror every scenario's blueprint into a local repo as canonical JSON (one file per scenario)",
		Example: strings.Trim(`
  make-pp-cli blueprint sync --repo ./make-blueprints --all-teams
  make-pp-cli blueprint sync --repo ./make-blueprints --team 588013 --keep-metadata
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagRepo == "" {
				return usageErr(fmt.Errorf("--repo is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			teamIDs, err := teamIDsFromFlags(ctx, c, flagTeam, flagAllTeams)
			if err != nil {
				return err
			}
			if len(teamIDs) == 0 {
				return usageErr(fmt.Errorf("specify --team <id> or --all-teams"))
			}

			if err := os.MkdirAll(flagRepo, 0o755); err != nil {
				return fmt.Errorf("creating --repo %q: %w", flagRepo, err)
			}

			var written []map[string]any
			for _, tid := range teamIDs {
				scenarios, err := listScenarios(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list scenarios for team %d failed: %v\n", tid, err)
					continue
				}
				teamDir := filepath.Join(flagRepo, fmt.Sprintf("team-%d", tid))
				if err := os.MkdirAll(teamDir, 0o755); err != nil {
					return fmt.Errorf("creating team dir: %w", err)
				}
				for _, s := range scenarios {
					sid := int64(asFloat(s["id"]))
					if sid == 0 {
						continue
					}
					name := stringOf(s["name"])
					slug := slugify(name)
					if slug == "" {
						slug = fmt.Sprintf("scenario-%d", sid)
					}
					bp, err := getBlueprint(ctx, c, sid)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warn: blueprint scenario %d failed: %v\n", sid, err)
						continue
					}
					base := filepath.Join(teamDir, fmt.Sprintf("%d-%s", sid, slug))
					canonical, err := canonicalBlueprintJSON(bp, flagKeepMetadata)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warn: canonicalize scenario %d failed: %v\n", sid, err)
						continue
					}
					if err := os.WriteFile(base+".blueprint.json", canonical, 0o644); err != nil {
						return fmt.Errorf("writing blueprint: %w", err)
					}
					// Sidecar: original blueprint, scenario metadata.
					meta, _ := json.MarshalIndent(s, "", "  ")
					if err := os.WriteFile(base+".scenario.json", meta, 0o644); err != nil {
						return fmt.Errorf("writing scenario metadata: %w", err)
					}
					if !flagKeepMetadata {
						// Also write a sidecar with the raw blueprint so nothing is lost.
						rawIndent, _ := json.MarshalIndent(json.RawMessage(bp), "", "  ")
						_ = os.WriteFile(base+".blueprint.raw.json", rawIndent, 0o644)
					}
					written = append(written, map[string]any{
						"scenarioId": sid,
						"teamId":     tid,
						"name":       name,
						"path":       base + ".blueprint.json",
					})
				}
			}

			summary := map[string]any{
				"repo":              flagRepo,
				"teamsScanned":      len(teamIDs),
				"blueprintsWritten": len(written),
				"files":             written,
			}
			b, _ := json.Marshal(summary)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagRepo, "repo", "", "Local directory to write blueprints into (created if missing)")
	cmd.Flags().StringVar(&flagTeam, "team", "", "Team ID (omit to require --all-teams)")
	cmd.Flags().BoolVar(&flagAllTeams, "all-teams", false, "Sync blueprints across every team the token can see")
	cmd.Flags().BoolVar(&flagKeepMetadata, "keep-metadata", false, "Preserve metadata.expect/restore/designer blocks (default strips them for cleaner diffs)")
	return cmd
}

var slugifyNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugifyNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}
