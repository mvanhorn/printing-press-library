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
	ReachedEpisodes    int     `json:"reached_episodes,omitempty"`
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

// adaptationBand converts an episode reach into a chapter band. MyAnimeList
// records no stopping point, so the band is deliberately wide. What it must not
// do is assume the adaptation finished: chaptersPerEpisode is derived from the
// announced episode total (the only denominator MyAnimeList publishes) and then
// scaled by how many episodes have actually been reached, and the band is capped
// at the source's chapter count. ok is false when any count is unknown.
func adaptationBand(mangaChapters, announcedEpisodes, reachedEpisodes int) (chaptersPerEpisode float64, low, high int, ok bool) {
	if mangaChapters <= 0 || announcedEpisodes <= 0 || reachedEpisodes <= 0 {
		return 0, 0, 0, false
	}
	perEpisode := float64(mangaChapters) / float64(announcedEpisodes)
	chaptersPerEpisode = math.Round(perEpisode*100) / 100
	mid := min(perEpisode*float64(reachedEpisodes), float64(mangaChapters))
	low = int(math.Floor(mid * 0.8))
	high = min(int(math.Ceil(mid*1.2)), mangaChapters)
	high = max(high, low)
	return chaptersPerEpisode, low, high, true
}

// adaptationNote explains how the coverage band was derived, says plainly when
// no band could be produced, and flags the two ways the estimate can understate
// the source: the anime has not finished airing, or the manga is still running.
func adaptationNote(mangaChapters int, bandComputed, stillAiring, sourceOngoing bool) string {
	if mangaChapters <= 0 {
		return "the manga entry publishes no chapter count, so only the link between the two entries is known"
	}
	note := "estimate derived from episode and chapter counts; MyAnimeList does not record where the adaptation stopped"
	if !bandComputed {
		note = "no chapter band could be estimated because the anime's announced episode total is unknown; MyAnimeList does not record where the adaptation stopped"
	}
	if stillAiring {
		note += "; the anime is still airing, so the source may not yet be fully covered"
	}
	if sourceOngoing {
		note += "; the source manga is still publishing"
	}
	return note
}

// airedSummary describes how far the adaptation has aired, or "" when there is
// nothing worth saying. It never claims a fraction of an unknown total.
func airedSummary(announcedEpisodes, reachedEpisodes int) string {
	switch {
	case reachedEpisodes <= 0:
		return ""
	case announcedEpisodes <= 0:
		return fmt.Sprintf("aired so far: %d episodes (announced total unknown)", reachedEpisodes)
	case reachedEpisodes != announcedEpisodes:
		return fmt.Sprintf("aired so far: %d of %d announced episodes", reachedEpisodes, announcedEpisodes)
	default:
		return ""
	}
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
			// anime.Episodes is the announced total, not the reached total. For
			// a show that is still airing (or whose total is unknown) it
			// overstates how far the adaptation got, so the reached count comes
			// from the episode table instead.
			stillAiring := anime.Status != "Finished Airing"
			reachedEpisodes := anime.Episodes
			if stillAiring || reachedEpisodes <= 0 {
				eps, eerr := malEpisodesWith(ctx, c, id)
				if eerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read the episode table (%v); falling back to the announced episode total\n", eerr)
				} else if aired := airedEpisodeCount(eps); aired > 0 {
					reachedEpisodes = aired
				}
			}
			view.ReachedEpisodes = reachedEpisodes
			bandComputed := false
			if cpe, low, high, ok := adaptationBand(manga.Chapters, anime.Episodes, reachedEpisodes); ok {
				bandComputed = true
				view.ChaptersPerEpisode = cpe
				view.CoveredLow, view.CoveredHigh = low, high
				if rem := manga.Chapters - high; rem > 0 {
					view.RemainingLow = rem
				}
				if rem := manga.Chapters - low; rem > 0 {
					view.RemainingHigh = rem
				}
			}
			note := adaptationNote(manga.Chapters, bandComputed, stillAiring, view.SourceOngoing)
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
	if line := airedSummary(view.AnimeEpisodes, view.ReachedEpisodes); line != "" {
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
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
