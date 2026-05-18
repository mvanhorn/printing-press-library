// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newForgottenCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagByPerson bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "forgotten",
		Short: "Commitments with no follow-up signal in any later recording",
		Long: "Reuses the commitments scan, then for each commitment extracts a\n" +
			"few distinctive content tokens and checks whether ANY of them reappear\n" +
			"in any later transcript segment by any speaker. Commitments with no\n" +
			"such reappearance are flagged as forgotten.",
		Example: `  plaud-pp-cli forgotten --since 90d
  plaud-pp-cli forgotten --by-person --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := parseSinceFlag(flagSince)
			if err != nil {
				return usageErr(err)
			}

			s, err := openPlaudStore(cmd.Context())
			var _ *store.Store = s
			if err != nil {
				return err
			}
			defer s.Close()

			likeClause := ""
			likeArgs := []any{}
			for i, p := range commitmentLikePatterns {
				if i == 0 {
					likeClause = "t.content LIKE ?"
				} else {
					likeClause += " OR t.content LIKE ?"
				}
				likeArgs = append(likeArgs, p)
			}
			query := fmt.Sprintf(`
				SELECT t.recording_id, t.idx, t.start_time AS segment_start, t.speaker, t.content,
				       r.filename, r.start_time AS recording_start
				FROM transcripts t
				JOIN recordings_typed r ON r.id = t.recording_id
				WHERE (%s) AND r.is_trash = 0
			`, likeClause)
			args2 := likeArgs
			if since > 0 {
				query += " AND r.start_time >= ?"
				args2 = append(args2, since)
			}
			query += " ORDER BY r.start_time DESC, t.idx ASC"

			candidates, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("forgotten scan: %w", err))
			}

			forgotten := make([]map[string]any, 0)
			now := time.Now().Unix()
			for _, row := range candidates {
				content, _ := row["content"].(string)
				if !commitmentGoRegex.MatchString(content) {
					continue
				}
				tokens := topContentTokens(content, 3)
				recStart := toInt64(row["recording_start"])

				if len(tokens) > 0 && hasLaterMention(cmd.Context(), s.DB(), recStart, tokens) {
					continue // followed up — not forgotten
				}
				row["days_ago"] = daysAgoFrom(recStart, now)
				row["unfollowed_tokens"] = tokens
				forgotten = append(forgotten, row)
				if flagLimit > 0 && len(forgotten) >= flagLimit {
					break
				}
			}

			if flagByPerson {
				return printJSONFiltered(cmd.OutOrStdout(), groupRowsBySpeaker(forgotten), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), forgotten, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Look back this far for commitments")
	cmd.Flags().BoolVar(&flagByPerson, "by-person", false, "Group output by speaker")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max forgotten commitments")
	return cmd
}

// topContentTokens returns up to n distinctive tokens from the text following
// the commitment verb. Strips stop-words. Used by the forgotten follow-up
// check.
func topContentTokens(content string, n int) []string {
	loc := commitmentGoRegex.FindStringIndex(content)
	if loc == nil {
		return nil
	}
	tail := content[loc[1]:]
	tail = strings.ToLower(tail)
	words := strings.FieldsFunc(tail, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "of": true, "in": true, "on": true, "at": true,
		"to": true, "for": true, "with": true, "you": true, "i": true,
		"me": true, "my": true, "our": true, "we": true, "it": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"will": true, "can": true, "let": true, "ll": true,
	}
	out := make([]string, 0, n)
	seen := map[string]bool{}
	for _, w := range words {
		if len(w) < 4 || stop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) >= n {
			break
		}
	}
	return out
}

// hasLaterMention returns true if any of tokens appears in any transcript
// segment from a recording with start_time > recStart. One query, OR'd
// LIKE patterns.
func hasLaterMention(ctx context.Context, db *sql.DB, recStart int64, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	clause := ""
	args2 := []any{recStart}
	for i, tok := range tokens {
		if i == 0 {
			clause = "t.content LIKE ?"
		} else {
			clause += " OR t.content LIKE ?"
		}
		args2 = append(args2, "%"+tok+"%")
	}
	query := fmt.Sprintf(`
		SELECT 1 FROM transcripts t
		JOIN recordings_typed r ON r.id = t.recording_id
		WHERE r.start_time > ? AND (%s) AND r.is_trash = 0
		LIMIT 1
	`, clause)
	var hit int
	err := db.QueryRowContext(ctx, query, args2...).Scan(&hit)
	return err == nil && hit > 0
}
