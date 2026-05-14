// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/store"
)

func newRegularsCmd(flags *rootFlags) *cobra.Command {
	var by string
	var top int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "regulars",
		Short: "Rank your local reservation history by instructor, class type, time-of-day, day-of-week, or location",
		Long: `regulars groups your synced reservation history by any single dimension and
ranks by count. /me/metrics/top_instructors returns one dimension; regulars
joins reservations + classes + instructors + locations and aggregates by any
dimension you ask for.

Run 'marianatek-pp-cli sync --resources me_reservations,classes' first to
populate the local store.`,
		Example: `  # Top 5 instructors you've booked
  marianatek-pp-cli regulars --by instructor --top 5

  # Most-booked time-of-day, JSON for an agent
  marianatek-pp-cli regulars --by time --top 5 --json`,
		Annotations: map[string]string{
			"pp:novel":      "regulars",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dim, err := normalizeRegularsDim(by)
			if err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("marianatek-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			reservations, err := db.List("me_reservations", 10000)
			if err != nil {
				return fmt.Errorf("listing reservations: %w", err)
			}
			classes, err := db.List("classes", 10000)
			if err != nil {
				return fmt.Errorf("listing classes: %w", err)
			}
			classByID := indexClassesByID(classes)

			ranking := rankRegulars(reservations, classByID, dim)
			if top > 0 && len(ranking) > top {
				ranking = ranking[:top]
			}
			return printJSONFiltered(cmd.OutOrStdout(), ranking, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "instructor", "dimension: instructor, type, time, day, location")
	cmd.Flags().IntVar(&top, "top", 5, "max rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/marianatek-pp-cli/data.db)")
	return cmd
}

func normalizeRegularsDim(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "instructor", "instructors":
		return "instructor", nil
	case "type", "class-type", "class_type":
		return "type", nil
	case "time", "time-of-day":
		return "time", nil
	case "day", "day-of-week":
		return "day", nil
	case "location", "studio":
		return "location", nil
	default:
		return "", fmt.Errorf("--by must be one of: instructor, type, time, day, location (got %q)", s)
	}
}

type regularsRow struct {
	Dimension  string `json:"dimension"`
	Value      string `json:"value"`
	Count      int    `json:"count"`
	LastBooked string `json:"last_booked,omitempty"`
}

func rankRegulars(reservations []json.RawMessage, classByID map[string]map[string]any, dim string) []regularsRow {
	counts := map[string]int{}
	last := map[string]string{}
	for _, rsv := range reservations {
		var rec map[string]any
		if err := json.Unmarshal(rsv, &rec); err != nil {
			continue
		}
		attrs := pickAttrs(rec)
		if attrs == nil {
			continue
		}
		classID := ""
		if id, ok := attrs["class_session_id"].(string); ok {
			classID = id
		} else if id, ok := attrs["class_id"].(string); ok {
			classID = id
		}
		var cattrs map[string]any
		if classID != "" {
			cattrs = classByID[classID]
		}
		if cattrs == nil {
			cattrs = attrs // fall back to reservation attributes
		}
		val := extractDimensionValue(cattrs, dim)
		if val == "" {
			continue
		}
		counts[val]++
		bookedAt := stringAttr(attrs, "created_at", "booked_at", "start_datetime")
		if bookedAt != "" {
			if existing, ok := last[val]; !ok || bookedAt > existing {
				last[val] = bookedAt
			}
		}
	}
	out := make([]regularsRow, 0, len(counts))
	for v, c := range counts {
		out = append(out, regularsRow{Dimension: dim, Value: v, Count: c, LastBooked: last[v]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].LastBooked > out[j].LastBooked
	})
	return out
}

func indexClassesByID(classes []json.RawMessage) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, raw := range classes {
		var rec map[string]any
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		if data, ok := rec["data"].(map[string]any); ok {
			id, _ := data["id"].(string)
			attrs, _ := data["attributes"].(map[string]any)
			if id != "" && attrs != nil {
				out[id] = attrs
			}
			continue
		}
		id, _ := rec["id"].(string)
		attrs, _ := rec["attributes"].(map[string]any)
		if id != "" && attrs != nil {
			out[id] = attrs
		}
	}
	return out
}

func extractDimensionValue(attrs map[string]any, dim string) string {
	switch dim {
	case "instructor":
		return stringAttr(attrs, "instructor_name", "instructor")
	case "type":
		return stringAttr(attrs, "class_type_name", "class_type", "name")
	case "location":
		return stringAttr(attrs, "location_name", "location")
	case "time":
		start := parseStart(attrs)
		if start.IsZero() {
			return ""
		}
		return fmt.Sprintf("%02d:00", start.Hour())
	case "day":
		start := parseStart(attrs)
		if start.IsZero() {
			return ""
		}
		return start.Weekday().String()
	}
	return ""
}

func stringAttr(attrs map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := attrs[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if arr, ok := v.([]any); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok && s != "" {
					return s
				}
				if m, ok := arr[0].(map[string]any); ok {
					if s, ok := m["name"].(string); ok && s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}
