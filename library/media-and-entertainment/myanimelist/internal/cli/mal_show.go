// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/malhtml"
)

// newAnimeShowCmd / newMangaShowCmd expose the typed record parsed from a
// MyAnimeList detail page. The generated `anime get` / `manga get` commands stay
// as the raw plumbing (they return the page envelope), while `show` returns the
// structured fields a user or agent actually wants: titles, type, episode or
// chapter counts, airing window, broadcast slot, studios, genres, score, rank,
// popularity, members, favorites, synopsis, and the related-entries graph.
func newAnimeShowCmd(flags *rootFlags) *cobra.Command { return newTitleShowCmd(flags, "anime") }
func newMangaShowCmd(flags *rootFlags) *cobra.Command { return newTitleShowCmd(flags, "manga") }

// happyArgsFor supplies the live-dogfood fixture id for a title kind. MyAnimeList
// ids are per-kind: 52991 is an anime, not a manga.
func happyArgsFor(kind string) string {
	if kind == "manga" {
		return "id=2"
	}
	return "id=52991"
}

func newTitleShowCmd(flags *rootFlags, kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a title as a typed record parsed from its MyAnimeList page",
		Long: "Returns the structured title record (titles, type, counts, airing window, broadcast slot,\n" +
			"studios, genres, score, rank, popularity, members, favorites, synopsis, related entries)\n" +
			"parsed from the site's server-rendered page. Use the generated `" + kind + " get` command when you\n" +
			"want the raw page envelope instead.",
		Example: "  myanimelist-pp-cli " + kind + " show 52991 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     happyArgsFor(kind),
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, kind+" show")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a %s id is required, e.g. myanimelist-pp-cli %s show 52991", kind, kind))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			detail, err := malDetailWith(ctx, c, kind, id)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
			}
			return renderTitleHuman(cmd, detail)
		},
	}
	return cmd
}

func renderTitleHuman(cmd *cobra.Command, d *malhtml.Detail) error {
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", d.Title)
	if d.TitleEnglish != "" && d.TitleEnglish != d.Title {
		fmt.Fprintf(cmd.OutOrStdout(), "  English:   %s\n", d.TitleEnglish)
	}
	if d.TitleJapanese != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Japanese:  %s\n", d.TitleJapanese)
	}
	if d.Type != "" || d.Status != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Type:      %s (%s)\n", d.Type, d.Status)
	}
	if d.Episodes > 0 || d.Chapters > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Length:    %d eps, %d chapters, %d volumes\n", d.Episodes, d.Chapters, d.Volumes)
	}
	if d.Aired != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Aired:     %s\n", d.Aired)
	}
	if d.Published != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Published: %s\n", d.Published)
	}
	if d.Broadcast != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Broadcast: %s\n", d.Broadcast)
	}
	if len(d.Studios) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Studios:   %v\n", d.Studios)
	}
	if len(d.Genres) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Genres:    %v\n", d.Genres)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Score:     %.2f (%d users), rank #%d, popularity #%d, %d members\n",
		d.Score, d.ScoredBy, d.Rank, d.Popularity, d.Members)
	for _, rel := range d.Related {
		fmt.Fprintf(cmd.OutOrStdout(), "  Related:   %-18s %s %d %s\n", rel.Relation, rel.Kind, rel.ID, rel.Title)
	}
	return nil
}
