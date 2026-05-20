// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type failureRow struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type failureMessageRow struct {
	ID        string `json:"id"`
	Reason    string `json:"reason,omitempty"`
	Status    string `json:"status,omitempty"`
	Direction string `json:"direction,omitempty"`
	Type      string `json:"type,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

func newMessagesFailuresCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		since   string
		groupBy string
	)
	cmd := &cobra.Command{
		Use:   "failures",
		Short: "Aggregate recent message-interaction failures grouped by reason code.",
		Long: `Reads the local messages table for messages whose status or interaction type is
"failed", "undelivered", "rejected", or "expired" within the --since window
(default 24h). With --group-by reason returns counts; otherwise returns the
flat list of failed messages.

Bird has no aggregation endpoint, so this answer only exists when sync has
populated the local store.`,
		Example:     "  bird-pp-cli messages failures --since 24h --group-by reason --json\n  bird-pp-cli messages failures --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseSince(since)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bird-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := queryFailures(db, cutoff)
			if err != nil {
				return err
			}
			if groupBy == "reason" {
				return printJSONFiltered(cmd.OutOrStdout(), aggregateByReason(rows), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "24h", "Only include messages newer than this (e.g. 1h, 24h, 7d)")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Group results by 'reason' (returns counts) or leave empty for the flat list")
	return cmd
}

func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Now().Add(-24 * time.Hour), nil
	}
	d, err := parseDayDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing --since %q: %w", s, err)
	}
	return time.Now().Add(-d), nil
}

func parseDayDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func queryFailures(db *store.Store, cutoff time.Time) ([]failureMessageRow, error) {
	rows, err := db.DB().Query(`SELECT id, data, status, direction, type, timestamp, reason, created_at FROM messages`)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()
	out := make([]failureMessageRow, 0, 32)
	for rows.Next() {
		var (
			id, status, direction, mtype, tsStr, reason, createdAtStr string
			raw                                                       []byte
		)
		if err := rows.Scan(&id, &raw, &status, &direction, &mtype, &tsStr, &reason, &createdAtStr); err != nil {
			return nil, err
		}
		// Parse the JSON envelope to recover any reason/timestamp not in columns.
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if reason == "" {
			if r, ok := m["reason"].(string); ok {
				reason = r
			}
		}
		if !isTerminalFailure(mtype) && !isTerminalFailure(status) {
			continue
		}
		// PATCH: --since must exclude messages whose timestamp can't be
		// resolved (NULL or non-RFC3339 in both `timestamp` and `created_at`)
		// instead of silently including them. The previous guard
		// `!t.IsZero() && t.Before(cutoff)` evaluated false for zero times,
		// so messages without a usable timestamp bypassed the window and
		// inflated `--group-by reason` counts with arbitrarily old failures.
		// Same shape as the compliance_auto_block.go fix; surfaced by
		// Greptile P1 in the PR #417 sixth review pass.
		t := pickTime(tsStr, createdAtStr)
		if t.IsZero() || t.Before(cutoff) {
			continue
		}
		out = append(out, failureMessageRow{
			ID:        id,
			Reason:    reason,
			Status:    status,
			Direction: direction,
			Type:      mtype,
			Timestamp: tsStr,
			CreatedAt: createdAtStr,
		})
	}
	// PATCH: surface mid-iteration scan errors (Greptile P1 in PR #417 ninth
	// review pass). Same shape as the fixes in compliance_auto_block.go,
	// sms_search.go, and messages_from.go: rows.Next() returns false on
	// both exhaustion and error, so without this check a truncated failure
	// list would feed --group-by reason and silently understate counts.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	return out, nil
}

func pickTime(a, b string) time.Time {
	for _, s := range []string{a, b} {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func aggregateByReason(rows []failureMessageRow) []failureRow {
	counts := make(map[string]int)
	for _, r := range rows {
		k := r.Reason
		if k == "" {
			k = "(no reason)"
		}
		counts[k]++
	}
	out := make([]failureRow, 0, len(counts))
	for k, v := range counts {
		out = append(out, failureRow{Reason: k, Count: v})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}
