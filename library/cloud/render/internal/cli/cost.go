// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// pp:novel-static-reference
//
// Render plan prices, USD/month, sourced from https://render.com/pricing
// (October 2025). Renders update plans periodically; verify via the dashboard
// before relying on absolute numbers. Plan keys match the API response's
// "plan" field on each resource type.
var renderPlanPrices = map[string]float64{
	// services (web/static/private/worker/cron)
	"free":      0.00,
	"starter":   7.00,
	"standard":  25.00,
	"pro":       85.00,
	"pro-plus":  175.00,
	"pro-max":   225.00,
	"pro-ultra": 450.00,
	// postgres
	"basic-256mb": 6.00,
	"basic-1gb":   19.00,
	"basic-4gb":   65.00,
	"pro-4gb":     95.00,
	"pro-8gb":     185.00,
	"pro-16gb":    365.00,
	"pro-32gb":    700.00,
	// key-value / redis
	// "free", "starter", "standard", "pro", "pro-plus" intentionally omitted
	// here — the service-tier values above are reused for kv/redis, since
	// the Go map dedupes keys. If kv/redis pricing diverges, split the table.
}

// costRow is one normalized resource line ahead of grouping.
type costRow struct {
	ID      string  `json:"id"`
	Kind    string  `json:"kind"` // service | postgres | key-value | redis | disk
	Name    string  `json:"name,omitempty"`
	Plan    string  `json:"plan,omitempty"`
	Project string  `json:"project,omitempty"`
	Env     string  `json:"environment,omitempty"`
	Owner   string  `json:"owner,omitempty"`
	Monthly float64 `json:"monthly_usd"`
}

// costGroup is one row in the rendered table.
type costGroup struct {
	Group      string  `json:"group"`
	Services   int     `json:"services"`
	Postgres   int     `json:"postgres"`
	KeyValue   int     `json:"key_value"`
	Redis      int     `json:"redis"`
	Disks      int     `json:"disks"`
	MonthlyUSD float64 `json:"monthly_usd"`
}

type costReport struct {
	GroupBy string      `json:"group_by"`
	Groups  []costGroup `json:"groups"`
	Total   costGroup   `json:"total"`
}

// planPrice returns the static USD/month for a plan key, or 0 when unknown.
// Unknown plans return 0 rather than an error so the report still totals
// correctly when Render adds a new plan tier.
func planPrice(plan string) float64 {
	plan = strings.ToLower(strings.TrimSpace(plan))
	if plan == "" {
		return 0
	}
	if v, ok := renderPlanPrices[plan]; ok {
		return v
	}
	return 0
}

// loadCostRows walks the local store for billable resources and produces a
// cost row per resource. Disks are priced at 0 in the static table (Render
// charges per GB, not per plan); they're still counted for the disk column.
func loadCostRows(db *store.Store) ([]costRow, error) {
	out := []costRow{}
	for _, kind := range []struct{ Kind, Resource string }{
		{"service", "services"},
		{"postgres", "postgres"},
		{"key-value", "key-value"},
		{"redis", "redis"},
		{"disk", "disks"},
	} {
		rows, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type = ?`, kind.Resource)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return nil, err
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			row := costRow{
				ID:      id,
				Kind:    kind.Kind,
				Name:    strFromAny(obj["name"]),
				Plan:    strFromAny(obj["plan"]),
				Project: strFromAny(obj["projectId"]),
				Env:     strFromAny(obj["environmentId"]),
				Owner:   strFromAny(obj["ownerId"]),
			}
			if row.Kind != "disk" {
				row.Monthly = planPrice(row.Plan)
			}
			out = append(out, row)
		}
		rows.Close()
	}
	return out, nil
}

// groupCostRows applies the --group-by selector and produces an ordered
// list of (group, counts, total). "none" returns one row with the whole
// universe.
func groupCostRows(rows []costRow, groupBy string) costReport {
	rep := costReport{GroupBy: groupBy}
	if groupBy == "" {
		groupBy = "none"
		rep.GroupBy = groupBy
	}
	keyFor := func(r costRow) string {
		switch groupBy {
		case "project":
			if r.Project == "" {
				return "(unassigned)"
			}
			return r.Project
		case "env", "environment":
			if r.Env == "" {
				return "(unassigned)"
			}
			return r.Env
		case "owner":
			if r.Owner == "" {
				return "(unassigned)"
			}
			return r.Owner
		default:
			return "all"
		}
	}
	groups := map[string]*costGroup{}
	for _, r := range rows {
		key := keyFor(r)
		g, ok := groups[key]
		if !ok {
			g = &costGroup{Group: key}
			groups[key] = g
		}
		switch r.Kind {
		case "service":
			g.Services++
		case "postgres":
			g.Postgres++
		case "key-value":
			g.KeyValue++
		case "redis":
			g.Redis++
		case "disk":
			g.Disks++
		}
		g.MonthlyUSD += r.Monthly
		rep.Total.Services += boolToInt(r.Kind == "service")
		rep.Total.Postgres += boolToInt(r.Kind == "postgres")
		rep.Total.KeyValue += boolToInt(r.Kind == "key-value")
		rep.Total.Redis += boolToInt(r.Kind == "redis")
		rep.Total.Disks += boolToInt(r.Kind == "disk")
		rep.Total.MonthlyUSD += r.Monthly
	}
	rep.Total.Group = "TOTAL"
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rep.Groups = append(rep.Groups, *groups[k])
	}
	return rep
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func newCostCmd(flags *rootFlags) *cobra.Command {
	var (
		groupBy string
		dbPath  string
	)
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Sum monthly cost across services, postgres, key-value, redis, and disks; group by project, env, or owner.",
		Example: strings.Trim(`
  render-pp-cli cost
  render-pp-cli cost --group-by project
  render-pp-cli cost --group-by env --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "cost"}`)
				return nil
			}
			switch groupBy {
			case "", "none", "project", "env", "environment", "owner":
			default:
				return fmt.Errorf("invalid --group-by %q: expected one of project, env, owner, none", groupBy)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()
			rows, err := loadCostRows(db)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("no billable resources found in cache — run 'render-pp-cli sync' first")
			}
			rep := groupCostRows(rows, groupBy)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %-10s %-6s %-7s %-7s %s\n", strings.ToUpper(rep.GroupBy), "SERVICES", "POSTGRES", "KV", "REDIS", "DISKS", "MONTHLY USD")
			for _, g := range rep.Groups {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10d %-10d %-6d %-7d %-7d $%.2f\n", g.Group, g.Services, g.Postgres, g.KeyValue, g.Redis, g.Disks, g.MonthlyUSD)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10d %-10d %-6d %-7d %-7d $%.2f\n", "TOTAL", rep.Total.Services, rep.Total.Postgres, rep.Total.KeyValue, rep.Total.Redis, rep.Total.Disks, rep.Total.MonthlyUSD)
			return nil
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "none", "Group by: project | env | owner | none")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}
