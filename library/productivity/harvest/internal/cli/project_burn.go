// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/harvest/internal/store"
)

func newProjectCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project-level local insights (budget, burn, velocity)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newProjectBurnCmd(flags))
	return cmd
}

type projectBurnRow struct {
	ProjectID            int64    `json:"project_id"`
	Name                 string   `json:"project"`
	Client               string   `json:"client,omitempty"`
	BudgetBy             string   `json:"budget_by"`
	Budget               float64  `json:"budget"`
	Used                 float64  `json:"used"`
	PctUsed              float64  `json:"pct_used"`
	OverThreshold        bool     `json:"over_threshold"`
	Velocity4w           float64  `json:"velocity_4w_per_week,omitempty"`
	WeeksToExhaust       *float64 `json:"weeks_to_exhaust,omitempty"`
	ProjectedExhaustDate string   `json:"projected_exhaust_date,omitempty"`
}

func newProjectBurnCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		threshold  float64
		projection bool
		onlyActive bool
		clientName string
	)

	cmd := &cobra.Command{
		Use:   "burn",
		Short: "List projects burning toward their budget with velocity-based projection",
		Long: `Joins projects (budget) with locally aggregated time_entries (used) and computes:
  - pct_used        = used / budget * 100
  - velocity_4w     = total hours in the last 4 ISO weeks / 4
  - weeks_to_exhaust = (budget - used) / velocity_4w
  - projected_exhaust_date = today + weeks_to_exhaust * 7

Exit code 2 if any active project crosses the threshold. Cron-friendly.

LOCAL command: requires 'sync' to populate projects + time_entries.`,
		Example: `  # Active projects over 80% with projected exhaust date
  harvest-pp-cli project burn --threshold 80 --projection --json

  # Cron: alert if any project hits 90%
  harvest-pp-cli project burn --threshold 90 --json > /tmp/burn.json || slack-notify`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []projectBurnRow{})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("harvest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'harvest-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := computeProjectBurn(cmd, db, threshold, projection, onlyActive, clientName)
			if err != nil {
				return err
			}
			if err := flags.printJSON(cmd, rows); err != nil {
				return err
			}
			for _, r := range rows {
				if r.OverThreshold {
					return &typedExit{code: 2, msg: fmt.Sprintf("%d project(s) over threshold", countOver(rows))}
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().Float64Var(&threshold, "threshold", 80.0, "Alert threshold (%)")
	cmd.Flags().BoolVar(&projection, "projection", true, "Include 4-week velocity and projected exhaust date")
	cmd.Flags().BoolVar(&onlyActive, "active-only", true, "Skip archived projects")
	cmd.Flags().StringVar(&clientName, "client", "", "Filter by client name (case-insensitive contains)")
	return cmd
}

func countOver(rows []projectBurnRow) int {
	n := 0
	for _, r := range rows {
		if r.OverThreshold {
			n++
		}
	}
	return n
}

func computeProjectBurn(cmd *cobra.Command, db *store.Store, threshold float64, projection, onlyActive bool, clientName string) ([]projectBurnRow, error) {
	projects, err := db.List("projects", 0)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	now := time.Now().UTC()
	fourWeeksAgo := now.AddDate(0, 0, -28)

	// Pre-aggregate hours per project: total and 4-week.
	type agg struct{ total, recent float64 }
	totals := map[int64]*agg{}

	sql := `SELECT
		IFNULL(CAST(json_extract(data, '$.project.id') AS INTEGER), 0) AS pid,
		IFNULL(json_extract(data, '$.spent_date'), '') AS sd,
		IFNULL(json_extract(data, '$.hours'), 0) AS h
	FROM time_entries`
	rows, err := db.DB().QueryContext(cmd.Context(), sql)
	if err != nil {
		return nil, fmt.Errorf("aggregate time_entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid int64
		var sd string
		var h float64
		if err := rows.Scan(&pid, &sd, &h); err != nil {
			return nil, err
		}
		if pid == 0 {
			continue
		}
		a := totals[pid]
		if a == nil {
			a = &agg{}
			totals[pid] = a
		}
		a.total += h
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			if !t.Before(fourWeeksAgo) {
				a.recent += h
			}
		}
	}

	out := make([]projectBurnRow, 0)
	for _, raw := range projects {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		active, _ := obj["is_active"].(bool)
		if onlyActive && !active {
			continue
		}
		var id int64
		switch v := obj["id"].(type) {
		case float64:
			id = int64(v)
		}
		name, _ := obj["name"].(string)
		cl := ""
		if client, ok := obj["client"].(map[string]any); ok {
			if cn, ok := client["name"].(string); ok {
				cl = cn
			}
		}
		if clientName != "" && !strFold(cl, clientName) {
			continue
		}

		budgetBy, _ := obj["budget_by"].(string)
		var budget float64
		if budgetBy == "hours" || budgetBy == "project_per_period" || budgetBy == "task" {
			if v, ok := obj["budget"].(float64); ok {
				budget = v
			}
		}
		if budget == 0 {
			// project has no hours budget (or it's monetary); skip
			continue
		}

		used := 0.0
		velocity := 0.0
		if a, ok := totals[id]; ok {
			used = a.total
			velocity = a.recent / 4.0
		}
		pct := 0.0
		if budget > 0 {
			pct = math.Round(used/budget*1000) / 10
		}
		row := projectBurnRow{
			ProjectID: id, Name: name, Client: cl, BudgetBy: budgetBy,
			Budget: budget, Used: used, PctUsed: pct,
			OverThreshold: pct >= threshold,
		}
		if projection && velocity > 0 && used < budget {
			weeks := (budget - used) / velocity
			row.Velocity4w = math.Round(velocity*10) / 10
			row.WeeksToExhaust = &weeks
			eta := now.AddDate(0, 0, int(weeks*7))
			row.ProjectedExhaustDate = eta.Format("2006-01-02")
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PctUsed > out[j].PctUsed })
	return out, nil
}

func strFold(haystack, needle string) bool {
	return stringContainsFold(haystack, needle)
}
