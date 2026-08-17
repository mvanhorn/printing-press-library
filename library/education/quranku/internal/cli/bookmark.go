// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: local verse bookmarks with notes.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type qkBookmark struct {
	Ref   string `json:"ref"`
	Note  string `json:"note,omitempty"`
	Added string `json:"added"`
}

// pp:data-source local
func newNovelBookmarkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bookmark",
		Short:       "Save verses with personal notes for later, offline.",
		Long:        "Manage a local list of bookmarked verses with optional personal notes. Fully offline.",
		Example:     "  quranku-pp-cli bookmark add 2:255 --note \"Ayat Kursi\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newBookmarkAddCmd(flags))
	cmd.AddCommand(newBookmarkListCmd(flags))
	cmd.AddCommand(newBookmarkRemoveCmd(flags))
	return cmd
}

func newBookmarkAddCmd(flags *rootFlags) *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:         "add <surah:verse>",
		Short:       "Bookmark a verse with an optional note",
		Example:     "  quranku-pp-cli bookmark add 2:255 --note \"Ayat Kursi\"",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "ref=2:255"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "add a bookmark")
			}
			if len(args) == 0 {
				return qkInputError(cmd, flags, "a verse reference (surah:verse) is required")
			}
			surah, verse, ok := qkParseRef(args[0])
			if !ok {
				return qkRefError(cmd, flags, args[0])
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			ref := fmt.Sprintf("%d:%d", surah, verse)
			bm := qkBookmark{Ref: ref, Note: note, Added: time.Now().UTC().Format(time.RFC3339)}
			b, _ := json.Marshal(bm)
			if err := s.Upsert("bookmark", ref, b); err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), bm, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bookmarked %s\n", ref)
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "an optional personal note for this verse")
	return cmd
}

func newBookmarkListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved bookmark references with their optional personal notes from the local store",
		Example:     "  quranku-pp-cli bookmark list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "list bookmarks")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.List("bookmark", 0)
			if err != nil {
				return err
			}
			out := make([]qkBookmark, 0, len(rows))
			for _, r := range rows {
				var bm qkBookmark
				if json.Unmarshal(r, &bm) == nil {
					out = append(out, bm)
				}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no bookmarks yet")
				return nil
			}
			for _, bm := range out {
				if bm.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n", bm.Ref, bm.Note)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", bm.Ref)
				}
			}
			return nil
		},
	}
}

func newBookmarkRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rm <surah:verse>",
		Short:       "Remove a saved bookmark by its surah:verse reference (idempotent, no error if absent)",
		Example:     "  quranku-pp-cli bookmark rm 2:255",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "ref=2:255"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "remove a bookmark")
			}
			if len(args) == 0 {
				return qkInputError(cmd, flags, "a verse reference (surah:verse) is required")
			}
			surah, verse, ok := qkParseRef(args[0])
			if !ok {
				return qkRefError(cmd, flags, args[0])
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			ref := fmt.Sprintf("%d:%d", surah, verse)
			res, err := s.DB().ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'bookmark' AND id = ?`, ref)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			// Idempotent delete: removing an absent bookmark is a no-op success,
			// not an error. Report whether a row was actually removed.
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"ref": ref, "removed": n > 0}, flags)
			}
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no bookmark for %s (nothing to remove)\n", ref)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed bookmark %s\n", ref)
			return nil
		},
	}
	return cmd
}
