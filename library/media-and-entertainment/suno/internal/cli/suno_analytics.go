// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `analytics` — grouped roll-ups over local clips. Reads the local SQLite
// store only; no network and no auth. Read-only.

package cli

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

// analyticsColumns are the typed clips columns directly groupable by name.
var analyticsColumns = map[string]bool{
	"model_name":        true,
	"status":            true,
	"make_instrumental": true,
	"is_remix":          true,
	"has_stem":          true,
	"title":             true,
	"tags":              true,
}

// analyticsGroupExprRE validates a json_extract path fragment so an arbitrary
// --group-by value can't smuggle SQL into the GROUP BY clause. Allows dotted
// json paths like metadata.duration_type.
var analyticsGroupExprRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

func newSunoAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var (
		typ     string
		groupBy string
		limit   int
		dbPath  string
	)
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Grouped roll-ups over local clips (counts, averages, sums)",
		Long: "Group your synced clips by a field and report per-group count, average " +
			"duration, average BPM, total play count, and total upvotes.\n\n" +
			"--group-by accepts a stored clips column (model_name, status, make_instrumental, " +
			"is_remix, has_stem, title, tags), the synthetic key \"project\" (rolls up by " +
			"workspace via the membership index), or a json path into the stored clip " +
			"JSON (e.g. metadata.duration_type).",
		Example:     "  suno-pp-cli analytics --type clips --group-by model_name\n  suno-pp-cli analytics --type clips --group-by project --json",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if typ != "clips" {
				return usageErr(fmt.Errorf("invalid --type %q: only \"clips\" is supported", typ))
			}
			// "project" (alias "workspace") is a synthetic group key: it rolls
			// up via the clip↔workspace membership index rather than a clips
			// column, so a clip in N projects counts in each.
			byProject := groupBy == "project" || groupBy == "workspace"
			var groupExpr string
			if !byProject {
				var err error
				groupExpr, err = analyticsGroupExpr(groupBy)
				if err != nil {
					return usageErr(err)
				}
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'suno-pp-cli sync' first.", err)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "clips")
			hintIfStale(cmd, db, "clips", flags.maxAge)

			var results []analyticsGroup
			if byProject {
				results, err = analyticsByProject(db, limit)
			} else {
				results, err = analyticsGroups(db, groupExpr, limit)
			}
			if err != nil {
				return fmt.Errorf("aggregating local clips: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&typ, "type", "clips", "Resource type to analyze (only \"clips\" supported)")
	cmd.Flags().StringVar(&groupBy, "group-by", "model_name", "Field or json path to group by")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of groups to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store")
	return cmd
}

// analyticsGroupExpr resolves a --group-by value to a safe SQL expression.
// A known typed column is used as a bare column reference; anything else that
// matches the json-path shape becomes json_extract(data, '$.path'). Invalid
// shapes are rejected.
func analyticsGroupExpr(groupBy string) (string, error) {
	groupBy = strings.TrimSpace(groupBy)
	if groupBy == "" {
		return "", fmt.Errorf("--group-by is required")
	}
	if analyticsColumns[groupBy] {
		return `"` + groupBy + `"`, nil
	}
	if !analyticsGroupExprRE.MatchString(groupBy) {
		return "", fmt.Errorf("invalid --group-by %q: use a clips column or a json path like metadata.duration_type", groupBy)
	}
	return fmt.Sprintf(`json_extract(data, '$.%s')`, groupBy), nil
}

// analyticsByProject rolls clips up by the workspace (project) they belong to,
// via the clip↔workspace membership index. A clip in multiple projects is
// counted in each; clips with no project membership are excluded. Groups are
// labeled by workspace name (falling back to the workspace id).
func analyticsByProject(db *store.Store, limit int) ([]analyticsGroup, error) {
	out := make([]analyticsGroup, 0)
	query := `
		SELECT COALESCE(NULLIF(w."name", ''), cw."workspace_id") AS grp,
		       COUNT(*),
		       COALESCE(AVG(c.duration), 0),
		       COALESCE(AVG(c.avg_bpm), 0),
		       COALESCE(SUM(c.play_count), 0),
		       COALESCE(SUM(c.upvote_count), 0)
		  FROM "clip_workspaces" cw
		  JOIN "clips" c ON c."id" = cw."clip_id"
		  LEFT JOIN "workspace" w ON w."id" = cw."workspace_id"
		 GROUP BY grp
		 ORDER BY COUNT(*) DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.DB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g analyticsGroup
		var grp sql.NullString
		if err := rows.Scan(&grp, &g.Count, &g.AvgDuration, &g.AvgBPM, &g.SumPlayCount, &g.SumUpvoteCount); err != nil {
			return nil, err
		}
		if grp.Valid {
			g.Group = grp.String
		} else {
			g.Group = "(none)"
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// analyticsGroup is one aggregated group row.
type analyticsGroup struct {
	Group          string  `json:"group"`
	Count          int64   `json:"count"`
	AvgDuration    float64 `json:"avg_duration"`
	AvgBPM         float64 `json:"avg_bpm"`
	SumPlayCount   int64   `json:"sum_play_count"`
	SumUpvoteCount int64   `json:"sum_upvote_count"`
}

// analyticsGroups runs the grouped aggregate. groupExpr is a vetted SQL
// expression (validated column or json_extract path). NULL group keys render
// as "(none)". All aggregates are NULL-safe.
func analyticsGroups(db *store.Store, groupExpr string, limit int) ([]analyticsGroup, error) {
	out := make([]analyticsGroup, 0)
	query := fmt.Sprintf(
		`SELECT %s AS grp,
		        COUNT(*),
		        COALESCE(AVG(duration), 0),
		        COALESCE(AVG(avg_bpm), 0),
		        COALESCE(SUM(play_count), 0),
		        COALESCE(SUM(upvote_count), 0)
		   FROM clips
		  GROUP BY grp
		  ORDER BY COUNT(*) DESC`, groupExpr)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.DB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var g analyticsGroup
		var grp sql.NullString
		if err := rows.Scan(&grp, &g.Count, &g.AvgDuration, &g.AvgBPM, &g.SumPlayCount, &g.SumUpvoteCount); err != nil {
			return nil, err
		}
		if grp.Valid {
			g.Group = grp.String
		} else {
			g.Group = "(none)"
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
