// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
	"github.com/spf13/cobra"
)

// newOrphansCmd surfaces published library entities (components, styles,
// variables) with zero usage over a window by joining the team-library
// publish tables with the figma_analytics rows. Empty analytics is treated as
// a soft skip (Enterprise tier required).
func newOrphansCmd(flags *rootFlags) *cobra.Command {
	var team, kind, window, dbPath string

	cmd := &cobra.Command{
		Use:   "orphans [team_id]",
		Short: "Find published library entities with zero usage over a window.",
		Long: `Join the team-library publish tables (components, styles, variables) with the
figma_analytics usage rows, and surface entities whose summed usage over --window is zero.
Requires an Enterprise tier OAuth token; if analytics rows are absent, the command exits
gracefully with a guidance message.`,
		Example: strings.Trim(`
  # All kinds, last 30 days
  figma-pp-cli orphans 1234567890

  # Only components, last 7 days, JSON
  figma-pp-cli orphans --team 1234567890 --kind component --window 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "orphans"}`)
				return nil
			}
			if team == "" && len(args) > 0 {
				team = args[0]
			}
			kindSet := map[string]bool{}
			for _, p := range strings.Split(kind, ",") {
				p = strings.TrimSpace(strings.ToLower(p))
				if p != "" {
					kindSet[p] = true
				}
			}
			if len(kindSet) == 0 {
				kindSet["component"] = true
				kindSet["style"] = true
				kindSet["variable"] = true
			}
			cutoff, err := parseAge(window)
			if err != nil {
				return usageErr(err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("figma-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'figma-pp-cli sync' first", err)
			}
			defer db.Close()

			// Soft check: any analytics rows at all?
			var analyticsRows int
			_ = db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM figma_analytics`).Scan(&analyticsRows)
			if analyticsRows == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "library analytics data is empty — Enterprise tier required; run 'figma-pp-cli sync' with an OAuth token if you have access")
				return nil
			}

			// Build per-kind usage maps from figma_analytics. Each row has
			// `id` and `data` JSON; we look up the entity id and the running
			// "total" usage count, summed over rows newer than the cutoff.
			usage, err := buildAnalyticsUsage(cmd.Context(), db, cutoff)
			if err != nil {
				return err
			}

			result := map[string]any{
				"team":   team,
				"window": window,
				"cutoff": cutoff.Format(time.RFC3339),
			}

			if kindSet["component"] {
				items, err := orphanLibraryEntities(cmd.Context(), db, "teams_components", team, usage)
				if err != nil {
					return err
				}
				result["orphan_components"] = items
			}
			if kindSet["style"] {
				items, err := orphanLibraryEntities(cmd.Context(), db, "teams_styles", team, usage)
				if err != nil {
					return err
				}
				result["orphan_styles"] = items
			}
			if kindSet["variable"] {
				items, err := orphanLibraryEntities(cmd.Context(), db, "variables", "", usage)
				if err != nil {
					return err
				}
				result["orphan_variables"] = items
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "Team id (overrides positional)")
	cmd.Flags().StringVar(&kind, "kind", "component,style,variable", "csv of kinds to scan")
	cmd.Flags().StringVar(&window, "window", "30d", "Analytics window (Nd, Nw, Nh, Nm, or RFC3339)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

// buildAnalyticsUsage scans figma_analytics rows newer than cutoff and sums
// the per-entity total usages. It is intentionally permissive: if the row
// shape varies (component vs style vs variable analytics), we look up
// component_key, style_key, or variable_id and sum any of total, usages,
// usages_in_other_files, or count.
func buildAnalyticsUsage(ctx interface{ Done() <-chan struct{} }, db *store.Store, cutoff time.Time) (map[string]int, error) {
	rows, err := db.DB().Query(`SELECT data FROM figma_analytics WHERE synced_at >= ?`, cutoff.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}
		// Some payloads wrap rows under "rows".
		var nested []map[string]any
		if rs, ok := obj["rows"].([]any); ok {
			for _, r := range rs {
				if rm, ok := r.(map[string]any); ok {
					nested = append(nested, rm)
				}
			}
		} else {
			nested = []map[string]any{obj}
		}
		for _, r := range nested {
			id := firstString(r, "component_key", "style_key", "variable_id", "id")
			if id == "" {
				continue
			}
			n := firstInt(r, "total", "usages", "usages_in_other_files", "count")
			out[id] += n
		}
	}
	return out, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return int(v)
		}
	}
	return 0
}

// orphanLibraryEntities reads rows from a publish table and returns entries
// whose summed usage is zero.
func orphanLibraryEntities(ctx interface{ Done() <-chan struct{} }, db *store.Store, table, team string, usage map[string]int) ([]map[string]any, error) {
	q := "SELECT id, data FROM " + table
	var args []any
	if team != "" && (table == "teams_components" || table == "teams_styles") {
		q += " WHERE teams_id = ?"
		args = append(args, team)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		var obj map[string]any
		_ = json.Unmarshal(data, &obj)
		key := id
		if k, ok := obj["key"].(string); ok && k != "" {
			key = k
		}
		if usage[key] > 0 || usage[id] > 0 {
			continue
		}
		entry := map[string]any{
			"id":  id,
			"key": key,
		}
		if name, ok := obj["name"].(string); ok {
			entry["name"] = name
		}
		out = append(out, entry)
	}
	return out, nil
}
