package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "One-shot stats blob for your local OpenArt media library",
		Long: `Aggregates over the local media + credits store: counts per resource
type, per model, per period, plus rolling spend.

Run 'openart-pp-cli sync' to refresh the local mirror first.`,
		Example:     `  openart-pp-cli stats --since 30d`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("open local store: %w (run 'openart-pp-cli sync' first)", err)
			}
			defer db.Close()

			cutoff := parseSince(since)
			cutoffStr := ""
			cutoffMs := int64(0)
			if !cutoff.IsZero() {
				cutoffStr = cutoff.Format("2006-01-02 15:04:05")
				cutoffMs = cutoff.UnixMilli()
			}

			// PATCH: media windows filter on created_at (generation time,
			// epoch ms from the resource payload) instead of synced_at,
			// which is rewritten on every resync — after a first-time full
			// sync every historical row would land inside `--since 30d`.
			// Rows without a payload createdAt fall back to synced_at.
			// Greptile P1 on PR #554.

			// Total counts.
			totalQ := `SELECT COUNT(*) FROM media`
			if cutoffStr != "" {
				totalQ += mediaSinceClause(cutoffMs, cutoffStr)
			}
			var totalCount int
			_ = db.QueryRowContext(cmd.Context(), totalQ).Scan(&totalCount)

			// Counts by resource_type.
			byTypeQ := `SELECT resource_type, COUNT(*) FROM media`
			if cutoffStr != "" {
				byTypeQ += mediaSinceClause(cutoffMs, cutoffStr)
			}
			byTypeQ += " GROUP BY resource_type"
			byType := map[string]int{}
			rows, err := db.QueryContext(cmd.Context(), byTypeQ)
			if err == nil {
				for rows.Next() {
					var t sql.NullString
					var n int
					if err := rows.Scan(&t, &n); err == nil {
						byType[stringOrEmpty(t)] += n
					}
				}
				rows.Close()
			}

			// Counts by model (parsed from data.input.model JSON).
			byModel := map[string]int{}
			rows2, err := db.QueryContext(cmd.Context(), `SELECT data, created_at, synced_at FROM media`)
			if err == nil {
				for rows2.Next() {
					var data []byte
					var createdAt sql.NullInt64
					var syncedAt string
					if err := rows2.Scan(&data, &createdAt, &syncedAt); err != nil {
						continue
					}
					if cutoffStr != "" {
						if createdAt.Valid {
							if createdAt.Int64 < cutoffMs {
								continue
							}
						} else if syncedAt < cutoffStr {
							continue
						}
					}
					var blob map[string]any
					if json.Unmarshal(data, &blob) != nil {
						continue
					}
					input, _ := blob["input"].(map[string]any)
					gen, _ := blob["generation"].(map[string]any)
					model := ""
					if input != nil {
						if s, ok := input["model"].(string); ok {
							model = s
						}
					}
					if model == "" && gen != nil {
						if cap, ok := gen["capabilityId"].(string); ok {
							if i := strings.IndexByte(cap, ':'); i > 0 {
								model = cap[:i]
							}
						}
					}
					if model == "" {
						model = "unknown"
					}
					byModel[model]++
				}
				rows2.Close()
			}

			// Spend totals from credits.
			// PATCH: filter on first_seen_at (consume-event observation
			// time) instead of synced_at, which is rewritten on every
			// resync. Without this, `stats --since 30d` would include
			// every consume event ever synced after a fresh full sync
			// (greptile P1 on PR #554).
			creditQ := `SELECT COALESCE(SUM(-amount), 0), COUNT(*) FROM credits WHERE type = 'CONSUME'`
			if cutoffStr != "" {
				creditQ += " AND first_seen_at >= '" + cutoffStr + "'"
			}
			var totalSpend, spendEvents int
			_ = db.QueryRowContext(cmd.Context(), creditQ).Scan(&totalSpend, &spendEvents)

			result := map[string]any{
				"since":          since,
				"total_media":    totalCount,
				"by_resource_type": byType,
				"by_model":       sortMapByValue(byModel),
				"credits_spent":  totalSpend,
				"spend_events":   spendEvents,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Time window: 24h, 7d, 30d, 90d, '' for all-time")
	return cmd
}

func stringOrEmpty(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// sortMapByValue returns the (key, count) pairs ordered by count desc.
func sortMapByValue(m map[string]int) []map[string]any {
	type kv struct {
		Key   string
		Count int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].Count > pairs[j].Count })
	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{"name": p.Key, "count": p.Count})
	}
	return out
}

// mediaSinceClause windows media rows on created_at (the generation time
// carried in the resource payload, epoch milliseconds) rather than
// synced_at, which is rewritten on every resync. Rows whose payload had no
// createdAt (created_at IS NULL) fall back to the synced_at proxy so they
// are not silently dropped.
func mediaSinceClause(cutoffMs int64, cutoffStr string) string {
	return fmt.Sprintf(" WHERE (created_at >= %d OR (created_at IS NULL AND synced_at >= '%s'))", cutoffMs, cutoffStr)
}
