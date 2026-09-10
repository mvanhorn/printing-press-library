// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newNovelTrackCmd owns the local, credential-free watch library. MyAnimeList's
// own list needs an OAuth session; this keeps an equivalent local record that
// `week`, `next`, `franchise gap`, `suggest`, and `export --format mal-xml`
// all read.
func newNovelTrackCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "track",
		Short:       "Keep a local watch library without a MyAnimeList account",
		Example:     "  myanimelist-pp-cli track add 52991 --status watching --progress 3",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTrackAddCmd(flags))
	cmd.AddCommand(newTrackProgressCmd(flags))
	cmd.AddCommand(newTrackRateCmd(flags))
	cmd.AddCommand(newTrackNoteCmd(flags))
	cmd.AddCommand(newTrackListCmd(flags))
	cmd.AddCommand(newTrackDropCmd(flags))
	cmd.AddCommand(newTrackRemoveCmd(flags))
	return cmd
}

type trackOpts struct {
	dbPath, kind, title, status, notes string
	progress, total, score             int
	episodes                           int
}

func trackFlags(cmd *cobra.Command, o *trackOpts) {
	cmd.Flags().StringVar(&o.dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&o.kind, "kind", "anime", "Entry kind (anime or manga)")
}

func trackOpen(cmd *cobra.Command, flags *rootFlags, o *trackOpts) (bool, error) {
	o.dbPath = malDBPath(flags, o.dbPath)
	if o.kind != "anime" && o.kind != "manga" {
		_ = cmd.Usage()
		return false, usageErr(fmt.Errorf("--kind must be anime or manga, got %q", o.kind))
	}
	return true, nil
}

func newTrackAddCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	cmd := &cobra.Command{
		Use:         "add <id>",
		Short:       "Add a title to the local library",
		Example:     "  myanimelist-pp-cli track add 52991 --status watching --progress 3",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=52991;--status=plan-to-watch", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track add")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a title id is required, e.g. myanimelist-pp-cli track add 52991 --status watching"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			if !malValidStatus(o.status) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--status must be one of %s, got %q", strings.Join(malStatuses, ", "), o.status))
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			entry := malLibraryEntry{Kind: o.kind, ID: id, Title: o.title, Status: o.status, Progress: o.progress, Total: o.total, Score: o.score, Notes: o.notes}
			if entry.Title == "" {
				if detail, derr := malDetail(ctx, flags, o.kind, id); derr == nil {
					entry.Title = detail.Title
					if entry.Total == 0 {
						if o.kind == "anime" {
							entry.Total = detail.Episodes
						} else {
							entry.Total = detail.Chapters
						}
					}
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch title details (%v); storing the id only\n", derr)
				}
			}
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := malWriteLibrary(ctx, db, &entry); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tracking %s %d (%s) as %s\n", entry.Kind, entry.ID, entry.Title, entry.Status)
			return nil
		},
	}
	trackFlags(cmd, o)
	cmd.Flags().StringVar(&o.status, "status", "plan-to-watch", "Library status: "+strings.Join(malStatuses, ", "))
	cmd.Flags().IntVar(&o.progress, "progress", 0, "Episodes watched or chapters read so far")
	cmd.Flags().IntVar(&o.total, "total", 0, "Total episodes/chapters (fetched from the title when omitted)")
	cmd.Flags().IntVar(&o.score, "score", 0, "Your score, 0-10")
	cmd.Flags().StringVar(&o.title, "title", "", "Title override (skips the detail fetch)")
	cmd.Flags().StringVar(&o.notes, "note", "", "Free-text note stored with the entry")
	return cmd
}

func newTrackProgressCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	var delta bool
	cmd := &cobra.Command{
		Use:         "progress <id>",
		Short:       "Set or advance watch progress for a tracked title",
		Example:     "  myanimelist-pp-cli track progress 52991 --episodes 4",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=52991;--episodes=1", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track progress")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a title id is required"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := malLoadLibrary(ctx, db, o.kind)
			if err != nil {
				return err
			}
			var found *malLibraryEntry
			for i := range entries {
				if entries[i].ID == id {
					found = &entries[i]
					break
				}
			}
			if found == nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("%s %d is not in the local library; add it first with `myanimelist-pp-cli track add %d --status watching`", o.kind, id, id))
			}
			if delta {
				found.Progress += o.episodes
			} else {
				found.Progress = o.episodes
			}
			if found.Total > 0 && found.Progress >= found.Total {
				found.Status = "completed"
			}
			if err := malWriteLibrary(ctx, db, found); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), *found, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d/%d (%s)\n", found.Title, found.Progress, found.Total, found.Status)
			return nil
		},
	}
	trackFlags(cmd, o)
	cmd.Flags().IntVar(&o.episodes, "episodes", 1, "Progress value to set (or to add when --increment is set)")
	cmd.Flags().BoolVar(&delta, "increment", false, "Add to the current progress instead of replacing it")
	return cmd
}

func newTrackRateCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	cmd := &cobra.Command{
		Use:         "rate <id>",
		Short:       "Set your local score for a tracked title",
		Example:     "  myanimelist-pp-cli track rate 52991 --score 9",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=52991;--score=8", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track rate")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a title id is required"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			if o.score < 0 || o.score > 10 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--score must be between 0 and 10, got %d", o.score))
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entry := malLibraryEntry{Kind: o.kind, ID: id, Status: "completed", Score: o.score}
			entries, err := malLoadLibrary(ctx, db, o.kind)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.ID == id {
					entry = e
					entry.Score = o.score
					break
				}
			}
			if err := malWriteLibrary(ctx, db, &entry); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s rated %d/10\n", entry.Title, entry.Score)
			return nil
		},
	}
	trackFlags(cmd, o)
	cmd.Flags().IntVar(&o.score, "score", 0, "Your score, 0-10")
	return cmd
}

func newTrackNoteCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	cmd := &cobra.Command{
		Use:         "note <id>",
		Short:       "Attach a note to a tracked title",
		Example:     "  myanimelist-pp-cli track note 52991 --text \"watch with subtitles\"",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=52991;--text=example", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track note")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a title id is required"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entry := malLibraryEntry{Kind: o.kind, ID: id, Status: "plan-to-watch", Notes: o.notes}
			entries, err := malLoadLibrary(ctx, db, o.kind)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.ID == id {
					entry = e
					entry.Notes = o.notes
					break
				}
			}
			if err := malWriteLibrary(ctx, db, &entry); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "note saved for %s\n", entry.Title)
			return nil
		},
	}
	trackFlags(cmd, o)
	cmd.Flags().StringVar(&o.notes, "text", "", "Note text")
	return cmd
}

func newTrackListCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List everything in the local library",
		Example:     "  myanimelist-pp-cli track list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--kind=anime", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track list")
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if !malStoreExists(o.dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: myanimelist-pp-cli track add 52991 --status watching --db %s\n", o.dbPath, o.dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]malLibraryEntry, 0), flags)
				}
				return nil
			}
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := malLoadLibrary(ctx, db, o.kind)
			if err != nil {
				return err
			}
			if o.status != "" {
				filtered := entries[:0]
				for _, e := range entries {
					if e.Status == o.status {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Local library is empty. Add a title with `myanimelist-pp-cli track add <id> --status watching`.")
				return nil
			}
			table := make([]map[string]any, 0, len(entries))
			for _, e := range entries {
				table = append(table, map[string]any{
					"id": e.ID, "title": e.Title, "status": e.Status,
					"progress": fmt.Sprintf("%d/%d", e.Progress, e.Total), "score": e.Score,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	trackFlags(cmd, o)
	cmd.Flags().StringVar(&o.status, "status", "", "Only list entries with this status")
	return cmd
}

func newTrackDropCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	cmd := &cobra.Command{
		Use:         "drop <id>",
		Short:       "Mark a tracked title dropped",
		Example:     "  myanimelist-pp-cli track drop 52991",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=52991", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track drop")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a title id is required"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entry := malLibraryEntry{Kind: o.kind, ID: id, Status: "dropped"}
			entries, err := malLoadLibrary(ctx, db, o.kind)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.ID == id {
					entry = e
					entry.Status = "dropped"
					break
				}
			}
			if err := malWriteLibrary(ctx, db, &entry); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s marked dropped\n", entry.Title)
			return nil
		},
	}
	trackFlags(cmd, o)
	return cmd
}

func newTrackRemoveCmd(flags *rootFlags) *cobra.Command {
	o := &trackOpts{}
	cmd := &cobra.Command{
		Use:         "remove <id>",
		Short:       "Remove a title from the local library",
		Example:     "  myanimelist-pp-cli track remove 52991",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=52991", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "track remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a title id is required"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			if ok, err := trackOpen(cmd, flags, o); !ok {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := malOpenStore(ctx, o.dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.DB().ExecContext(ctx, `DELETE FROM mal_library WHERE kind = ? AND id = ?`, o.kind, id); err != nil {
				return fmt.Errorf("removing library row: %w", err)
			}
			result := map[string]any{"kind": o.kind, "id": id, "removed": true}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s %d from the local library\n", o.kind, id)
			return nil
		},
	}
	trackFlags(cmd, o)
	return cmd
}
