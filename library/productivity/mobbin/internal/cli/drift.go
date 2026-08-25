// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newDriftCmd(flags *rootFlags) *cobra.Command {
	var since, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "drift <app-slug>",
		Short:       "Diff an app's oldest and newest local snapshots in a time window.",
		Example:     "  mobbin-pp-cli drift stripe-web --since 30d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseSince(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			db, err := openStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshalJSON(noSnapshotsResult(args[0], since)), flags)
			}
			defer db.Close()
			cutoff := time.Now().Add(-d).UTC().Format(time.RFC3339)
			appExpr := "(apps.slug=" + sqlQuote(args[0]) + " OR apps.slug LIKE " + sqlQuote("%"+args[0]+"%") + ")"
			q := `SELECT app_versions.id, app_versions.captured_at
FROM app_versions JOIN apps ON apps.id=app_versions.app_id
WHERE ` + appExpr + ` AND app_versions.captured_at >= ` + sqlQuote(cutoff) + ` ORDER BY app_versions.captured_at ASC LIMIT ` + fmt.Sprint(limit)
			snaps, err := db.RawQuery(cmd.Context(), q)
			if err != nil {
				return err
			}
			if len(snaps) == 0 {
				// Empty local stores are a successful no-diff state for agents.
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshalJSON(noSnapshotsResult(args[0], since)), flags)
			}
			if len(snaps) == 1 {
				return flags.printJSON(cmd, map[string]any{"snapshots": 1, "diff": nil})
			}
			oldID, newID := fmt.Sprint(snaps[0]["id"]), fmt.Sprint(snaps[len(snaps)-1]["id"])
			oldSet, err := idsForVersion(cmd.Context(), db, oldID)
			if err != nil {
				return err
			}
			newSet, err := idsForVersion(cmd.Context(), db, newID)
			if err != nil {
				return err
			}
			added, removed := diffSets(oldSet, newSet)
			return flags.printJSON(cmd, map[string]any{"snapshots": len(snaps), "oldest": oldID, "newest": newID, "added": added, "removed": removed, "changed_count": len(added) + len(removed)})
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window duration, e.g. 30d")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum snapshots to compare")
	return cmd
}

func idsForVersion(ctx context.Context, db interface {
	RawQuery(context.Context, string) ([]map[string]any, error)
}, version string) (map[string]bool, error) {
	rows, err := db.RawQuery(ctx, `SELECT 'screen:' || id AS id FROM screens WHERE app_version_id=`+sqlQuote(version)+` UNION SELECT 'flow:' || id AS id FROM flows WHERE app_version_id=`+sqlQuote(version))
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[fmt.Sprint(r["id"])] = true
	}
	return out, nil
}

func diffSets(a, b map[string]bool) ([]string, []string) {
	added, removed := []string{}, []string{}
	for id := range b {
		if !a[id] {
			added = append(added, strings.TrimPrefix(strings.TrimPrefix(id, "screen:"), "flow:"))
		}
	}
	for id := range a {
		if !b[id] {
			removed = append(removed, strings.TrimPrefix(strings.TrimPrefix(id, "screen:"), "flow:"))
		}
	}
	return added, removed
}

// noSnapshotsResult is the empty-local-store response shared by the missing-DB
// and zero-snapshot paths: a successful no-diff state that points the caller at
// `sync`.
func noSnapshotsResult(appSlug, since string) map[string]any {
	return map[string]any{
		"snapshots": 0,
		"diff":      nil,
		"app_slug":  appSlug,
		"since":     since,
		"note":      "no snapshots found; run `mobbin-pp-cli sync` first to populate the local store",
	}
}
