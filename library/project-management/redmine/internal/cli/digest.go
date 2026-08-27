// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/project-management/redmine/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/project-management/redmine/internal/store"
)

type digestRow struct {
	ID         int64  `json:"id"`
	Subject    string `json:"subject"`
	Project    string `json:"project"`
	Status     string `json:"status"`
	CreatedOn  string `json:"created_on"`
	UpdatedOn  string `json:"updated_on"`
	ClosedOn   string `json:"closed_on,omitempty"`
	ChangeType string `json:"change_type"`
	Watched    bool   `json:"watched,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagMine bool
	var flagWatched bool

	cmd := &cobra.Command{
		Use:     "digest",
		Short:   "A personal activity report of what was created, updated, or closed in a time window, optionally scoped to issues assigned to or watched by you.",
		Long:    "Use this for a personal activity report over a time window. Do NOT confuse with the framework's 'sync --since', which controls what data is pulled from the API, not what is reported from local data. Do NOT use for inactivity detection; use 'issues stale' instead.",
		Example: "  redmine-pp-cli digest --since 7d --mine --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "digest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			since := flagSince
			if since == "" {
				since = "7d"
			}
			dur, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			cutoffTime := time.Now().UTC().Add(-dur)
			cutoff := cutoffTime.Format(time.RFC3339)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var myID int64
			if flagMine {
				body, err := c.Get(ctx, "/users/current.json", nil)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var wrap struct {
					User struct {
						ID int64 `json:"id"`
					} `json:"user"`
				}
				if err := json.Unmarshal(body, &wrap); err != nil {
					return fmt.Errorf("parsing current user: %w", err)
				}
				myID = wrap.User.ID
			}

			results := make([]digestRow, 0)
			seen := map[int64]bool{}

			// Local scan runs whenever the digest is team-wide (no scope flags) or
			// --mine narrows it to the caller's own assignments. A bare --watched
			// (no --mine) skips the local scan entirely and relies solely on the
			// live watcher_id=me fetch below, so "watched" means watched, not
			// "everything plus watched".
			runLocalScan := flagMine || !flagWatched
			dbPath := defaultDBPath("redmine-pp-cli")
			if _, statErr := os.Stat(dbPath); statErr == nil && runLocalScan {
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("opening database: %w", err)
				}
				query := `SELECT
						json_extract(data, '$.id'),
						json_extract(data, '$.subject'),
						json_extract(data, '$.project.name'),
						json_extract(data, '$.status.name'),
						json_extract(data, '$.created_on'),
						json_extract(data, '$.updated_on'),
						json_extract(data, '$.closed_on')
					FROM resources
					WHERE resource_type = 'issues-json'
					  AND json_extract(data, '$.updated_on') >= ?`
				queryArgs := []any{cutoff}
				if flagMine {
					query += ` AND json_extract(data, '$.assigned_to.id') = ?`
					queryArgs = append(queryArgs, myID)
				}
				rows, err := db.DB().QueryContext(ctx, query, queryArgs...)
				if err != nil {
					_ = db.Close()
					return fmt.Errorf("query: %w", err)
				}
				for rows.Next() {
					var row digestRow
					var createdOn, updatedOn, closedOn *string
					if err := rows.Scan(&row.ID, &row.Subject, &row.Project, &row.Status, &createdOn, &updatedOn, &closedOn); err != nil {
						_ = rows.Close()
						_ = db.Close()
						return fmt.Errorf("scan row: %w", err)
					}
					if createdOn != nil {
						row.CreatedOn = *createdOn
					}
					if updatedOn != nil {
						row.UpdatedOn = *updatedOn
					}
					if closedOn != nil {
						row.ClosedOn = *closedOn
						row.ChangeType = "closed"
					} else if row.CreatedOn >= cutoff {
						row.ChangeType = "created"
					} else {
						row.ChangeType = "updated"
					}
					results = append(results, row)
					seen[row.ID] = true
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					_ = db.Close()
					return fmt.Errorf("iterate rows: %w", err)
				}
				if err := rows.Close(); err != nil {
					_ = db.Close()
					return fmt.Errorf("close rows: %w", err)
				}
				if err := db.Close(); err != nil {
					return fmt.Errorf("closing database: %w", err)
				}
			} else if runLocalScan {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: redmine-pp-cli sync --resources issues-json --db %s\n", dbPath, dbPath)
				}
			}

			if flagWatched {
				body, err := c.Get(ctx, "/issues.json", map[string]string{"watcher_id": "me", "status_id": "*", "limit": "100"})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var wrap struct {
					Issues []struct {
						ID      int64  `json:"id"`
						Subject string `json:"subject"`
						Project struct {
							Name string `json:"name"`
						} `json:"project"`
						Status struct {
							Name string `json:"name"`
						} `json:"status"`
						CreatedOn string `json:"created_on"`
						UpdatedOn string `json:"updated_on"`
						ClosedOn  string `json:"closed_on"`
					} `json:"issues"`
				}
				if err := json.Unmarshal(body, &wrap); err != nil {
					return fmt.Errorf("parsing watched issues: %w", err)
				}
				for _, issue := range wrap.Issues {
					if issue.UpdatedOn < cutoff || seen[issue.ID] {
						continue
					}
					changeType := "updated"
					if issue.ClosedOn != "" {
						changeType = "closed"
					} else if issue.CreatedOn >= cutoff {
						changeType = "created"
					}
					results = append(results, digestRow{
						ID: issue.ID, Subject: issue.Subject, Project: issue.Project.Name, Status: issue.Status.Name,
						CreatedOn: issue.CreatedOn, UpdatedOn: issue.UpdatedOn, ClosedOn: issue.ClosedOn,
						ChangeType: changeType, Watched: true,
					})
					seen[issue.ID] = true
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No activity in the last %s.\n", since)
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] #%d %s\t%s\t%s\n", r.ChangeType, r.ID, r.Subject, r.Project, r.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Time window to report on (e.g. 24h, 7d, 1w)")
	cmd.Flags().BoolVar(&flagMine, "mine", false, "Limit to issues assigned to the authenticated user")
	cmd.Flags().BoolVar(&flagWatched, "watched", false, "Include issues watched by the authenticated user (live lookup)")
	return cmd
}
