// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/store"
	"github.com/spf13/cobra"
)

func newScheduleCmd(flags *rootFlags) *cobra.Command {
	var anyTenant bool
	var classType string
	var instructor string
	var location string
	var before string
	var after string
	var window time.Duration
	var earliest bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Query the merged class catalog with structured filters the iframe widget can't offer",
		Long: `schedule reads upcoming class sessions and applies structured filters.

Default: single-tenant + local SQLite cache (populated by 'sync').
--any-tenant: fan out to every tenant configured under
~/.config/marianatek-pp-cli/tenants/, hit each tenant's API live, merge the
results with a "tenant" annotation. Local cache is not consulted in this mode
(per-tenant cache is a planned framework increment).

Filters compose with AND semantics. --before and --after accept either RFC3339
timestamps or "HH:MM" time-of-day (applied to every day in --window).`,
		Example: `  # Vinyasa classes starting before 7am in the next week
  marianatek-pp-cli schedule --type vinyasa --before 07:00 --window 168h

  # Earliest open slot for a given instructor
  marianatek-pp-cli schedule --instructor "Lauren K" --earliest --json`,
		Annotations: map[string]string{
			"pp:novel":      "schedule",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			filters := scheduleFilters{
				ClassType:  strings.ToLower(classType),
				Instructor: strings.ToLower(instructor),
				Location:   strings.ToLower(location),
				Before:     before,
				After:      after,
				Window:     window,
			}
			// PATCH(retro #marianatek-multi-tenant): --any-tenant fans out
			// across every configured tenant via the live API.
			if anyTenant {
				rows, err := scheduleAcrossTenants(cmd, flags, filters)
				if err != nil {
					return err
				}
				if earliest && len(rows) > 1 {
					rows = rows[:1]
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("marianatek-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.List("classes", 5000)
			if err != nil {
				return fmt.Errorf("listing classes: %w", err)
			}

			matched := filterClasses(rows, filters)
			sortClassRowsByStart(matched)

			if earliest && len(matched) > 1 {
				matched = matched[:1]
			}
			return printJSONFiltered(cmd.OutOrStdout(), matched, flags)
		},
	}
	cmd.Flags().BoolVar(&anyTenant, "any-tenant", false, "aggregate across every logged-in tenant (v0.2: currently single-tenant)")
	cmd.Flags().StringVar(&classType, "type", "", "class type substring (case-insensitive)")
	cmd.Flags().StringVar(&instructor, "instructor", "", "instructor name substring (case-insensitive)")
	cmd.Flags().StringVar(&location, "location", "", "location name substring (case-insensitive)")
	cmd.Flags().StringVar(&before, "before", "", "only sessions starting before this time (RFC3339 or HH:MM)")
	cmd.Flags().StringVar(&after, "after", "", "only sessions starting after this time (RFC3339 or HH:MM)")
	cmd.Flags().DurationVar(&window, "window", 168*time.Hour, "look-ahead window from now (default 7d)")
	cmd.Flags().BoolVar(&earliest, "earliest", false, "return only the soonest matching session")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/marianatek-pp-cli/data.db)")
	return cmd
}

type scheduleFilters struct {
	ClassType  string
	Instructor string
	Location   string
	Before     string
	After      string
	Window     time.Duration
}

const scheduleTenantMaxPages = 100

// scheduleAcrossTenants pages through live `GET /classes` results for each
// configured tenant and merges rows with a "tenant" annotation added in-memory.
// PATCH(retro #marianatek-multi-tenant): live fan-out only; per-tenant local
// caching is deferred to a future framework increment.
func scheduleAcrossTenants(cmd *cobra.Command, flags *rootFlags, f scheduleFilters) ([]map[string]any, error) {
	tenants, err := config.ListTenants()
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	if len(tenants) == 0 {
		return nil, fmt.Errorf("no tenants configured; run `marianatek-pp-cli auth from-browser --tenant <slug> '<cookie>'` first")
	}
	now := time.Now().UTC()
	deadline := now.Add(f.Window)
	params := map[string]string{
		"min_start_date": now.Format("2006-01-02"),
		"max_start_date": deadline.Format("2006-01-02"),
		"page_size":      "100",
	}
	out := []map[string]any{}
	for _, tenant := range tenants {
		c, err := flags.newClientForTenant(tenant)
		if err != nil {
			out = append(out, map[string]any{"tenant": tenant, "error": err.Error()})
			continue
		}
		cursor := ""
		seenCursors := map[string]struct{}{}
		for page := 0; ; page++ {
			if page >= scheduleTenantMaxPages {
				out = append(out, map[string]any{"tenant": tenant, "warning": fmt.Sprintf("stopped after %d schedule pages", scheduleTenantMaxPages)})
				break
			}
			pageParams := copyScheduleParams(params)
			if cursor != "" {
				pageParams["after"] = cursor
			}
			data, err := c.Get("/classes", pageParams)
			if err != nil {
				out = append(out, map[string]any{"tenant": tenant, "error": err.Error()})
				break
			}
			results, nextCursor, hasMore, err := decodeSchedulePage(data)
			if err != nil {
				out = append(out, map[string]any{"tenant": tenant, "error": "parse: " + err.Error()})
				break
			}
			for _, r := range results {
				if !matchesLiveFilters(r, f) {
					continue
				}
				r["tenant"] = tenant
				out = append(out, r)
			}
			if !hasMore || nextCursor == "" {
				break
			}
			if _, ok := seenCursors[nextCursor]; ok {
				out = append(out, map[string]any{"tenant": tenant, "warning": "stopped on repeated schedule pagination cursor"})
				break
			}
			seenCursors[nextCursor] = struct{}{}
			cursor = nextCursor
		}
	}
	sortByStart(out)
	return out, nil
}

func decodeSchedulePage(data []byte) ([]map[string]any, string, bool, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, "", false, err
	}
	var results []map[string]any
	if raw, ok := envelope["results"]; ok {
		if err := json.Unmarshal(raw, &results); err != nil {
			return nil, "", false, err
		}
	}
	nextCursor, hasMore := extractPaginationFromEnvelope(envelope, "after")
	return results, nextCursor, hasMore, nil
}

func copyScheduleParams(params map[string]string) map[string]string {
	out := make(map[string]string, len(params)+1)
	for k, v := range params {
		out[k] = v
	}
	return out
}

func matchesLiveFilters(r map[string]any, f scheduleFilters) bool {
	if f.ClassType != "" {
		hit := attrContains(r, []string{"class_type_name", "name"}, f.ClassType)
		if !hit {
			if ct, ok := r["class_type"].(map[string]any); ok {
				hit = attrContains(ct, []string{"name", "description"}, f.ClassType)
			}
		}
		if !hit {
			return false
		}
	}
	if f.Instructor != "" && !attrContains(r, []string{"instructors"}, f.Instructor) {
		return false
	}
	if f.Location != "" {
		hit := false
		if loc, ok := r["location"].(map[string]any); ok {
			hit = attrContains(loc, []string{"name"}, f.Location)
		}
		if !hit {
			return false
		}
	}
	start := parseStart(r)
	if !timeFilter(start, f.Before, f.After) {
		return false
	}
	return true
}

// sortByStart performs a stable in-place sort over the live fan-out results.
func sortByStart(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		a := parseStart(rows[i])
		b := parseStart(rows[j])
		return startLess(a, b, stringAttr(rows[i], "start_datetime", "start_date"), stringAttr(rows[j], "start_datetime", "start_date"))
	})
}

func sortClassRowsByStart(rows []json.RawMessage) {
	type rowWithStart struct {
		raw   json.RawMessage
		start time.Time
		key   string
	}
	items := make([]rowWithStart, 0, len(rows))
	for _, row := range rows {
		item := rowWithStart{raw: row}
		var rec map[string]any
		if err := json.Unmarshal(row, &rec); err == nil {
			attrs := pickAttrs(rec)
			item.start = parseStart(attrs)
			item.key = stringAttr(attrs, "start_datetime", "start_date")
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return startLess(items[i].start, items[j].start, items[i].key, items[j].key)
	})
	for i, item := range items {
		rows[i] = item.raw
	}
}

func startLess(a, b time.Time, fallbackA, fallbackB string) bool {
	switch {
	case !a.IsZero() && !b.IsZero():
		return a.Before(b)
	case !a.IsZero():
		return true
	case !b.IsZero():
		return false
	default:
		return fallbackA < fallbackB
	}
}

func filterClasses(rows []json.RawMessage, f scheduleFilters) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(rows))
	now := time.Now().UTC()
	deadline := now.Add(f.Window)
	for _, row := range rows {
		var rec map[string]any
		if err := json.Unmarshal(row, &rec); err != nil {
			continue
		}
		attrs := pickAttrs(rec)
		if attrs == nil {
			continue
		}
		start := parseStart(attrs)
		if !start.IsZero() && (start.Before(now) || start.After(deadline)) {
			continue
		}
		if f.ClassType != "" && !attrContains(attrs, []string{"class_type_name", "class_type", "name"}, f.ClassType) {
			continue
		}
		if f.Instructor != "" && !attrContains(attrs, []string{"instructor_name", "instructor", "instructors"}, f.Instructor) {
			continue
		}
		if f.Location != "" && !attrContains(attrs, []string{"location_name", "location"}, f.Location) {
			continue
		}
		if !timeFilter(start, f.Before, f.After) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func pickAttrs(rec map[string]any) map[string]any {
	if data, ok := rec["data"].(map[string]any); ok {
		if attrs, ok := data["attributes"].(map[string]any); ok {
			return attrs
		}
	}
	if attrs, ok := rec["attributes"].(map[string]any); ok {
		return attrs
	}
	return rec
}

func parseStart(attrs map[string]any) time.Time {
	for _, key := range []string{"start_datetime", "start_date", "start_time", "datetime"} {
		if v, ok := attrs[key].(string); ok && v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func attrContains(attrs map[string]any, keys []string, needle string) bool {
	for _, key := range keys {
		v, ok := attrs[key]
		if !ok {
			continue
		}
		switch s := v.(type) {
		case string:
			if strings.Contains(strings.ToLower(s), needle) {
				return true
			}
		case []any:
			for _, item := range s {
				if str, ok := item.(string); ok && strings.Contains(strings.ToLower(str), needle) {
					return true
				}
				if m, ok := item.(map[string]any); ok {
					if name, ok := m["name"].(string); ok && strings.Contains(strings.ToLower(name), needle) {
						return true
					}
				}
			}
		}
	}
	return false
}

func timeFilter(start time.Time, before, after string) bool {
	if start.IsZero() {
		return true
	}
	if before != "" {
		if !beforeBoundary(start, before) {
			return false
		}
	}
	if after != "" {
		if !afterBoundary(start, after) {
			return false
		}
	}
	return true
}

func beforeBoundary(t time.Time, s string) bool {
	if at, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Before(at)
	}
	if hh, mm, ok := parseHHMM(s); ok {
		return t.Hour() < hh || (t.Hour() == hh && t.Minute() < mm)
	}
	return true
}

func afterBoundary(t time.Time, s string) bool {
	if at, err := time.Parse(time.RFC3339, s); err == nil {
		return t.After(at)
	}
	if hh, mm, ok := parseHHMM(s); ok {
		return t.Hour() > hh || (t.Hour() == hh && t.Minute() > mm)
	}
	return true
}

func parseHHMM(s string) (int, int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var hh, mm int
	if _, err := fmt.Sscanf(s, "%d:%d", &hh, &mm); err != nil {
		return 0, 0, false
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, false
	}
	return hh, mm, true
}
