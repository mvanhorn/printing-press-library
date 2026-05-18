// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newCrossMeetingCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "cross-meeting [person] [topic]",
		Short: "Every utterance by a person about a topic, ordered chronologically (drift detection)",
		Long: "Returns every transcript segment by the named speaker that mentions\n" +
			"the topic, ordered by recording start time. Read it top to bottom to\n" +
			"see whether their position has drifted over time.",
		Example: `  plaud-pp-cli cross-meeting Marcus "launch date"
  plaud-pp-cli cross-meeting "Sandra" "renewal" --since 180d
  plaud-pp-cli cross-meeting Sandra pricing --json --select recording_start,content`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			person := strings.TrimSpace(args[0])
			topic := strings.TrimSpace(args[1])
			if person == "" || topic == "" {
				return usageErr(fmt.Errorf("both person and topic are required"))
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

			query := `
				SELECT t.recording_id, t.idx, t.start_time AS segment_start, t.speaker, t.content,
				       r.filename, r.start_time AS recording_start, r.scene
				FROM transcripts_fts
				JOIN transcripts t ON t.rowid = transcripts_fts.rowid
				JOIN recordings_typed r ON r.id = t.recording_id
				WHERE transcripts_fts MATCH ? AND t.speaker LIKE ? AND r.is_trash = 0
			`
			args2 := []any{topic, "%" + person + "%"}
			if since > 0 {
				query += " AND r.start_time >= ?"
				args2 = append(args2, since)
			}
			query += " ORDER BY r.start_time ASC, t.idx ASC LIMIT ?"
			args2 = append(args2, flagLimit)

			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("cross-meeting query: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "180d", "Only segments from recordings within this window")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max segments")
	return cmd
}
