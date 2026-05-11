package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newCoverageCmd computes a set-difference report between two object
// tables on a shared attribute key. Optionally buckets by --by and flags
// matched-but-stale rows by --stale.
func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath, left, right, key, by, stale string
	var limit int

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Set-difference report between two synced object tables on a shared key",
		Long: `Compares two synced object tables (e.g. salesforce_account vs company) on a
shared attribute (e.g. 'domain'). Reports:

  - missing_in_right: keys in --left that don't appear in --right
  - missing_in_left:  keys in --right that don't appear in --left
  - matched:          keys present in both (with stale flag when --stale is set)

Optional --by <attr> buckets the report by an attribute value (industry, owner, etc.).
Optional --stale 30d flags matched rows whose last_activity_at on the LEFT side is
older than the duration.

Both objects must be synced into the local store first (run 'sync').`,
		Example: strings.Trim(`
  unify-pp-cli coverage --left salesforce_account --right company --key domain --agent
  unify-pp-cli coverage --left salesforce_account --right company --key domain --by industry --stale 90d
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if left == "" || right == "" || key == "" {
				return usageErr(fmt.Errorf("--left, --right, and --key are required"))
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()

			leftRows, err := loadCoverageRows(ctx, s, left, key, by)
			if err != nil {
				return apiErr(err)
			}
			rightRows, err := loadCoverageRows(ctx, s, right, key, by)
			if err != nil {
				return apiErr(err)
			}

			// Index by key (lower-cased).
			leftIdx := map[string]coverageRow{}
			for _, r := range leftRows {
				if r.Key == "" {
					continue
				}
				leftIdx[strings.ToLower(r.Key)] = r
			}
			rightIdx := map[string]coverageRow{}
			for _, r := range rightRows {
				if r.Key == "" {
					continue
				}
				rightIdx[strings.ToLower(r.Key)] = r
			}

			// Resolve stale threshold.
			var staleCutoff time.Time
			if stale != "" {
				d, err := parseHumanDuration(stale)
				if err != nil {
					return usageErr(fmt.Errorf("--stale: %w", err))
				}
				staleCutoff = time.Now().Add(-d)
			}

			missingInRight := []map[string]any{}
			matched := []map[string]any{}
			for k, lrow := range leftIdx {
				if rrow, ok := rightIdx[k]; ok {
					m := map[string]any{
						"key":         lrow.Key,
						"left_id":     lrow.ID,
						"right_id":    rrow.ID,
						"left_bucket": lrow.Bucket,
					}
					if !staleCutoff.IsZero() {
						m["stale"] = isOlderThan(lrow.LastActivityAt, staleCutoff)
					}
					matched = append(matched, m)
					continue
				}
				missingInRight = append(missingInRight, map[string]any{
					"key":     lrow.Key,
					"left_id": lrow.ID,
					"bucket":  lrow.Bucket,
				})
			}
			missingInLeft := []map[string]any{}
			for k, rrow := range rightIdx {
				if _, ok := leftIdx[k]; !ok {
					missingInLeft = append(missingInLeft, map[string]any{
						"key":      rrow.Key,
						"right_id": rrow.ID,
						"bucket":   rrow.Bucket,
					})
				}
			}

			report := map[string]any{
				"left":  left,
				"right": right,
				"key":   key,
				"counts": map[string]int{
					"left_total":       len(leftIdx),
					"right_total":      len(rightIdx),
					"matched":          len(matched),
					"missing_in_right": len(missingInRight),
					"missing_in_left":  len(missingInLeft),
				},
			}
			// Cap detailed rows under limit.
			if limit > 0 {
				if len(matched) > limit {
					matched = matched[:limit]
				}
				if len(missingInRight) > limit {
					missingInRight = missingInRight[:limit]
				}
				if len(missingInLeft) > limit {
					missingInLeft = missingInLeft[:limit]
				}
			}
			report["matched"] = matched
			report["missing_in_right"] = missingInRight
			report["missing_in_left"] = missingInLeft

			if by != "" {
				report["by"] = bucketCounts(leftIdx, rightIdx)
				report["bucket_attribute"] = by
			}

			blob, _ := json.MarshalIndent(report, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().StringVar(&left, "left", "", "Left object name (e.g. salesforce_account)")
	cmd.Flags().StringVar(&right, "right", "", "Right object name (e.g. company)")
	cmd.Flags().StringVar(&key, "key", "", "Attribute name shared between the two objects (e.g. domain)")
	cmd.Flags().StringVar(&by, "by", "", "Bucket the report by this attribute (e.g. industry)")
	cmd.Flags().StringVar(&stale, "stale", "", "Flag matched rows whose left.last_activity_at is older than this duration (e.g. 30d, 90d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Cap per-bucket detail rows (0 = unlimited)")
	return cmd
}

type coverageRow struct {
	ID             string
	Key            string
	Bucket         string
	LastActivityAt string
}

func loadCoverageRows(ctx context.Context, s *store.Store, objectName, key, by string) ([]coverageRow, error) {
	table := store.RecordTable(objectName)
	// Verify table exists.
	var name string
	err := s.DB.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err != nil {
		return nil, fmt.Errorf("no synced records for %q (table %s missing). Run 'sync' first.", objectName, table)
	}
	q := fmt.Sprintf(`SELECT id, json_extract(attrs, '$."%s"') as k`, escapeJSONKey(key))
	q += `, json_extract(attrs, '$."last_activity_at"') as last_at`
	if by != "" {
		q += fmt.Sprintf(`, json_extract(attrs, '$."%s"') as b`, escapeJSONKey(by))
	} else {
		q += `, '' as b`
	}
	q += fmt.Sprintf(` FROM %q`, table)
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", table, err)
	}
	defer rows.Close()
	var out []coverageRow
	for rows.Next() {
		var id, k, la, b string
		var kv, lav, bv any
		if err := rows.Scan(&id, &kv, &lav, &bv); err != nil {
			return nil, err
		}
		k = anyToString(kv)
		la = anyToString(lav)
		b = anyToString(bv)
		out = append(out, coverageRow{ID: id, Key: k, Bucket: b, LastActivityAt: la})
	}
	return out, rows.Err()
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// escapeJSONKey escapes characters that would break a SQLite json path. We
// only handle the conservative case (double quotes); other chars are
// extremely rare in Unify api_names.
func escapeJSONKey(k string) string {
	return strings.ReplaceAll(k, "\"", "")
}

func parseHumanDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// Accept 30d, 24h, 1w, 90m, 60s.
	if strings.HasSuffix(s, "d") {
		var n int
		_, err := fmt.Sscanf(s, "%dd", &n)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "w") {
		var n int
		_, err := fmt.Sscanf(s, "%dw", &n)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func isOlderThan(timestamp string, cutoff time.Time) bool {
	if timestamp == "" {
		return true // treat unset as stale
	}
	// Try common formats.
	for _, fmtSpec := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(fmtSpec, timestamp); err == nil {
			return t.Before(cutoff)
		}
	}
	return false
}

func bucketCounts(left, right map[string]coverageRow) map[string]any {
	counts := map[string]struct {
		L, R, M, Lonly, Ronly int
	}{}
	keys := map[string]struct{}{}
	for k := range left {
		keys[k] = struct{}{}
	}
	for k := range right {
		keys[k] = struct{}{}
	}
	for k := range keys {
		b := ""
		_, inL := left[k]
		_, inR := right[k]
		if inL {
			b = left[k].Bucket
		} else if inR {
			b = right[k].Bucket
		}
		c := counts[b]
		if inL {
			c.L++
			if !inR {
				c.Lonly++
			}
		}
		if inR {
			c.R++
			if !inL {
				c.Ronly++
			}
		}
		if inL && inR {
			c.M++
		}
		counts[b] = c
	}
	buckets := []map[string]any{}
	for b, c := range counts {
		buckets = append(buckets, map[string]any{
			"bucket":      b,
			"left_total":  c.L,
			"right_total": c.R,
			"matched":     c.M,
			"left_only":   c.Lonly,
			"right_only":  c.Ronly,
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i]["bucket"].(string) < buckets[j]["bucket"].(string)
	})
	return map[string]any{"buckets": buckets}
}
