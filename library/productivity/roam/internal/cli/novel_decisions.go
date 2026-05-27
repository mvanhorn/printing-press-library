package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/roam/internal/store"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// decisionPatterns are decision-anchored phrases. Lines that match are
// emitted as candidate decisions. Tuned for English meeting transcripts.
var decisionPatterns = []string{
	`(?i)\b(we|i|let's|lets)\s+(decided|agreed|concluded|chose|will|are going to)\b`,
	`(?i)\b(action item|action items|todo|to do|next steps?)\b[:\-]`,
	`(?i)\b(decision|agreed)[:\-]`,
	`(?i)\blet's go with\b`,
	`(?i)\bsign(ed)? off on\b`,
}

func newDecisionsCmd(flags *rootFlags) *cobra.Command {
	var since, inGroup string
	var limit int

	cmd := &cobra.Command{
		Use:   "decisions",
		Short: `Surface decision-shaped lines ("we decided", "action item", "agreed") from synced transcripts`,
		Long: `Scans the local transcript_text store for decision-anchored phrases and prints one
row per match with the transcript ID and snippet. No LLM, no API calls.

Run 'roam-pp-cli sync' first to populate the local store.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:store-read — scans the local transcript_text table populated by sync
			var _ store.Store
			var db *sql.DB
			db, closeDB, err := openNovelDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer closeDB()
			if err := ensureMessagesTables(cmd.Context(), db); err != nil {
				return apiErr(err)
			}

			cutoff := ""
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(err)
				}
				cutoff = ts.UTC().Format(time.RFC3339)
			}

			sql := `SELECT transcript_id, event_name, start, text FROM transcript_text WHERE 1=1`
			a := []any{}
			if cutoff != "" {
				sql += ` AND start >= ?`
				a = append(a, cutoff)
			}
			sql += ` ORDER BY start DESC`
			rows, err := db.QueryContext(cmd.Context(), sql, a...)
			if err != nil {
				return apiErr(fmt.Errorf("query transcripts: %w", err))
			}
			defer rows.Close()

			compiled := make([]*regexp.Regexp, 0, len(decisionPatterns))
			for _, p := range decisionPatterns {
				re, err := regexp.Compile(p)
				if err != nil {
					continue
				}
				compiled = append(compiled, re)
			}

			type decision struct {
				TranscriptID string `json:"transcript_id"`
				EventName    string `json:"event_name"`
				Start        string `json:"start"`
				Line         string `json:"line"`
				Pattern      string `json:"pattern"`
			}
			out := []decision{}
			_ = inGroup // group filter requires meeting->group join; not implemented in v1
			if limit <= 0 {
				limit = 100
			}
			for rows.Next() && len(out) < limit {
				var d decision
				var text string
				if err := rows.Scan(&d.TranscriptID, &d.EventName, &d.Start, &text); err != nil {
					continue
				}
				for _, line := range strings.Split(text, "\n") {
					if len(out) >= limit {
						break
					}
					trimmed := strings.TrimSpace(line)
					if trimmed == "" {
						continue
					}
					for _, re := range compiled {
						if re.MatchString(trimmed) {
							d2 := d
							d2.Line = trimmed
							d2.Pattern = re.String()
							out = append(out, d2)
							break
						}
					}
				}
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				body, _ := json.Marshal(map[string]any{"decisions": out})
				fmt.Fprintln(w, string(body))
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintln(w, "(no decisions found — run 'roam-pp-cli sync' to populate transcript_text)")
				return nil
			}
			for _, d := range out {
				fmt.Fprintf(w, "%s  %s\n  %s\n", d.Start, d.EventName, truncate(d.Line, 200))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only scan transcripts since (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&inGroup, "in-group", "", "Restrict to a specific group (reserved; not yet active)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max decisions to emit (default 100)")
	return cmd
}
