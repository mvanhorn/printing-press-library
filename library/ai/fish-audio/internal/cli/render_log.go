// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: list past renders from the local render log.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
)

func newNovelRenderLogCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit int
		flagVoice string
		flagModel string
		flagSince string
		flagDB    string
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "See every past TTS render with its text, model, voice, byte count, and cost.",
		Long: `Lists individual past renders newest first, straight from the local render
log. Fish Audio keeps no server-side generation history, so this table is the
only record that a render happened and what it cost.

Use 'render spend' instead when the question is a total rather than a list.

The log is empty until the first 'tts render' or 'tts batch'. An empty log is
reported as [] at exit 0, never as an error.`,
		Example: strings.Trim(`
  fish-audio-pp-cli render log --limit 20
  fish-audio-pp-cli render log --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --since 30d --json
  fish-audio-pp-cli render log --model s2.1-pro-free --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "render log")
			}
			since, err := fishSince(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			dbPath := fishRenderDBPath(flagDB)
			if stop, mirrorErr := fishMissingMirror(cmd, flags, dbPath, make([]store.RenderRow, 0)); stop {
				return mirrorErr
			}
			db, err := openRenderStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.ListRenderRows(cmd.Context(), store.RenderLogFilter{
				Limit: flagLimit,
				Voice: flagVoice,
				Model: flagModel,
				Since: since,
			})
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No renders recorded yet. Run 'tts render' or 'tts batch' first.")
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "ID\tCREATED\tMODEL\tVOICE\tFORMAT\tBYTES IN\tCOST\tTEXT")
			for _, row := range rows {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d\t$%.6f\t%s\n",
					row.ID, row.CreatedAt, row.Model, truncate(row.VoiceID, 12), row.Format,
					row.BytesIn, row.CostUSD, truncate(strings.ReplaceAll(row.Text, "\n", " "), 40))
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows to return, newest first")
	cmd.Flags().StringVar(&flagVoice, "voice", "", "Show only renders that used this voice model_id")
	cmd.Flags().StringVar(&flagModel, "model", "", "Show only renders that used this TTS model")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show only renders newer than this duration (30d, 12h) or date (2026-08-01)")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
