package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newTranscriptFanoutCmd(flags *rootFlags) *cobra.Command {
	var question, since string
	var limit int

	cmd := &cobra.Command{
		Use:   "transcript-fanout",
		Short: "Run a question against every transcript in a date range; one row per transcript",
		Long: `Reads transcript IDs from the local store (synced via 'transcript-list'), then calls
/transcript.prompt for each one and emits a row per transcript with the answer + citation.

Example:
  roam-pp-cli sync transcripts
  roam-pp-cli transcript fanout --question "Did we decide on pricing?" --since 7d`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if question == "" {
				return usageErr(fmt.Errorf("--question is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, closeDB, err := openNovelDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer closeDB()

			cutoff := ""
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(err)
				}
				cutoff = ts.UTC().Format(time.RFC3339)
			}

			query := `SELECT id, event_name, start FROM transcript_info WHERE 1=1`
			args2 := []any{}
			if cutoff != "" {
				query += ` AND start >= ?`
				args2 = append(args2, cutoff)
			}
			query += ` ORDER BY start DESC`
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}
			rows, err := db.Query(query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("query transcripts: %w", err))
			}
			defer rows.Close()

			type result struct {
				ID        string `json:"transcript_id"`
				EventName string `json:"event_name"`
				Start     string `json:"start"`
				Answer    string `json:"answer,omitempty"`
				Error     string `json:"error,omitempty"`
			}
			out := []result{}
			for rows.Next() {
				var r result
				if err := rows.Scan(&r.ID, &r.EventName, &r.Start); err != nil {
					continue
				}
				body := map[string]any{"id": r.ID, "prompt": question}
				resp, code, err := c.Post("/transcript.prompt", body)
				if err != nil || code >= 400 {
					if err != nil {
						r.Error = err.Error()
					} else {
						r.Error = fmt.Sprintf("HTTP %d", code)
					}
				} else {
					var parsed map[string]any
					if err := json.Unmarshal(resp, &parsed); err == nil {
						if a, ok := parsed["answer"].(string); ok {
							r.Answer = a
						} else if d, ok := parsed["data"].(map[string]any); ok {
							if a, ok := d["answer"].(string); ok {
								r.Answer = a
							}
						}
					}
				}
				out = append(out, r)
			}

			body, _ := json.Marshal(map[string]any{"question": question, "results": out})
			fmt.Fprintln(cmd.OutOrStdout(), string(body))
			return nil
		},
	}
	cmd.Flags().StringVar(&question, "question", "", "Question to ask each transcript (required)")
	cmd.Flags().StringVar(&since, "since", "", "Only consider transcripts started since (e.g. 7d, 24h, 30m)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max transcripts to fan out across (0 = no limit)")
	return cmd
}
