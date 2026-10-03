// ratings.go — hand-written Slice B novel command (top-level) plus the
// multi-source helpers shared with series and similar.
// pp:data-source live — RAWG detail + keyless Steam storefront enrichment.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/steam"

	"github.com/spf13/cobra"
)

// ----- shared multi-source helpers (ratings, series, similar, retention) -----

// resolveTitleForMultiSource maps a title to one RAWG game for the
// multi-source commands. Unlike games get, remake ambiguity is a NOTICE,
// not an error: same-named games across release years are listed on stderr
// and in the returned candidates, and the best match still resolves so the
// rating card or comparison can proceed. Pin a remake with --year.
func resolveTitleForMultiSource(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, title, year string) (rawgGame, []ambiguousCandidate, error) {
	// A bare-numeric argument is a RAWG id, matching retention and games
	// get — the not-found hints below promise an id recovery path, so the
	// multi-source commands must honor it. Detail is fetched by the
	// caller via the returned ID.
	// PATCH(amend-2026-09-28: ratings/series/similar accept a bare RAWG id) — was title-only, contradicting the hinted id recovery path
	if id, ok := parseGameID(title); ok {
		return rawgGame{ID: id}, nil, nil
	}
	exact, ranked, nearby, err := resolveExactTitleMatches(ctx, cmd, c, flags, title, year)
	if err != nil {
		return rawgGame{}, nil, err
	}
	if len(exact) == 0 {
		// A pinned --year is a hard constraint on the title too:
		// resolveExactTitleMatches already filtered exact-title matches to
		// the requested year, so an empty set means no game with that
		// title was released that year in the top RAWG results. Never
		// fall back to an arbitrary year-matching hit — its title can be
		// a different game entirely. The one safe partial-title path is a
		// year-matching hit whose normalized name continues the query at
		// a word boundary ("the witcher 3" -> "The Witcher 3: Wild Hunt"):
		// an unrelated same-year game never passes that check.
		// PATCH(amend-2026-09-28: year pin must not resolve a different title) — was filterByReleaseYear + bestKnownGame fallback
		// PATCH(amend-2026-09-28: year-pinned partial titles resolve via a boundary-continuation hit) — was a hard not-found that broke "the witcher 3" --year 2015
		if year != "" {
			continuations := make([]rawgGame, 0, len(ranked))
			for _, g := range filterByReleaseYear(ranked, year) {
				if isFranchiseContinuation(title, g.Name) {
					continuations = append(continuations, g)
				}
			}
			if len(continuations) > 0 {
				best := bestKnownGame(continuations)
				fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q to %q (%s); no exact title match — year-pinned partial title resolved to a same-year continuation\n",
					title, best.Name, year)
				return best, nil, nil
			}
			return rawgGame{}, nil, notFoundErr(fmt.Errorf("no game titled %q released in %s in the top RAWG search results; drop --year to resolve across years, pass the full title, or use a RAWG id", title, year))
		}
		// No normalized exact match: fall back to the top-ranked search hit
		// with an explicit notice. Users type partial titles ("the witcher
		// 3") whose full form is the obvious #1 search result; refusing
		// them hides intent behind an error. Use the full title or a RAWG
		// id to pin a different game.
		if len(ranked) > 0 {
			// Best-known hit, not RAWG's first: search order is
			// relevance-shaped and can lead with an obscure title.
			best := bestKnownGame(ranked)
			fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q to %q (%s); no exact title match — use the full title or a RAWG id to pin\n",
				title, best.Name, orDash(yearOf(best.Released)))
			return best, nil, nil
		}
		return rawgGame{}, nil, notFoundErr(fmt.Errorf("no game titled %q in the top RAWG search results; %s", title,
			nearbyMatchHint(nearby, "try 'game-goat-pp-cli games search' or pin a remake with --year")))
	}
	var candidates []ambiguousCandidate
	if len(exact) > 1 {
		candidates = make([]ambiguousCandidate, 0, len(exact))
		for _, g := range exact {
			candidates = append(candidates, ambiguousCandidate{ID: g.ID, Name: g.Name, Released: g.Released})
		}
		writeAmbiguousNotice(cmd.ErrOrStderr(), candidates)
		return exact[0], candidates, nil
	}
	// Single exact match that is obscure while a franchise continuation
	// dominates it on community data: the user typed a franchise name
	// ("halo", "zelda") and RAWG's exact hit is a junk same-named title.
	// Skipped when --year is pinned: the override crosses release years,
	// and a pinned year means the user already chose the release.
	if year == "" {
		if best, ok := franchiseOverride(exact[0], ranked, title); ok {
			fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q to %q (franchise entry, strongest community data); an obscure game titled %q also exists — pass the full title, --year, or a RAWG id to pin it\n",
				title, best.Name, exact[0].Name)
			return best, nil, nil
		}
	}
	return exact[0], candidates, nil
}

// fetchResolvedGameDetail resolves a title (with a non-fatal remake
// ambiguity notice) and fetches its full RAWG detail record. Shared by
// ratings and the other title-taking commands.
func fetchResolvedGameDetail(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, title, year string) (rawgGame, []ambiguousCandidate, error) {
	match, candidates, err := resolveTitleForMultiSource(ctx, cmd, c, flags, title, year)
	if err != nil {
		return rawgGame{}, nil, err
	}
	game, err := fetchGameByID(ctx, c, match.ID)
	if err != nil {
		return rawgGame{}, nil, err
	}
	return game, candidates, nil
}

// steamLookupName picks the best string for the keyless Steam storefront
// search: the canonical RAWG name when present, else the user's title.
func steamLookupName(title string, game rawgGame) string {
	if game.Name != "" {
		return game.Name
	}
	return title
}

// formatSteamPrice renders an appdetails price block for humans.
func formatSteamPrice(p *steam.PriceOverview) string {
	if p == nil || p.Final <= 0 {
		return ""
	}
	amount := float64(p.Final) / 100
	if strings.EqualFold(p.Currency, "USD") {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("%.2f %s", amount, p.Currency)
}

// yearOf extracts the release year from a RAWG YYYY-MM-DD date.
func yearOf(released string) string {
	if len(released) >= 4 && isYearValue(released[:4]) {
		return released[:4]
	}
	return ""
}

// ----- ratings -----

// ratingSourceRow is one per-source score in the ratings card.
type ratingSourceRow struct {
	Source     string  `json:"source"`
	Score      float64 `json:"score"`
	Scale      string  `json:"scale"`
	SampleSize int     `json:"sample_size"`
	Detail     string  `json:"detail,omitempty"`
}

type ratingsMeta struct {
	Source         string               `json:"source"`
	Title          string               `json:"title,omitempty"`
	Year           string               `json:"year,omitempty"`
	ResolvedBy     string               `json:"resolved_by"`
	Ambiguous      []ambiguousCandidate `json:"ambiguous,omitempty"`
	SourcesMissing []string             `json:"sources_missing,omitempty"`
	// SteamResolvedBy records how the Steam appid was found
	// ("rawg-store-link" or "title") so the card shows its provenance.
	SteamResolvedBy string `json:"steam_resolved_by,omitempty"`
}

type ratingsCard struct {
	Name         string             `json:"name"`
	Released     string             `json:"released"`
	RawgID       int                `json:"rawg_id"`
	Playtime     int                `json:"playtime"`
	EsrbRating   string             `json:"esrb_rating,omitempty"`
	RawgRating   float64            `json:"rawg_rating"`
	RatingsCount int                `json:"ratings_count"`
	Metacritic   *int               `json:"metacritic"`
	Steam        *steam.SteamReview `json:"steam,omitempty"`
	HasDemo      *bool              `json:"has_demo,omitempty"`
	DemoAppIDs   []int64            `json:"demo_app_ids,omitempty"`
	Sources      []ratingSourceRow  `json:"sources"`
}

type ratingsView struct {
	Meta    ratingsMeta   `json:"meta"`
	Results []ratingsCard `json:"results"`
}

func buildRatingsCard(g rawgGame, review *steam.SteamReview) ratingsCard {
	card := ratingsCard{
		Name:         g.Name,
		Released:     g.Released,
		RawgID:       g.ID,
		Playtime:     g.Playtime,
		RawgRating:   g.Rating,
		RatingsCount: g.RatingsCount,
		Metacritic:   g.Metacritic,
		Sources:      make([]ratingSourceRow, 0, 3),
	}
	if g.EsrbRating != nil {
		card.EsrbRating = g.EsrbRating.Name
	}
	card.Sources = append(card.Sources, ratingSourceRow{
		Source:     "rawg",
		Score:      g.Rating,
		Scale:      "0-5",
		SampleSize: g.RatingsCount,
	})
	if g.Metacritic != nil {
		card.Sources = append(card.Sources, ratingSourceRow{
			Source: "metacritic",
			Score:  float64(*g.Metacritic),
			Scale:  "0-100",
		})
	}
	if review != nil {
		card.Steam = review
		card.HasDemo = review.HasDemo
		card.DemoAppIDs = review.DemoAppIDs
		card.Sources = append(card.Sources, ratingSourceRow{
			Source:     "steam",
			Score:      review.Score,
			Scale:      "0-100",
			SampleSize: review.Total,
			Detail:     review.Desc,
		})
	}
	return card
}

func renderRatingsCard(cmd *cobra.Command, card ratingsCard, meta ratingsMeta) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s (%s) — RAWG id %d\n", orDash(card.Name), orDash(card.Released), card.RawgID)
	rows := make([]map[string]any, 0, len(card.Sources))
	for _, s := range card.Sources {
		detail := s.Detail
		if s.Source == "rawg" {
			detail = fmt.Sprintf("community rating, %d ratings", s.SampleSize)
		}
		rows = append(rows, map[string]any{
			"source":      s.Source,
			"score":       strconv.FormatFloat(s.Score, 'f', -1, 64),
			"scale":       s.Scale,
			"sample_size": s.SampleSize,
			"detail":      detail,
		})
	}
	if err := printAutoTable(w, rows); err != nil {
		return err
	}
	fmt.Fprintf(w, "  playtime:     ~%dh to beat\n", card.Playtime)
	if card.EsrbRating != "" {
		fmt.Fprintf(w, "  esrb:         %s\n", card.EsrbRating)
	}
	if card.Steam != nil {
		fmt.Fprintf(w, "  steam price:  %s\n", orDash(formatSteamPrice(card.Steam.Price)))
	}
	if card.HasDemo != nil {
		if !*card.HasDemo {
			fmt.Fprintln(w, "  steam demo:   no")
		} else if len(card.DemoAppIDs) > 0 {
			appids := make([]string, 0, len(card.DemoAppIDs))
			for _, id := range card.DemoAppIDs {
				appids = append(appids, strconv.FormatInt(id, 10))
			}
			fmt.Fprintf(w, "  steam demo:   yes (appid %s)\n", strings.Join(appids, ", "))
		} else {
			fmt.Fprintln(w, "  steam demo:   yes")
		}
	}
	if len(meta.SourcesMissing) > 0 {
		fmt.Fprintf(w, "  missing sources: %s (card degraded to the remaining sources)\n", strings.Join(meta.SourcesMissing, ", "))
	}
	return nil
}

func newRatingsCmd(flags *rootFlags) *cobra.Command {
	var year string

	cmd := &cobra.Command{
		Use:   "ratings <title>",
		Short: "Rating card for one game: RAWG, Metacritic, and keyless Steam reviews",
		Long: `Resolve a game by title and print one rating card with every score this
CLI can see: the RAWG community rating, Metacritic (via the RAWG field), and
a keyless Steam review summary with current price. A failed Steam lookup
degrades the card to RAWG + Metacritic and lists "steam" in sources_missing
instead of failing. Titles shared by remakes across release years are
flagged as ambiguous on stderr and in meta.ambiguous; pin with --year. A
bare-numeric argument is a RAWG id, matching retention and games get.`,
		Example: strings.Trim(`
  game-goat-pp-cli ratings "Elden Ring"
  game-goat-pp-cli ratings "God of War" --year 2018 --json
  game-goat-pp-cli ratings "Yakuza 0" --json --select results.sources
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Elden Ring;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ratings")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--year <yyyy>]", "a game title is required")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2018")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			title := strings.Join(args, " ")
			game, candidates, err := fetchResolvedGameDetail(ctx, cmd, c, flags, title, year)
			if err != nil {
				return err
			}
			resolvedBy := "title"
			if _, ok := parseGameID(title); ok {
				resolvedBy = "id"
			}
			meta := ratingsMeta{Source: "live", Title: title, ResolvedBy: resolvedBy}
			if year != "" {
				meta.Year = year
			}
			if candidates != nil {
				meta.Ambiguous = candidates
			}
			// Keyless Steam enrichment: never fail the whole command for it.
			// The appid comes from RAWG's Steam store link first, because that
			// is exact; the store-aware title search is the fallback.
			steamReview, steamResolvedBy, serr := steamEnrichmentForGame(ctx, c, game, title)
			if serr != nil {
				meta.SourcesMissing = []string{"steam"}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam enrichment unavailable for %q: %v\n", title, serr)
			} else {
				meta.SteamResolvedBy = steamResolvedBy
			}
			card := buildRatingsCard(game, steamReview)
			view := ratingsView{Meta: meta, Results: []ratingsCard{card}}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderRatingsCard(cmd, card, meta)
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin the title to a release year when remakes share the name (e.g. 2018)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newRatingsCmd(flags))
	})
}
