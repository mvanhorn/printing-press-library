// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var sinceFlag string
	var failed bool
	var endpoint string
	var typeID int64
	var byDay bool
	var byEndpoint bool
	var limit int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "View the lookup_log: every Numista API call, HTTP status, duration, and cache-hit status.",
		Long:  "Local SQL view over the lookup_log table populated by every API call. No API call needed.",
		Example: `  numista-pp-cli audit --since 7d --json
  numista-pp-cli audit --failed --by-endpoint --json
  numista-pp-cli audit --type-id 11013`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseAuditSince(sinceFlag)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", sinceFlag, err))
			}

			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return fmt.Errorf("audit: %w", err)
			}
			defer s.Close()

			var out any
			if byDay || byEndpoint {
				if byDay && byEndpoint {
					return usageErr(fmt.Errorf("--by-day and --by-endpoint are mutually exclusive"))
				}
				rows, err := runAuditAggregate(cmd.Context(), s.DB(), since, failed, endpoint, typeID, byDay)
				if err != nil {
					return fmt.Errorf("audit: %w", err)
				}
				out = rows
			} else {
				rows, err := s.RecentLookups(cmd.Context(), since, failed, endpoint, typeID, limit)
				if err != nil {
					return fmt.Errorf("audit: %w", err)
				}
				out = rows
			}

			data, err := json.Marshal(out)
			if err != nil {
				return fmt.Errorf("audit: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&sinceFlag, "since", "", "filter to rows newer than this duration (e.g. \"7d\", \"24h\", \"1h\"); empty = no filter")
	cmd.Flags().BoolVar(&failed, "failed", false, "only rows where IsValidRequest=false")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "exact-match on endpoint column")
	cmd.Flags().Int64Var(&typeID, "type-id", 0, "exact-match on type_id column")
	cmd.Flags().BoolVar(&byDay, "by-day", false, "aggregate: emit one row per UTC day with count")
	cmd.Flags().BoolVar(&byEndpoint, "by-endpoint", false, "aggregate: emit one row per endpoint with count")
	cmd.Flags().IntVar(&limit, "limit", 200, "max rows when not aggregating")

	return cmd
}

func parseAuditSince(v string) (time.Time, error) {
	if strings.TrimSpace(v) == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(v, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(v, "d"))
		if err != nil || n < 0 {
			return time.Time{}, fmt.Errorf("expected positive day count like 7d")
		}
		return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return time.Time{}, err
	}
	if d < 0 {
		return time.Time{}, fmt.Errorf("duration must be positive")
	}
	return time.Now().Add(-d), nil
}

// PATCH: local audit aggregation over Numista lookup_log rows.
func runAuditAggregate(ctx context.Context, db *sql.DB, since time.Time, failedOnly bool, endpoint string, typeID int64, byDay bool) ([]map[string]any, error) {
	var base string
	if byDay {
		base = "SELECT date(called_at), COUNT(*) FROM lookup_log WHERE 1=1"
	} else {
		base = "SELECT endpoint, COUNT(*) FROM lookup_log WHERE 1=1"
	}

	args := make([]any, 0, 4)
	if !since.IsZero() {
		base += " AND called_at >= ?"
		args = append(args, since.UTC().Format("2006-01-02 15:04:05"))
	}
	if failedOnly {
		base += " AND is_valid_request = 0"
	}
	if endpoint != "" {
		base += " AND endpoint = ?"
		args = append(args, endpoint)
	}
	if typeID != 0 {
		base += " AND type_id = ?"
		args = append(args, typeID)
	}

	if byDay {
		base += " GROUP BY date(called_at) ORDER BY date(called_at) DESC"
	} else {
		base += " GROUP BY endpoint ORDER BY COUNT(*) DESC"
	}

	rows, err := db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var key sql.NullString
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		if byDay {
			out = append(out, map[string]any{
				"day":   key.String,
				"count": count,
			})
		} else {
			out = append(out, map[string]any{
				"endpoint": key.String,
				"count":    count,
			})
		}
	}
	return out, rows.Err()
}
