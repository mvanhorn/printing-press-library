// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: hifz (memorization) tracker, per surah.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

type qkHifz struct {
	Surah  int    `json:"surah"`
	Marked string `json:"marked_at"`
}

// pp:data-source local
func newNovelHifzCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "hifz",
		Short:       "Mark and review Qur'an memorization progress per surah.",
		Long:        "Track which surahs you have memorized. State is stored locally and works offline.",
		Example:     "  quranku-pp-cli hifz mark 1",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newHifzMarkCmd(flags, true))
	cmd.AddCommand(newHifzMarkCmd(flags, false))
	cmd.AddCommand(newHifzListCmd(flags))
	return cmd
}

func newHifzMarkCmd(flags *rootFlags, mark bool) *cobra.Command {
	use, verb := "mark <surah>", "mark"
	short := "Mark a surah as memorized"
	if !mark {
		use, verb = "unmark <surah>", "unmark"
		short = "Remove the memorized mark from a surah"
	}
	return &cobra.Command{
		Use:         use,
		Short:       short,
		Example:     "  quranku-pp-cli hifz " + verb + " 1",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, verb+" a surah")
			}
			if len(args) == 0 {
				return qkInputError(cmd, flags, "a surah number (1-114) is required")
			}
			surah, err := strconv.Atoi(args[0])
			if err != nil || surah < 1 || surah > 114 {
				return qkInputError(cmd, flags, fmt.Sprintf("invalid surah number %q; expected 1-114", args[0]))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			id := strconv.Itoa(surah)
			if mark {
				rec := qkHifz{Surah: surah, Marked: time.Now().UTC().Format(time.RFC3339)}
				b, _ := json.Marshal(rec)
				if err := s.Upsert("hifz", id, b); err != nil {
					return err
				}
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "marked surah %d as memorized\n", surah)
				return nil
			}
			res, err := s.DB().ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'hifz' AND id = ?`, id)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"surah": surah, "unmarked": false}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "surah %d was not marked\n", surah)
				return nil
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"surah": surah, "unmarked": true}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "unmarked surah %d\n", surah)
			return nil
		},
	}
}

func newHifzListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List memorized surahs and overall progress",
		Example:     "  quranku-pp-cli hifz list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "list memorized surahs")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.List("hifz", 0)
			if err != nil {
				return err
			}
			marked := make([]int, 0, len(rows))
			for _, r := range rows {
				var h qkHifz
				if json.Unmarshal(r, &h) == nil {
					marked = append(marked, h.Surah)
				}
			}
			view := map[string]any{
				"memorized_surahs": marked,
				"memorized_count":  len(marked),
				"total_surahs":     114,
				"percent":          fmt.Sprintf("%.1f", float64(len(marked))/114.0*100),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "memorized %d/114 surahs (%.1f%%)\n", len(marked), float64(len(marked))/114.0*100)
			if len(marked) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "surahs: %v\n", marked)
			}
			return nil
		},
	}
}
