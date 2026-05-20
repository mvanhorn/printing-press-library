// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
	"github.com/spf13/cobra"
)

// parseAge interprets a human-friendly age string and returns the cutoff
// instant — i.e. comments older than the returned time are stale.
//
// Accepted forms:
//   - "Nd" / "Nw" / "Nh" / "Nm" relative durations (days/weeks/hours/minutes)
//   - any RFC3339 absolute timestamp ("2026-04-01T00:00:00Z")
func parseAge(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty duration")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	last := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("invalid duration %q (want Nd, Nw, Nh, Nm, or RFC3339)", s)
	}
	now := time.Now().UTC()
	switch last {
	case 'd', 'D':
		return now.AddDate(0, 0, -n), nil
	case 'w', 'W':
		return now.AddDate(0, 0, -7*n), nil
	case 'h', 'H':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'm', 'M':
		return now.Add(-time.Duration(n) * time.Minute), nil
	default:
		return time.Time{}, fmt.Errorf("invalid duration suffix %q in %q (want d, w, h, m)", string(last), s)
	}
}

func newCommentsAuditCmd(flags *rootFlags) *cobra.Command {
	var olderThan, groupBy, dbPath string

	cmd := &cobra.Command{
		Use:   "comments-audit",
		Short: "Aggregate unresolved comments across every synced file with age and group-by filters.",
		Long: `Read locally synced comments and surface unresolved ones older than the cutoff.
Group by file, author, or node. Output is a Markdown table by default; pass --json for
structured output.`,
		Example: strings.Trim(`
  # Unresolved comments older than 14 days, grouped by file+author
  figma-pp-cli comments-audit

  # 7-day cutoff, group by node
  figma-pp-cli comments-audit --older-than 7d --group-by node

  # Absolute cutoff (RFC3339)
  figma-pp-cli comments-audit --older-than 2026-04-01T00:00:00Z --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "comments-audit"}`)
				return nil
			}
			cutoff, err := parseAge(olderThan)
			if err != nil {
				return usageErr(err)
			}
			groupKeys := map[string]bool{}
			for _, p := range strings.Split(groupBy, ",") {
				p = strings.TrimSpace(strings.ToLower(p))
				if p != "" {
					groupKeys[p] = true
				}
			}
			if len(groupKeys) == 0 {
				groupKeys["file"] = true
				groupKeys["author"] = true
			}

			if dbPath == "" {
				dbPath = defaultDBPath("figma-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'figma-pp-cli sync' first", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, files_id, data FROM comments
				 WHERE (json_extract(data, '$.resolved_at') IS NULL OR json_extract(data, '$.resolved_at') = '')
				 ORDER BY files_id, json_extract(data, '$.created_at')`)
			if err != nil {
				return fmt.Errorf("querying comments: %w", err)
			}
			defer rows.Close()

			type comment struct {
				ID        string `json:"id"`
				FileID    string `json:"file_id"`
				Author    string `json:"author"`
				NodeID    string `json:"node_id,omitempty"`
				Message   string `json:"message,omitempty"`
				CreatedAt string `json:"created_at,omitempty"`
				AgeDays   int    `json:"age_days"`
			}

			var stale []comment
			now := time.Now().UTC()
			seen := false
			for rows.Next() {
				seen = true
				var id, filesID string
				var data []byte
				if err := rows.Scan(&id, &filesID, &data); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(data, &obj); err != nil {
					continue
				}
				createdAtRaw, _ := obj["created_at"].(string)
				if createdAtRaw == "" {
					continue
				}
				t, err := time.Parse(time.RFC3339, createdAtRaw)
				if err != nil {
					continue
				}
				if !t.Before(cutoff) {
					continue
				}
				author := ""
				if u, ok := obj["user"].(map[string]any); ok {
					author, _ = u["handle"].(string)
				}
				nodeID := ""
				if anchor, ok := obj["client_meta"].(map[string]any); ok {
					if nid, ok := anchor["node_id"].(string); ok {
						nodeID = nid
					}
				}
				message, _ := obj["message"].(string)
				stale = append(stale, comment{
					ID:        id,
					FileID:    filesID,
					Author:    author,
					NodeID:    nodeID,
					Message:   message,
					CreatedAt: createdAtRaw,
					AgeDays:   int(now.Sub(t).Hours() / 24),
				})
			}
			if !seen {
				return fmt.Errorf("local cache empty — run 'figma-pp-cli sync' first")
			}

			// Build group keys: ordered concatenation of selected fields.
			grouped := map[string][]comment{}
			for _, c := range stale {
				var parts []string
				if groupKeys["file"] {
					parts = append(parts, "file="+c.FileID)
				}
				if groupKeys["author"] {
					parts = append(parts, "author="+c.Author)
				}
				if groupKeys["node"] {
					parts = append(parts, "node="+c.NodeID)
				}
				k := strings.Join(parts, " ")
				grouped[k] = append(grouped[k], c)
			}

			groupNames := make([]string, 0, len(grouped))
			for k := range grouped {
				groupNames = append(groupNames, k)
			}
			sort.Strings(groupNames)

			if flags.asJSON {
				out := map[string]any{
					"cutoff":     cutoff.Format(time.RFC3339),
					"group_by":   sortedKeys(groupKeys),
					"groups":     map[string][]comment{},
					"total":      len(stale),
					"group_keys": groupNames,
				}
				gmap := map[string][]comment{}
				for _, k := range groupNames {
					gmap[k] = grouped[k]
				}
				out["groups"] = gmap
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "# Comments audit (older than %s)\n\n", cutoff.Format(time.RFC3339))
			if len(stale) == 0 {
				fmt.Fprintln(w, "_no unresolved comments older than cutoff_")
				return nil
			}
			fmt.Fprintf(w, "**Total stale:** %d · **Groups:** %d\n\n", len(stale), len(groupNames))
			for _, k := range groupNames {
				fmt.Fprintf(w, "## %s (%d)\n\n", k, len(grouped[k]))
				fmt.Fprintln(w, "| age | author | created_at | message |")
				fmt.Fprintln(w, "|---|---|---|---|")
				for _, c := range grouped[k] {
					msg := strings.ReplaceAll(c.Message, "\n", " ")
					if len(msg) > 80 {
						msg = msg[:77] + "..."
					}
					fmt.Fprintf(w, "| %dd | %s | %s | %s |\n", c.AgeDays, c.Author, c.CreatedAt, msg)
				}
				fmt.Fprintln(w)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&olderThan, "older-than", "14d", "Minimum age (Nd, Nw, Nh, Nm, or RFC3339)")
	cmd.Flags().StringVar(&groupBy, "group-by", "file,author", "Group by csv: file,author,node")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
