// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #7 — audit triage (rank Site Audit pages by weighted severity).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newAuditTriageCmd(flags *rootFlags) *cobra.Command {
	var top int
	var snapshot string

	cmd := &cobra.Command{
		Use:         "triage [project-id]",
		Short:       "Rank Site Audit pages by weighted issue severity (errors*3 + warnings*1 + notices*0.1).",
		Long:        "triage joins audit_issue rows with audit_page rows and emits pages ordered by a weighted severity score so the highest-impact fixes float to the top.",
		Example:     "  semrush-pp-cli audit triage 12345 --top 20",
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

			// Resolve snapshot id (latest if unspecified).
			if snapshot == "" {
				row := db.DB().QueryRowContext(ctx,
					`SELECT COALESCE(json_extract(data, '$.snapshot_id'), json_extract(data, '$.id'), '') AS snap_id
					 FROM resources
					 WHERE resource_type IN ('audit_snapshot', 'audit_snapshots')
					   AND (json_extract(data, '$.project_id') = ? OR json_extract(data, '$.project_id') = CAST(? AS INTEGER))
					 ORDER BY synced_at DESC LIMIT 1`, projectID, projectID)
				_ = row.Scan(&snapshot)
			}

			// Aggregate page → counts via the audit_issue rows. The
			// generated audit table is keyed by id; both issue rows and
			// page rows live there. We filter by severity/page_id.
			pageRows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(json_extract(data, '$.page_id'), json_extract(data, '$.id'), '') AS pid,
				        COALESCE(json_extract(data, '$.url'), '') AS url,
				        COALESCE(json_extract(data, '$.title'), '') AS title,
				        COALESCE(json_extract(data, '$.errors_count'), json_extract(data, '$.errors'), 0) AS errs,
				        COALESCE(json_extract(data, '$.warnings_count'), json_extract(data, '$.warnings'), 0) AS warns,
				        COALESCE(json_extract(data, '$.notices_count'), json_extract(data, '$.notices'), 0) AS notes
				 FROM resources
				 WHERE resource_type IN ('audit_page', 'audit_pages')
				   AND (? = '' OR json_extract(data, '$.snapshot_id') = ?)
				   AND (json_extract(data, '$.project_id') = ? OR json_extract(data, '$.project_id') = CAST(? AS INTEGER) OR json_extract(data, '$.project_id') IS NULL)`,
				snapshot, snapshot, projectID, projectID)
			if err != nil {
				return fmt.Errorf("query audit pages: %w", err)
			}
			defer pageRows.Close()

			type pageScore struct {
				PageID  string  `json:"page_id"`
				URL     string  `json:"url"`
				Title   string  `json:"title"`
				Errors  float64 `json:"errors"`
				Warns   float64 `json:"warnings"`
				Notices float64 `json:"notices"`
				Score   float64 `json:"weighted_score"`
			}
			var pages []pageScore
			for pageRows.Next() {
				var p pageScore
				if err := pageRows.Scan(&p.PageID, &p.URL, &p.Title, &p.Errors, &p.Warns, &p.Notices); err != nil {
					return fmt.Errorf("scan audit page: %w", err)
				}
				p.Score = p.Errors*3 + p.Warns*1 + p.Notices*0.1
				pages = append(pages, p)
			}
			if err := pageRows.Err(); err != nil {
				return fmt.Errorf("iterate audit pages: %w", err)
			}

			// If audit_page rows don't carry per-severity counts, fall back
			// to aggregating from audit_issue rows — but only when we
			// actually resolved a snapshot for THIS project. If snapshot is
			// still empty (the snapshot lookup above didn't match the
			// requested project_id), running the fallback would dump every
			// audit_issue across every project in the local store and
			// aggregate them into a triage report unrelated to the
			// requested project. Bail with a clean empty result instead.
			if len(pages) == 0 {
				if snapshot == "" {
					out := map[string]any{
						"project_id": projectID,
						"snapshot":   "",
						"page_count": 0,
						"pages":      []any{},
						"hint":       fmt.Sprintf("no audit_snapshot rows for project %s in local store; run 'semrush-pp-cli sync --resources audit' first", projectID),
					}
					raw, err := json.Marshal(out)
					if err != nil {
						return fmt.Errorf("encoding empty triage response: %w", err)
					}
					return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
				}
				issueRows, err := db.DB().QueryContext(ctx,
					`SELECT COALESCE(json_extract(data, '$.page_id'), '') AS pid,
					        COALESCE(json_extract(data, '$.url'), '') AS url,
					        COALESCE(json_extract(data, '$.severity'), '') AS sev
					 FROM resources
					 WHERE resource_type IN ('audit_issue', 'audit_issues')
					   AND json_extract(data, '$.snapshot_id') = ?
					   AND (json_extract(data, '$.project_id') = ? OR json_extract(data, '$.project_id') = CAST(? AS INTEGER) OR json_extract(data, '$.project_id') IS NULL)`,
					snapshot, projectID, projectID)
				if err != nil {
					return fmt.Errorf("query audit issues: %w", err)
				}
				defer issueRows.Close()
				groups := map[string]*pageScore{}
				for issueRows.Next() {
					var pid, url, sev string
					if err := issueRows.Scan(&pid, &url, &sev); err != nil {
						return fmt.Errorf("scan audit issue: %w", err)
					}
					if pid == "" {
						pid = url
					}
					g, ok := groups[pid]
					if !ok {
						g = &pageScore{PageID: pid, URL: url}
						groups[pid] = g
					}
					switch sev {
					case "error", "errors", "critical":
						g.Errors++
					case "warning", "warnings", "warn":
						g.Warns++
					default:
						g.Notices++
					}
				}
				if err := issueRows.Err(); err != nil {
					return fmt.Errorf("iterate audit issues: %w", err)
				}
				for _, g := range groups {
					g.Score = g.Errors*3 + g.Warns*1 + g.Notices*0.1
					pages = append(pages, *g)
				}
			}

			// Sort by weighted score desc, with a PageID tiebreak so pages
			// with identical scores (rare but real on small audits) always
			// appear in the same order across runs. Without the tiebreak,
			// equal-score pages would inherit the upstream map's
			// non-deterministic iteration order.
			sort.SliceStable(pages, func(i, j int) bool {
				if pages[i].Score != pages[j].Score {
					return pages[i].Score > pages[j].Score
				}
				return pages[i].PageID < pages[j].PageID
			})
			totalPageCount := len(pages)
			truncated := false
			if top > 0 && len(pages) > top {
				pages = pages[:top]
				truncated = true
			}

			out := map[string]any{
				"project_id":       projectID,
				"snapshot":         snapshot,
				"top":              top,
				"page_count":       totalPageCount,
				"page_count_shown": len(pages),
				"truncated":        truncated,
				"pages":            pages,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&top, "top", 20, "Maximum pages to return (0 disables)")
	cmd.Flags().StringVar(&snapshot, "snapshot", "", "Snapshot id to scope to; default: latest synced snapshot for the project")
	return cmd
}
