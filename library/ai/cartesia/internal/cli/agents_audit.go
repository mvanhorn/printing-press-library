// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

type metricSummary struct {
	MetricID  string  `json:"metric_id"`
	MeanScore float64 `json:"mean_score"`
	N         int     `json:"n"`
}

type deploymentSummary struct {
	ID        string          `json:"id"`
	Start     string          `json:"start,omitempty"`
	End       string          `json:"end,omitempty"`
	CallCount int             `json:"call_count"`
	Metrics   []metricSummary `json:"metrics"`
}

type regression struct {
	MetricID     string  `json:"metric_id"`
	PriorMean    float64 `json:"prior_mean"`
	CurrentMean  float64 `json:"current_mean"`
	Delta        float64 `json:"delta"`
}

// newAgentsAuditCmd registers `agents audit <agent-id>` — computes per-metric
// score deltas between the latest deployment and the prior deployment, flags
// regressions exceeding the configured threshold.
func newAgentsAuditCmd(flags *rootFlags) *cobra.Command {
	var since string
	var threshold float64
	var dbPath string
	cmd := &cobra.Command{
		Use:         "audit [agent-id]",
		Short:       "Compare metric scores across an agent's two latest deployments",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cartesia-pp-cli agents audit agent_abc --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			agentID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			var sinceISO string
			if since != "" {
				t, err := parseSince(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				sinceISO = t.UTC().Format("2006-01-02T15:04:05Z")
			}

			depIDs, err := loadAgentDeploymentIDs(cmd.Context(), db.DB(), agentID, sinceISO)
			if err != nil {
				return err
			}

			summaries := make([]deploymentSummary, 0, len(depIDs))
			for _, depID := range depIDs {
				s, err := buildDeploymentSummary(cmd.Context(), db.DB(), agentID, depID, sinceISO)
				if err != nil {
					return err
				}
				summaries = append(summaries, s)
			}

			regs := computeRegressions(summaries, threshold)
			out := map[string]any{
				"agent_id":    agentID,
				"deployments": summaries,
				"regressions": regs,
			}
			return flags.printJSON(cmd, out)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Restrict calls to this window (7d, 24h, or RFC3339)")
	cmd.Flags().Float64Var(&threshold, "regression-threshold", 0.1, "Flag metrics whose mean dropped by more than this delta")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// loadAgentDeploymentIDs returns the deployment IDs touched by this agent's
// calls (within the window when set), latest first. Returns at most two so
// the audit only ever compares "latest vs prior". When fewer than two
// deployments exist, we still return what we have — the caller produces an
// empty regressions list rather than failing.
func loadAgentDeploymentIDs(ctx context.Context, db *sql.DB, agentID, sinceISO string) ([]string, error) {
	q := `SELECT deployment_id, MAX(COALESCE(start_time, '')) AS latest
          FROM calls
          WHERE agent_id = ? AND deployment_id IS NOT NULL AND deployment_id != ''`
	argv := []any{agentID}
	if sinceISO != "" {
		q += " AND start_time >= ?"
		argv = append(argv, sinceISO)
	}
	q += " GROUP BY deployment_id ORDER BY latest DESC LIMIT 2"
	rows, err := db.QueryContext(ctx, q, argv...)
	if err != nil {
		return nil, fmt.Errorf("loading deployment ids: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var depID string
		var latest sql.NullString
		if err := rows.Scan(&depID, &latest); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		out = append(out, depID)
	}
	return out, rows.Err()
}

func buildDeploymentSummary(ctx context.Context, db *sql.DB, agentID, depID, sinceISO string) (deploymentSummary, error) {
	s := deploymentSummary{ID: depID}

	countQ := `SELECT COUNT(*), MIN(start_time), MAX(end_time) FROM calls WHERE agent_id = ? AND deployment_id = ?`
	argv := []any{agentID, depID}
	if sinceISO != "" {
		countQ += " AND start_time >= ?"
		argv = append(argv, sinceISO)
	}
	var start, end sql.NullString
	if err := db.QueryRowContext(ctx, countQ, argv...).Scan(&s.CallCount, &start, &end); err != nil {
		return s, fmt.Errorf("count calls: %w", err)
	}
	if start.Valid {
		s.Start = start.String
	}
	if end.Valid {
		s.End = end.String
	}

	metricsQ := `SELECT mr.metric_id, AVG(mr.score), COUNT(*)
                 FROM metric_results mr
                 JOIN calls c ON c.id = mr.call_id
                 WHERE c.agent_id = ? AND c.deployment_id = ? AND mr.score IS NOT NULL`
	argv2 := []any{agentID, depID}
	if sinceISO != "" {
		metricsQ += " AND c.start_time >= ?"
		argv2 = append(argv2, sinceISO)
	}
	metricsQ += " GROUP BY mr.metric_id"
	mrows, err := db.QueryContext(ctx, metricsQ, argv2...)
	if err != nil {
		return s, fmt.Errorf("agg metrics: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var ms metricSummary
		var mean sql.NullFloat64
		if err := mrows.Scan(&ms.MetricID, &mean, &ms.N); err != nil {
			return s, fmt.Errorf("scan metric: %w", err)
		}
		if mean.Valid {
			ms.MeanScore = mean.Float64
		}
		s.Metrics = append(s.Metrics, ms)
	}
	sort.Slice(s.Metrics, func(i, j int) bool { return s.Metrics[i].MetricID < s.Metrics[j].MetricID })
	return s, mrows.Err()
}

// computeRegressions diffs the latest deployment's metric means against
// the prior deployment's. Reports any metric whose mean dropped by more
// than threshold. summaries[0] is treated as the most recent.
func computeRegressions(summaries []deploymentSummary, threshold float64) []regression {
	out := []regression{}
	if len(summaries) < 2 {
		return out
	}
	current := indexMetrics(summaries[0])
	prior := indexMetrics(summaries[1])
	for id, cur := range current {
		p, ok := prior[id]
		if !ok {
			continue
		}
		delta := cur - p
		if -delta > threshold {
			out = append(out, regression{
				MetricID:    id,
				PriorMean:   p,
				CurrentMean: cur,
				Delta:       delta,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Delta < out[j].Delta })
	return out
}

func indexMetrics(s deploymentSummary) map[string]float64 {
	m := map[string]float64{}
	for _, ms := range s.Metrics {
		m[ms.MetricID] = ms.MeanScore
	}
	return m
}
