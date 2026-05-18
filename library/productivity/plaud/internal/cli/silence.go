// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newSilenceCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagPeople string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "silence",
		Short: "Speakers who used to appear but haven't in N days",
		Long: "Aggregates the last time each speaker appeared in your recordings.\n" +
			"Returns speakers whose last_heard is more than --days ago, with the\n" +
			"recording they last appeared in and a sample of their last spoken topic.\n" +
			"For managers and operators: which reports, customers, or stakeholders\n" +
			"have gone quiet?",
		Example: `  plaud-pp-cli silence --days 21
  plaud-pp-cli silence --days 30 --people "Sandra,Marcus,John"
  plaud-pp-cli silence --json --select speaker,last_heard,days_silent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			s, err := openPlaudStore(cmd.Context())
			var _ *store.Store = s
			if err != nil {
				return err
			}
			defer s.Close()

			threshold := int64(0)
			if flagDays > 0 {
				threshold = nowUnix() - int64(flagDays)*86400
			}

			query := `
				SELECT t.speaker AS speaker,
				       MAX(r.start_time) AS last_heard,
				       COUNT(DISTINCT r.id) AS appearance_count,
				       MAX(t.content) AS last_topic_sample
				FROM transcripts t
				JOIN recordings_typed r ON r.id = t.recording_id
				WHERE t.speaker IS NOT NULL AND t.speaker != '' AND r.is_trash = 0
			`
			args2 := []any{}
			if flagPeople != "" {
				names := strings.Split(flagPeople, ",")
				placeholders := make([]string, len(names))
				for i, n := range names {
					placeholders[i] = "?"
					args2 = append(args2, strings.TrimSpace(n))
				}
				query += fmt.Sprintf(" AND t.speaker IN (%s)", strings.Join(placeholders, ","))
			}
			query += " GROUP BY t.speaker"
			if threshold > 0 {
				query += " HAVING last_heard < ?"
				args2 = append(args2, threshold)
			}
			query += " ORDER BY last_heard ASC LIMIT ?"
			args2 = append(args2, flagLimit)

			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("silence aggregate: %w", err))
			}

			now := nowUnix()
			for _, r := range rows {
				lastHeard := toInt64(r["last_heard"])
				if lastHeard > 0 {
					r["days_silent"] = int((now - lastHeard) / 86400)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 21, "Silent threshold in days")
	cmd.Flags().StringVar(&flagPeople, "people", "", "Comma-separated list of speakers to check (default: all)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max speakers")
	return cmd
}

func nowUnix() int64 {
	return time.Now().Unix()
}
