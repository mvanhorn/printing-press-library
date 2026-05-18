// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newAboutCmd(flags *rootFlags) *cobra.Command {
	var flagTopic, flagSince string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "about [person]",
		Short: "Every utterance by one speaker, across every recording",
		Long: "Pulls every transcript segment by the named speaker, joined to the\n" +
			"recordings they appeared in. Optional --topic filter applies FTS5 MATCH\n" +
			"to narrow to a subject. Useful for pre-call prep: read everything a\n" +
			"person has said about a topic in one shot.",
		Example: `  plaud-pp-cli about Sandra --topic "renewal" --since 90d
  plaud-pp-cli about "John Smith" --json
  plaud-pp-cli about Sandra --topic pricing --agent --select start_time,content`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			person := strings.TrimSpace(args[0])
			if person == "" {
				return usageErr(fmt.Errorf("speaker name required"))
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

			var query string
			args2 := []any{}
			if flagTopic != "" {
				query = `
					SELECT t.recording_id, t.idx, t.start_time AS segment_start, t.speaker, t.content,
					       r.filename, r.start_time AS recording_start, r.scene
					FROM transcripts_fts
					JOIN transcripts t ON t.rowid = transcripts_fts.rowid
					JOIN recordings_typed r ON r.id = t.recording_id
					WHERE transcripts_fts MATCH ? AND t.speaker LIKE ? AND r.is_trash = 0`
				args2 = []any{flagTopic, "%" + person + "%"}
			} else {
				query = `
					SELECT t.recording_id, t.idx, t.start_time AS segment_start, t.speaker, t.content,
					       r.filename, r.start_time AS recording_start, r.scene
					FROM transcripts t
					JOIN recordings_typed r ON r.id = t.recording_id
					WHERE t.speaker LIKE ? AND r.is_trash = 0`
				args2 = []any{"%" + person + "%"}
			}
			if since > 0 {
				query += " AND r.start_time >= ?"
				args2 = append(args2, since)
			}
			query += " ORDER BY r.start_time DESC, t.idx ASC LIMIT ?"
			args2 = append(args2, flagLimit)

			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("about query: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagTopic, "topic", "", "Filter to segments matching this term (FTS5 syntax supported)")
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Only segments from recordings within this window")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Max segments")
	return cmd
}
