// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/internal/store"
	"github.com/spf13/cobra"
)

// slaRow is one group's aggregated SLA stats.
type slaRow struct {
	Group                   string  `json:"group"`
	Count                   int     `json:"count"`
	FirstResponseP50Minutes float64 `json:"first_response_p50_minutes,omitempty"`
	FirstResponseP90Minutes float64 `json:"first_response_p90_minutes,omitempty"`
	ResolutionP50Hours      float64 `json:"resolution_p50_hours,omitempty"`
	ResolutionP90Hours      float64 `json:"resolution_p90_hours,omitempty"`
	ClosedCount             int     `json:"closed_count,omitempty"`
}

func newConversationsSlaCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var metric string
	var since string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "sla",
		Short:       "Compute first-response and resolution-time SLAs from the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Group by team, last 7 days, both metrics
  intercom-pp-cli conversations sla

  # Group by admin, last 30 days, first-response only
  intercom-pp-cli conversations sla --group-by admin --since 30d --metric first-response

  # Top 5 teams by volume
  intercom-pp-cli conversations sla --limit 5 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"group_by": groupBy,
					"metric":   metric,
					"since":    since,
					"limit":    limit,
				}, flags)
				return nil
			}
			if groupBy != "team" && groupBy != "admin" {
				return usageErr(fmt.Errorf("--group-by must be 'team' or 'admin', got %q", groupBy))
			}
			wantFR, wantRes := false, false
			for _, m := range strings.Split(metric, ",") {
				switch strings.TrimSpace(m) {
				case "first-response":
					wantFR = true
				case "resolution":
					wantRes = true
				case "":
					// allow trailing comma
				default:
					return usageErr(fmt.Errorf("--metric must be 'first-response' or 'resolution', got %q", m))
				}
			}
			if !wantFR && !wantRes {
				wantFR, wantRes = true, true
			}

			sinceDur, err := parseSlaDuration(since)
			if err != nil {
				return usageErr(err)
			}
			sinceEpoch := time.Now().Add(-sinceDur).Unix()

			if dbPath == "" {
				dbPath = defaultDBPath("intercom-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				// No store at all → empty result + hint.
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: no local data. Run 'intercom-pp-cli sync --resources conversations,conversation_parts' first.\n")
				return printJSONFiltered(cmd.OutOrStdout(), make([]slaRow, 0), flags)
			}
			defer db.Close()

			if hintIfUnsynced(cmd, db, "conversations") {
				// Continue — hintIfUnsynced just wrote the hint.
			}

			rows, err := computeSLA(db, groupBy, sinceEpoch, wantFR, wantRes, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&groupBy, "group-by", "team", "Group rows by 'team' or 'admin'")
	cmd.Flags().StringVar(&metric, "metric", "first-response,resolution", "Metrics to compute: 'first-response', 'resolution' (CSV)")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window (e.g. 24h, 7d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max groups to return")

	return cmd
}

// parseSlaDuration accepts Go duration strings AND a "<N>d" day shorthand.
func parseSlaDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil || n < 0 {
			return 0, fmt.Errorf("invalid duration %q (expected e.g. 7d, 24h)", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// computeSLA pulls conversations + parts from the store and aggregates.
// The schema stores most fields inside the `data` JSON blob, so we use
// SQLite's json_extract.
func computeSLA(db *store.Store, groupBy string, sinceEpoch int64, wantFR, wantRes bool, limit int) ([]slaRow, error) {
	groupExpr := "json_extract(c.data, '$.team_assignee_id')"
	if groupBy == "admin" {
		groupExpr = "json_extract(c.data, '$.admin_assignee_id')"
	}

	// First-response per conversation:
	// min(parts.created_at where author.type='admin') - conversations.created_at
	// Resolution per conversation (only when state='closed'):
	// conversations.updated_at - conversations.created_at  (Intercom has no closed_at)
	//
	// PATCH(sla-cast-integer): CAST(json_extract(...) AS INTEGER) on every
	// timestamp pulled from a JSON blob. Intercom emits created_at and
	// updated_at as Unix integers on the wire, but JSON1's json_extract
	// returns whatever scalar type the stored JSON literal carries. If a
	// sync run stored "1779668479" as a JSON string instead of a number,
	// scanning into sql.NullInt64 silently yields NULL — which would zero
	// out every first_response sample. Forcing INTEGER coerces both
	// shapes consistently.
	q := fmt.Sprintf(`
		SELECT
			COALESCE(CAST(%s AS TEXT), '(unassigned)') AS grp,
			c.id,
			c.created_at,
			CAST(json_extract(c.data, '$.updated_at') AS INTEGER) AS updated_at,
			json_extract(c.data, '$.state') AS state,
			(
				SELECT MIN(CAST(json_extract(p.data, '$.created_at') AS INTEGER))
				FROM parts p
				WHERE p.conversations_id = c.id
				  AND json_extract(p.data, '$.author.type') = 'admin'
			) AS first_admin_part_at
		FROM conversations c
		WHERE c.created_at IS NOT NULL
		  AND c.created_at >= ?
	`, groupExpr)

	rows, err := db.DB().Query(q, sinceEpoch)
	if err != nil {
		// Table might not exist; treat as no data.
		if strings.Contains(err.Error(), "no such table") {
			return make([]slaRow, 0), nil
		}
		return nil, fmt.Errorf("sla query: %w", err)
	}
	defer rows.Close()

	type sample struct {
		frSecs, resSecs float64
		hasFR, isClosed bool
	}
	groups := map[string][]sample{}

	for rows.Next() {
		var (
			grp       string
			id        string
			created   sql.NullInt64
			updated   sql.NullInt64
			state     sql.NullString
			firstResp sql.NullInt64
		)
		if err := rows.Scan(&grp, &id, &created, &updated, &state, &firstResp); err != nil {
			return nil, err
		}
		if !created.Valid {
			continue
		}
		s := sample{}
		if firstResp.Valid && firstResp.Int64 >= created.Int64 {
			s.frSecs = float64(firstResp.Int64 - created.Int64)
			s.hasFR = true
		}
		if state.Valid && state.String == "closed" && updated.Valid && updated.Int64 >= created.Int64 {
			s.resSecs = float64(updated.Int64 - created.Int64)
			s.isClosed = true
		}
		groups[grp] = append(groups[grp], s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]slaRow, 0, len(groups))
	for grp, samples := range groups {
		row := slaRow{Group: grp, Count: len(samples)}
		if wantFR {
			var frs []float64
			for _, s := range samples {
				if s.hasFR {
					frs = append(frs, s.frSecs)
				}
			}
			if len(frs) > 0 {
				row.FirstResponseP50Minutes = roundTo(percentile(frs, 50)/60.0, 2)
				row.FirstResponseP90Minutes = roundTo(percentile(frs, 90)/60.0, 2)
			}
		}
		if wantRes {
			var rs []float64
			for _, s := range samples {
				if s.isClosed {
					rs = append(rs, s.resSecs)
				}
			}
			row.ClosedCount = len(rs)
			if len(rs) > 0 {
				row.ResolutionP50Hours = roundTo(percentile(rs, 50)/3600.0, 2)
				row.ResolutionP90Hours = roundTo(percentile(rs, 90)/3600.0, 2)
			}
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// percentile returns the linear-interpolated p-th percentile (0-100).
func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100.0) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

func roundTo(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
