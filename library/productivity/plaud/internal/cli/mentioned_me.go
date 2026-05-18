// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newMentionedMeCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagName string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "mentioned-me",
		Short: "Third-party mentions of you across all transcripts",
		Long: "Reads your display name from the cached /user/me row (after sync),\n" +
			"then runs FTS5 MATCH for that name across all transcripts WHERE the\n" +
			"speaker is not you. Returns every time someone else said your name.\n\n" +
			"Override the auto-detected name with --name when sync hasn't run yet,\n" +
			"when your Plaud profile name differs from how people address you, or\n" +
			"when you want to check nicknames.",
		Example: `  plaud-pp-cli mentioned-me --since 90d
  plaud-pp-cli mentioned-me --name "Justin"
  plaud-pp-cli mentioned-me --json --select recording_start,speaker,content`,
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

			myName := strings.TrimSpace(flagName)
			if myName == "" {
				cached, err := s.CachedUserName()
				if err == nil {
					myName = cached
				}
			}
			if myName == "" {
				return usageErr(fmt.Errorf("could not determine your name; run `plaud-pp-cli sync` or pass --name"))
			}

			query := `
				SELECT t.recording_id, t.idx, t.start_time AS segment_start, t.speaker, t.content,
				       r.filename, r.start_time AS recording_start, r.scene
				FROM transcripts_fts
				JOIN transcripts t ON t.rowid = transcripts_fts.rowid
				JOIN recordings_typed r ON r.id = t.recording_id
				WHERE transcripts_fts MATCH ? AND (t.speaker IS NULL OR t.speaker NOT LIKE ?) AND r.is_trash = 0
			`
			args2 := []any{myName, "%" + myName + "%"}
			if since > 0 {
				query += " AND r.start_time >= ?"
				args2 = append(args2, since)
			}
			query += " ORDER BY r.start_time DESC, t.idx ASC LIMIT ?"
			args2 = append(args2, flagLimit)

			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("mentioned-me query: %w", err))
			}

			out := map[string]any{
				"my_name":  myName,
				"mentions": rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Only mentions from recordings within this window")
	cmd.Flags().StringVar(&flagName, "name", "", "Your name to search for (defaults to cached /user/me display name)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max mentions")
	return cmd
}
