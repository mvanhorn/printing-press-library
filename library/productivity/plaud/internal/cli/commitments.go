// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

// commitmentLikePatterns are SQL LIKE prefilters — cheap candidate selection
// before the precise Go regex confirms. Together they cover the most common
// commitment-shaped phrases in business conversation.
var commitmentLikePatterns = []string{
	"%I'll %", "%I will %", "%let me %", "%I can %",
	"%by EOD%", "%by EOW%", "%by Friday%", "%by next week%",
	"%I'll send%", "%I'll follow up%", "%I'll get back%",
	"%I'll circle back%", "%I'll loop you in%",
}

var commitmentGoRegex = regexp.MustCompile(`(?i)\b(I'?ll|I will|let me|I can(?:'t)?|by (?:EOD|EOW|Friday|next week|end of))\b`)

func newCommitmentsCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagByPerson, flagOpen bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "commitments",
		Short: "Surface every promise you made across all recordings",
		Long: "Scans speaker-diarized transcripts for commitment-shaped phrases\n" +
			"(\"I'll\", \"I will\", \"let me\", \"by EOW\") and lists each one with the\n" +
			"speaker, date, and recording. Use --open to filter to commitments\n" +
			"that have no follow-up signal in any later recording.\n\n" +
			"Note: the --open filter currently marks every commitment as open;\n" +
			"the precise no-follow-up left-anti-join is a v0.2 enhancement.",
		Example: `  plaud-pp-cli commitments --since 30d
  plaud-pp-cli commitments --open --by-person --agent
  plaud-pp-cli commitments --since 7d --json --select speaker,content,recording_id`,
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
				       r.filename, r.start_time AS recording_start, r.scene
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
			if flagLimit > 0 {
				query += " LIMIT ?"
				args2 = append(args2, flagLimit*4) // pull extra; regex will trim
			}

			candidates, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("commitments scan: %w", err))
			}

			confirmed := make([]map[string]any, 0, len(candidates))
			now := time.Now().Unix()
			for _, row := range candidates {
				content, _ := row["content"].(string)
				if !commitmentGoRegex.MatchString(content) {
					continue
				}
				row["days_ago"] = daysAgoFrom(toInt64(row["recording_start"]), now)
				if flagOpen {
					row["is_open"] = true
				}
				confirmed = append(confirmed, row)
				if flagLimit > 0 && len(confirmed) >= flagLimit {
					break
				}
			}

			if flagByPerson {
				return printJSONFiltered(cmd.OutOrStdout(), groupRowsBySpeaker(confirmed), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), confirmed, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Only commitments from recordings within this window (e.g. 30d, 12h, ISO)")
	cmd.Flags().BoolVar(&flagByPerson, "by-person", false, "Group output by speaker")
	cmd.Flags().BoolVar(&flagOpen, "open", false, "Mark commitments as open (no follow-up signal). v1 marks all as open; v0.2 will add left-anti-join.")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Max results")
	return cmd
}

func groupRowsBySpeaker(rows []map[string]any) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, r := range rows {
		speaker, _ := r["speaker"].(string)
		if strings.TrimSpace(speaker) == "" {
			speaker = "<unknown>"
		}
		out[speaker] = append(out[speaker], r)
	}
	return out
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case []byte:
		var i int64
		fmt.Sscanf(string(x), "%d", &i)
		return i
	case string:
		var i int64
		fmt.Sscanf(x, "%d", &i)
		return i
	}
	return 0
}

func daysAgoFrom(epoch, now int64) int {
	if epoch == 0 {
		return -1
	}
	delta := now - epoch
	if delta < 0 {
		return 0
	}
	return int(delta / 86400)
}
