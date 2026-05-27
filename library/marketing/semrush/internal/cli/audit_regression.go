// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #12 — audit regression watch.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newAuditRegressionCmd(flags *rootFlags) *cobra.Command {
	var snapshotsOverride string

	cmd := &cobra.Command{
		Use:         "regression [project-id]",
		Short:       "Diff the two most recent Site Audit snapshots: new issues, resolved issues, severity delta.",
		Long:        "regression takes the two most-recent audit_snapshot rows for a project and emits the set of new issue_ids, resolved issue_ids, and the count delta per severity bucket.",
		Example:     "  semrush-pp-cli audit regression 12345 --snapshots snap_a,snap_b",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			if !hintIfUnsynced(cmd, db, "audit") {
				hintIfStale(cmd, db, "audit", flags.maxAge)
			}

			projectID := args[0]
			var aID, bID string
			if strings.TrimSpace(snapshotsOverride) != "" {
				parts := strings.SplitN(snapshotsOverride, ",", 2)
				if len(parts) != 2 {
					return usageErr(fmt.Errorf("--snapshots requires exactly two comma-separated ids"))
				}
				aID = strings.TrimSpace(parts[0])
				bID = strings.TrimSpace(parts[1])
			} else {
				rows, err := db.DB().QueryContext(ctx,
					`SELECT COALESCE(json_extract(data, '$.snapshot_id'), json_extract(data, '$.id'), '') AS snap_id
					 FROM resources
					 WHERE resource_type IN ('audit_snapshot', 'audit_snapshots')
					   AND (json_extract(data, '$.project_id') = ? OR json_extract(data, '$.project_id') = CAST(? AS INTEGER))
					 ORDER BY synced_at DESC LIMIT 2`, projectID, projectID)
				if err != nil {
					return fmt.Errorf("query audit snapshots: %w", err)
				}
				defer rows.Close()
				var ids []string
				for rows.Next() {
					var s string
					if err := rows.Scan(&s); err != nil {
						return fmt.Errorf("scan audit snapshot: %w", err)
					}
					if strings.TrimSpace(s) == "" {
						continue
					}
					ids = append(ids, s)
				}
				if err := rows.Err(); err != nil {
					return fmt.Errorf("iterate audit snapshots: %w", err)
				}
				if len(ids) < 2 {
					return notFoundErr(fmt.Errorf("need at least two audit snapshots for project %s; found %d", projectID, len(ids)))
				}
				bID, aID = ids[0], ids[1] // newest = b, prior = a
			}

			loadIssues := func(snap string) (map[string]string, map[string]int, error) {
				if snap == "" {
					return map[string]string{}, map[string]int{}, nil
				}
				issues := map[string]string{}
				sev := map[string]int{}
				rows, err := db.DB().QueryContext(ctx,
					`SELECT COALESCE(json_extract(data, '$.issue_id'), json_extract(data, '$.id'), '') AS iid,
					        COALESCE(json_extract(data, '$.severity'), '') AS sev
					 FROM resources
					 WHERE resource_type IN ('audit_issue', 'audit_issues')
					   AND json_extract(data, '$.snapshot_id') = ?`, snap)
				if err != nil {
					return nil, nil, err
				}
				defer rows.Close()
				for rows.Next() {
					var iid, s string
					if err := rows.Scan(&iid, &s); err != nil {
						return nil, nil, err
					}
					if iid == "" {
						continue
					}
					issues[iid] = s
					sev[s]++
				}
				return issues, sev, rows.Err()
			}

			aIssues, aSev, err := loadIssues(aID)
			if err != nil {
				return fmt.Errorf("loading prior snapshot: %w", err)
			}
			bIssues, bSev, err := loadIssues(bID)
			if err != nil {
				return fmt.Errorf("loading current snapshot: %w", err)
			}

			var newIssues, resolved []string
			for id := range bIssues {
				if _, ok := aIssues[id]; !ok {
					newIssues = append(newIssues, id)
				}
			}
			for id := range aIssues {
				if _, ok := bIssues[id]; !ok {
					resolved = append(resolved, id)
				}
			}
			// Sort for deterministic JSON output. Both arrays are built by
			// ranging over Go maps (aIssues/bIssues), so without this sort
			// the order varies between runs and scripted diffing of two
			// 'audit regression' reports would be unreliable.
			sort.Strings(newIssues)
			sort.Strings(resolved)
			delta := map[string]int{}
			seen := map[string]bool{}
			for s := range aSev {
				seen[s] = true
			}
			for s := range bSev {
				seen[s] = true
			}
			for s := range seen {
				delta[s] = bSev[s] - aSev[s]
			}

			out := map[string]any{
				"project_id":     projectID,
				"snapshot_prior": aID,
				"snapshot_now":   bID,
				"new_issues":     newIssues,
				"resolved":       resolved,
				"severity_delta": delta,
				"new_count":      len(newIssues),
				"resolved_count": len(resolved),
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&snapshotsOverride, "snapshots", "", "Comma-separated snapshot ids 'prior,now' to override the default of latest-two")
	return cmd
}
