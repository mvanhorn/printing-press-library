// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newTopicCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagBucket string

	cmd := &cobra.Command{
		Use:   "topic [term]",
		Short: "Trace how often a topic comes up across recordings over time",
		Long: "Counts mentions of <term> per time bucket. Shows whether a topic\n" +
			"is emerging or fading. Each bucket includes the speakers anchoring it\n" +
			"and a sample of recording IDs.",
		Example: `  plaud-pp-cli topic "pricing" --since 90d
  plaud-pp-cli topic "launch" --bucket day --json
  plaud-pp-cli topic "renewal" --since 180d --bucket week --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			term := strings.TrimSpace(args[0])
			if term == "" {
				return usageErr(fmt.Errorf("topic term required"))
			}
			since, err := parseSinceFlag(flagSince)
			if err != nil {
				return usageErr(err)
			}
			bucketFmt := "%Y-W%W"
			if flagBucket == "day" {
				bucketFmt = "%Y-%m-%d"
			} else if flagBucket == "month" {
				bucketFmt = "%Y-%m"
			}

			s, err := openPlaudStore(cmd.Context())
			var _ *store.Store = s
			if err != nil {
				return err
			}
			defer s.Close()

			query := fmt.Sprintf(`
				SELECT strftime('%s', datetime(r.start_time, 'unixepoch')) AS bucket,
				       COUNT(*) AS mentions,
				       GROUP_CONCAT(DISTINCT t.speaker) AS speakers,
				       GROUP_CONCAT(DISTINCT t.recording_id) AS recording_ids
				FROM transcripts_fts
				JOIN transcripts t ON t.rowid = transcripts_fts.rowid
				JOIN recordings_typed r ON r.id = t.recording_id
				WHERE transcripts_fts MATCH ? AND r.is_trash = 0
			`, bucketFmt)
			args2 := []any{term}
			if since > 0 {
				query += " AND r.start_time >= ?"
				args2 = append(args2, since)
			}
			query += " GROUP BY bucket ORDER BY bucket ASC"

			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("topic query: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Only mentions from recordings within this window")
	cmd.Flags().StringVar(&flagBucket, "bucket", "week", "Time bucket: day, week, or month")
	return cmd
}
