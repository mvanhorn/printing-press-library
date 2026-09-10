// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"math"

	"github.com/spf13/cobra"
)

type adaptationView struct {
	AnimeID            int     `json:"anime_id"`
	AnimeTitle         string  `json:"anime_title,omitempty"`
	AnimeEpisodes      int     `json:"anime_episodes,omitempty"`
	AnimeStatus        string  `json:"anime_status,omitempty"`
	MangaID            int     `json:"manga_id,omitempty"`
	MangaTitle         string  `json:"manga_title,omitempty"`
	MangaChapters      int     `json:"manga_chapters,omitempty"`
	MangaVolumes       int     `json:"manga_volumes,omitempty"`
	MangaStatus        string  `json:"manga_status,omitempty"`
	Relation           string  `json:"relation,omitempty"`
	ChaptersPerEpisode float64 `json:"chapters_per_episode,omitempty"`
	CoveredLow         int     `json:"estimated_chapters_covered_low,omitempty"`
	CoveredHigh        int     `json:"estimated_chapters_covered_high,omitempty"`
	RemainingLow       int     `json:"estimated_chapters_remaining_low,omitempty"`
	RemainingHigh      int     `json:"estimated_chapters_remaining_high,omitempty"`
	SourceOngoing      bool    `json:"source_still_publishing"`
	Note               string  `json:"note"`
}

// newNovelAdaptationCmd joins a cached anime entry with its related manga entry
// to answer "how much of the source did the anime cover?". MyAnimeList never
// states coverage, so this reports a deliberately wide band rather than a
// fabricated chapter number.
func newNovelAdaptationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adaptation <anime-id>",
		Short: "Estimate how much source manga an anime adaptation covered",
		Long: "Use this command for whether an anime's adaptation covered its source manga and how much source remains.\n" +
			"Do NOT use this command for other entries in the same franchise (sequels, movies, side stories); use 'franchise gap' instead.\n\n" +
			"The estimate is a band, not a chapter number: MyAnimeList does not record where an adaptation stopped.",
		Example: "  myanimelist-pp-cli adaptation 52991 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     "id=52991",
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "adaptation")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an anime id is required, e.g. myanimelist-pp-cli adaptation 52991"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			anime, err := malDetailWith(ctx, c, "anime", id)
			if err != nil {
				return err
			}
			view := adaptationView{
				AnimeID: anime.ID, AnimeTitle: anime.Title,
				AnimeEpisodes: anime.Episodes, AnimeStatus: anime.Status,
			}
			mangaID, relation := 0, ""
			for _, rel := range anime.Related {
				if rel.Kind != "manga" {
					continue
				}
				if rel.Relation == "Adaptation" || mangaID == 0 {
					mangaID, relation = rel.ID, rel.Relation
				}
				if rel.Relation == "Adaptation" {
					break
				}
			}
			if mangaID == 0 {
				return printAdaptation(cmd, flags, view, "no manga entry is linked from this anime's Related Entries; nothing to compare")
			}
			manga, err := malDetailWith(ctx, c, "manga", mangaID)
			if err != nil {
				return err
			}
			view.MangaID, view.MangaTitle = manga.ID, manga.Title
			view.MangaChapters, view.MangaVolumes, view.MangaStatus = manga.Chapters, manga.Volumes, manga.Status
			view.Relation = relation
			view.SourceOngoing = manga.Status == "Publishing"
			if manga.Chapters > 0 && anime.Episodes > 0 {
				cpe := float64(manga.Chapters) / float64(anime.Episodes)
				view.ChaptersPerEpisode = math.Round(cpe*100) / 100
				mid := float64(manga.Chapters)
				if anime.Status != "Finished Airing" {
					mid = cpe * float64(anime.Episodes)
				}
				low := int(math.Floor(mid * 0.8))
				high := int(math.Ceil(mid * 1.2))
				if high > manga.Chapters {
					high = manga.Chapters
				}
				view.CoveredLow, view.CoveredHigh = low, high
				if rem := manga.Chapters - high; rem > 0 {
					view.RemainingLow = rem
				}
				if rem := manga.Chapters - low; rem > 0 {
					view.RemainingHigh = rem
				}
			}
			note := "estimate derived from episode and chapter counts; MyAnimeList does not record where the adaptation stopped"
			if view.SourceOngoing {
				note += "; the source manga is still publishing"
			}
			if view.MangaChapters == 0 {
				note = "the manga entry publishes no chapter count, so only the link between the two entries is known"
			}
			return printAdaptation(cmd, flags, view, note)
		},
	}
	return cmd
}

func printAdaptation(cmd *cobra.Command, flags *rootFlags, view adaptationView, note string) error {
	view.Note = note
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%d eps, %s)\n", view.AnimeTitle, view.AnimeEpisodes, view.AnimeStatus)
	if view.MangaID == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "source: %s\n", note)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "source: %s (%d volumes, %d chapters, %s)\n", view.MangaTitle, view.MangaVolumes, view.MangaChapters, view.MangaStatus)
	if view.CoveredLow > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "estimated coverage: ~%d-%d of %d chapters (%.2f chapters/episode band)\n", view.CoveredLow, view.CoveredHigh, view.MangaChapters, view.ChaptersPerEpisode)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", note)
	return nil
}
