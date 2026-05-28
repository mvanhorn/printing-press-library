// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// auditHit is one row in the audit search result.
type auditHit struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Actor     string `json:"actor,omitempty"`
	Action    string `json:"action,omitempty"`
	Target    string `json:"target,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Search and inspect cached audit log entries.",
	}
	cmd.AddCommand(newAuditSearchCmd(flags))
	return cmd
}

func newAuditSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		actor  string
		target string
		action string
		since  string
		owner  string
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "FTS5 search across cached owners_audit_logs by actor, target, action, time, and owner.",
		Example: strings.Trim(`
  render-pp-cli audit search --actor user-d12abc --since 7d
  render-pp-cli audit search --action service.delete --target srv-d34xyz
  render-pp-cli audit search --owner own-d99zzz --since 30d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "audit search"}`)
				return nil
			}
			if actor == "" && target == "" && action == "" && since == "" && owner == "" {
				return cmd.Help()
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			cutoff := time.Time{}
			if since != "" {
				t, err := parseTimelineWindow(since, time.Now().UTC())
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				cutoff = t
			}
			hits, err := searchAuditLogs(db, actor, target, action, owner, cutoff, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No audit entries matched.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-20s %-25s %-25s %s\n", "TIMESTAMP", "ACTION", "ACTOR", "TARGET", "SUMMARY")
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-20s %-25s %-25s %s\n", h.Timestamp, h.Action, h.Actor, h.Target, h.Summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "Filter by actor / user id (substring match on actor field)")
	cmd.Flags().StringVar(&target, "target", "", "Filter by target / resource id (substring match on target/resourceId)")
	cmd.Flags().StringVar(&action, "action", "", "Filter by action name (e.g. service.delete)")
	cmd.Flags().StringVar(&since, "since", "", "Window start: duration (7d, 30d) or RFC3339 timestamp")
	cmd.Flags().StringVar(&owner, "owner", "", "Restrict to a specific owner id")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum hits to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// searchAuditLogs runs structured filters against owners_audit_logs and
// returns chronologically sorted hits. The filters are applied as SQL JSON
// extracts where possible; the FTS5 fallback is used for free-text actor
// matches when an exact value would be too brittle (Render audit shapes
// vary by event class).
func searchAuditLogs(db *store.Store, actor, target, action, owner string, cutoff time.Time, limit int) ([]auditHit, error) {
	out := []auditHit{}
	if limit <= 0 {
		limit = 200
	}

	conditions := []string{}
	args := []any{}
	if owner != "" {
		conditions = append(conditions, "owners_id = ?")
		args = append(args, owner)
	}
	if action != "" {
		conditions = append(conditions, "json_extract(data, '$.action') = ?")
		args = append(args, action)
	}
	if actor != "" {
		conditions = append(conditions, "(json_extract(data, '$.actor') LIKE ? OR json_extract(data, '$.userId') LIKE ?)")
		args = append(args, "%"+actor+"%", "%"+actor+"%")
	}
	if target != "" {
		conditions = append(conditions, "(json_extract(data, '$.target') LIKE ? OR json_extract(data, '$.resourceId') LIKE ?)")
		args = append(args, "%"+target+"%", "%"+target+"%")
	}
	q := `SELECT id, owners_id, data FROM owners_audit_logs`
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}
	q += " LIMIT ?"
	args = append(args, limit*4) // overfetch; we filter client-side by cutoff

	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ownerID string
		var raw []byte
		if err := rows.Scan(&id, &ownerID, &raw); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		ts := pickTimestamp(obj)
		if !cutoff.IsZero() && ts != "" {
			if t, err := time.Parse(time.RFC3339, ts); err == nil && t.Before(cutoff) {
				continue
			}
		}
		hit := auditHit{
			ID:        id,
			Timestamp: ts,
			Actor:     pickActor(obj),
			Action:    strFromAny(obj["action"]),
			Owner:     ownerID,
			Summary:   pickSummary(obj, "audit"),
		}
		hit.Target = strFromAny(obj["target"])
		if hit.Target == "" {
			hit.Target = strFromAny(obj["resourceId"])
		}
		out = append(out, hit)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
