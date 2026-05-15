// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// callGrepHit is the JSON shape returned per matching call.
type callGrepHit struct {
	ID         string  `json:"id"`
	AgentID    string  `json:"agent_id"`
	Summary    string  `json:"summary,omitempty"`
	StartTime  string  `json:"start_time,omitempty"`
	Status     string  `json:"status,omitempty"`
	Snippet    string  `json:"snippet,omitempty"`
	TurnWindow []any   `json:"turns,omitempty"`
}

// newCallsGrepCmd registers `calls grep "<pattern>"`. Uses calls_fts MATCH
// for fast prefix queries and falls back to LIKE for patterns that include
// regex metacharacters FTS5 cannot parse cleanly.
func newCallsGrepCmd(flags *rootFlags) *cobra.Command {
	var agentID string
	var since string
	var turns int
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "grep [pattern]",
		Short:       "Full-text search across synced call transcripts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  cartesia-pp-cli calls grep "cancel" --json
  cartesia-pp-cli calls grep "refund" --agent agent_abc --since 7d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			pattern := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			useFTS := !looksLikeRegex(pattern)
			var where []string
			var argv []any
			var q string
			if useFTS {
				q = `SELECT c.id, c.agent_id, c.summary, c.start_time, c.status, c.transcript_json
                     FROM calls_fts f JOIN calls c ON c.id = f.id
                     WHERE calls_fts MATCH ?`
				// FTS5 MATCH interprets bare apostrophes and multi-word
				// patterns as syntax tokens; wrap in double quotes (and
				// escape embedded ones) so the pattern is treated as a
				// literal phrase.
				ftsPattern := `"` + strings.ReplaceAll(pattern, `"`, `""`) + `"`
				argv = append(argv, ftsPattern)
			} else {
				// PATCH(greptile #560): LIKE treats bare % and _ as wildcards.
				// Escape both (and the escape character itself) so patterns
				// like "100%" or "agent_id" match literally rather than
				// silently broadening the result set.
				q = `SELECT id, agent_id, summary, start_time, status, transcript_json
                     FROM calls WHERE transcript_text LIKE ? ESCAPE '\'`
				escaped := strings.NewReplacer(
					`\`, `\\`,
					`%`, `\%`,
					`_`, `\_`,
				).Replace(pattern)
				argv = append(argv, "%"+escaped+"%")
			}
			if agentID != "" {
				where = append(where, "c.agent_id = ?")
				if !useFTS {
					where[len(where)-1] = "agent_id = ?"
				}
				argv = append(argv, agentID)
			}
			if since != "" {
				t, err := parseSince(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				if useFTS {
					where = append(where, "c.start_time >= ?")
				} else {
					where = append(where, "start_time >= ?")
				}
				argv = append(argv, t.UTC().Format("2006-01-02T15:04:05Z"))
			}
			for _, w := range where {
				q += " AND " + w
			}
			q += " LIMIT ?"
			if limit <= 0 {
				limit = 50
			}
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query calls: %w", err)
			}
			defer rows.Close()

			var results []callGrepHit
			for rows.Next() {
				var hit callGrepHit
				var transcript string
				if err := rows.Scan(&hit.ID, &hit.AgentID, &hit.Summary, &hit.StartTime, &hit.Status, &transcript); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				hit.Snippet = makeSnippet(transcript, pattern)
				if turns > 0 && transcript != "" {
					hit.TurnWindow = extractTurnWindow(transcript, pattern, turns)
				}
				results = append(results, hit)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("rows: %w", err)
			}
			if results == nil {
				results = []callGrepHit{}
			}
			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Filter to a specific agent ID")
	cmd.Flags().StringVar(&since, "since", "", "Only consider calls newer than this window (7d, 24h, or RFC3339)")
	cmd.Flags().IntVar(&turns, "turns", 0, "Include N transcript turns around the match")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum calls to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// looksLikeRegex reports whether the pattern contains characters that
// FTS5's MATCH operator would reject. We err on the side of using LIKE
// when in doubt because LIKE is slower but always correct.
func looksLikeRegex(p string) bool {
	for _, c := range p {
		switch c {
		case '.', '*', '+', '?', '(', ')', '[', ']', '|', '\\', '^', '$':
			return true
		}
	}
	return false
}

// makeSnippet returns up to 160 chars of context around the first
// case-insensitive occurrence of pattern in transcript.
func makeSnippet(transcript, pattern string) string {
	if transcript == "" {
		return ""
	}
	lc := strings.ToLower(transcript)
	idx := strings.Index(lc, strings.ToLower(pattern))
	if idx < 0 {
		if len(transcript) > 160 {
			return transcript[:160] + "…"
		}
		return transcript
	}
	start := idx - 60
	if start < 0 {
		start = 0
	}
	end := idx + len(pattern) + 60
	if end > len(transcript) {
		end = len(transcript)
	}
	out := transcript[start:end]
	if start > 0 {
		out = "…" + out
	}
	if end < len(transcript) {
		out = out + "…"
	}
	return out
}

// extractTurnWindow parses the transcript JSON (an array of turn objects),
// finds the first turn whose text contains pattern, and returns a window
// of N turns centered on it.
func extractTurnWindow(transcriptJSON, pattern string, n int) []any {
	var turns []any
	if err := json.Unmarshal([]byte(transcriptJSON), &turns); err != nil {
		return nil
	}
	if n <= 0 || len(turns) == 0 {
		return turns
	}
	hit := -1
	lp := strings.ToLower(pattern)
	for i, t := range turns {
		obj, ok := t.(map[string]any)
		if !ok {
			continue
		}
		for _, k := range []string{"text", "content", "message"} {
			if v, ok := obj[k]; ok {
				if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), lp) {
					hit = i
					break
				}
			}
		}
		if hit >= 0 {
			break
		}
	}
	if hit < 0 {
		return turns
	}
	half := n / 2
	if half < 1 {
		half = 1
	}
	start := hit - half
	if start < 0 {
		start = 0
	}
	end := hit + half + 1
	if end > len(turns) {
		end = len(turns)
	}
	return turns[start:end]
}
