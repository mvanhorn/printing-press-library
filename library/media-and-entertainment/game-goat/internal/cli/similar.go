// similar.go — hand-written novel command (top-level).
// pp:data-source live — resolve one game, then build a tiered recommendation
// list from what RAWG's free tier can honestly say: same-studio games
// (capped, the seed's own DLC/editions excluded), then the seed's defining
// gameplay tag (tag-neighborhood co-occurrence), then a confidence-floored
// shared-genre join. RAWG's /games/{id}/suggested endpoint is business-tier
// only, so this join is by design. Absorbs the former 'suggested' command
// (scope cut 2026-09-27).
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"

	"github.com/spf13/cobra"
)

// ----- pure helpers (unit-tested) -----

// genreIDList extracts the RAWG genre ids of a game for /games?genres= joins.
func genreIDList(g rawgGame) []int {
	ids := make([]int, 0, len(g.Genres))
	for _, ref := range g.Genres {
		ids = append(ids, ref.ID)
	}
	return ids
}

// nonGameplayTags are tag names that describe platform metadata or audio
// options rather than what playing the game feels like. They carry no
// similarity signal.
var nonGameplayTags = map[string]bool{
	"singleplayer": true, "multiplayer": false, // multiplayer IS a signal
	"controller": true, "full controller support": true,
	"family sharing": true, "stats": true,
	"stereo sound": true, "surround sound": true,
	"playable without timed input": true, "camera comfort": true,
	"steam timeline": true,
}

// isGameplayTag reports whether a RAWG tag describes gameplay (mechanics,
// presentation, mood) rather than Steam-platform metadata.
func isGameplayTag(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || nonGameplayTags[n] {
		return false
	}
	if strings.HasPrefix(n, "steam ") || strings.HasPrefix(n, "remote play") {
		return false
	}
	return true
}

// tagRef pairs a gameplay tag with RAWG's games_count for the tag.
type tagRef struct {
	ID    int
	Name  string
	Count int
}

// mechanicsTags names RAWG tags that describe HOW a game plays rather than
// its trappings. When the seed carries one of these, it is the identity to
// match: a roguelite's neighbors are roguelites, not "games with loot".
// Hand-curated: the tags that name a core gameplay loop.
var mechanicsTags = map[string]bool{
	"roguelike": true, "roguelite": true, "bullet hell": true,
	"hack and slash": true, "metroidvania": true, "platformer": true,
	"souls-like": true, "survival": true, "open world": true,
	"sandbox": true, "tower defense": true, "dungeon crawler": true,
	"city builder": true, "farming simulator": true, "farming sim": true,
	"deckbuilder": true, "deck-building": true, "turn-based tactics": true,
	"turn-based strategy": true, "real-time strategy": true, "grand strategy": true,
	"4x": true, "battle royale": true, "looter shooter": true,
	"immersive sim": true, "stealth": true, "shoot 'em up": true, "shmup": true,
	"beat 'em up": true, "run and gun": true, "twin stick": true,
	"colony sim": true, "base building": true, "automation": true,
	"management": true, "trading card game": true, "action rpg": true,
	"jrpg": true, "crpg": true, "mmorpg": true, "psychological horror": true,
	"survival horror": true, "racing": true, "flight": true, "life sim": true,
}

// isMechanicsTag reports whether a gameplay tag names the game's core loop.
func isMechanicsTag(name string) bool {
	return mechanicsTags[strings.ToLower(strings.TrimSpace(name))]
}

// gameplayTagsByRarity returns the seed's gameplay tags ordered rarest
// first. games_count is RAWG's own census: a tag shared by 2,900 games
// (Loot) describes the game far more specifically than one shared by
// 99,660 (Pixel Graphics) — the rarest tags are the defining mechanics,
// and rarity ordering surfaces them without any hand-curated vocabulary.
func gameplayTagsByRarity(g rawgGame) []tagRef {
	tags := make([]tagRef, 0, len(g.Tags))
	for _, ref := range g.Tags {
		if !isGameplayTag(ref.Name) {
			continue
		}
		tags = append(tags, tagRef{ID: ref.ID, Name: ref.Name, Count: ref.GamesCount})
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].Count < tags[j].Count })
	return tags
}

// mechanicsClusterNeighborhood probes the seed's mechanics-vocabulary tags
// (independent -added top-20 per tag) and returns the neighborhood of the
// tag whose results co-occur most with the other tags' neighborhoods — the
// seed's identity cluster (megabonk: roguelike+roguelite+bullet-hell beat a
// lone hack-and-slash trapping). Probe sets are independent because a game
// appearing in several tags' neighborhoods IS the signal; RAWG's comma-
// separated tags= is a union, so single-tag queries are the tightest match
// the API offers. fill is the caller's list size; a tag that fills it
// outranks higher co-occurrence on a smaller set. Callers own dedupe and
// ordering.
func mechanicsClusterNeighborhood(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, seed rawgGame, fill int) ([]rawgGame, tagRef, bool, error) {
	seedTags := gameplayTagsByRarity(seed)
	var mechTags []tagRef
	for _, t := range seedTags {
		if isMechanicsTag(t.Name) {
			mechTags = append(mechTags, t)
		}
	}
	if len(mechTags) > 4 {
		mechTags = mechTags[:4] // rarity-ascending; enough to find the cluster
	}
	if len(mechTags) == 0 {
		return nil, tagRef{}, false, nil
	}
	filterSeed := func(gs []rawgGame) []rawgGame {
		out := make([]rawgGame, 0, len(gs))
		for _, g := range gs {
			if g.ID != 0 && g.ID != seed.ID && !(g.Slug != "" && g.Slug == seed.Slug) {
				out = append(out, g)
			}
		}
		return out
	}
	sets := make([][]rawgGame, len(mechTags))
	idSets := make([]map[int]bool, len(mechTags))
	for i, t := range mechTags {
		gs, _, err := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
			"tags":      strconv.Itoa(t.ID),
			"ordering":  "-added",
			"page_size": "20",
		})
		if err != nil {
			return nil, tagRef{}, false, err
		}
		sets[i] = filterSeed(gs)
		idSets[i] = make(map[int]bool, len(sets[i]))
		for _, g := range sets[i] {
			idSets[i][g.ID] = true
		}
	}
	bestIdx, bestScore := -1, -1
	for i := range mechTags {
		score := 0
		for j := range mechTags {
			if i == j {
				continue
			}
			for _, g := range sets[i] {
				if idSets[j][g.ID] {
					score++
				}
			}
		}
		if len(sets[i]) >= fill && score > bestScore {
			bestIdx, bestScore = i, score
		}
	}
	// No cluster tag filled the list: keep the largest usable set.
	if bestIdx == -1 {
		for i := range mechTags {
			if len(sets[i]) > 0 && (bestIdx == -1 || len(sets[i]) > len(sets[bestIdx])) {
				bestIdx = i
			}
		}
	}
	if bestIdx == -1 || len(sets[bestIdx]) == 0 {
		return nil, tagRef{}, false, nil
	}
	return sets[bestIdx], mechTags[bestIdx], true, nil
}

// scoreSimilarity ranks a candidate against a source game: shared-genre
// count first, then metacritic (nil loses to any score), then RAWG rating.
// Pure: unit-tested.
type similarityScore struct {
	Shared     int
	Metacritic float64
	Rating     float64
}

func scoreSimilarity(candidate rawgGame, sourceGenres map[string]bool) similarityScore {
	shared := 0
	for _, ref := range candidate.Genres {
		if sourceGenres[strings.ToLower(ref.Name)] {
			shared++
		}
	}
	meta := 0.0
	if candidate.Metacritic != nil {
		meta = float64(*candidate.Metacritic)
	}
	return similarityScore{Shared: shared, Metacritic: meta, Rating: candidate.Rating}
}

func lessSimilarity(a, b similarityScore) bool {
	if a.Shared != b.Shared {
		return a.Shared > b.Shared
	}
	if a.Metacritic != b.Metacritic {
		return a.Metacritic > b.Metacritic
	}
	return a.Rating > b.Rating
}

// minConfidentRatings is the smallest RAWG ratings_count treated as a
// confident community signal. Below it a 4.7 is a handful of fans, not a
// recommendation (UAT F-U13: megabonk's genre join led with 4.7s backed by
// 6-7 ratings).
const minConfidentRatings = 20

// idCSV renders ids as a RAWG comma-separated param value.
func idCSV(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

// namedRefIDs extracts ids from a named-ref slice (developers, publishers).
func namedRefIDs(refs []rawgNamedRef) []int {
	ids := make([]int, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	return ids
}

// broadGenres are RAWG genres carried by a majority of games; matching on
// one of them alone is not a similarity signal (UAT F-U16: guildrun).
var broadGenres = map[string]bool{
	"indie": true, "action": true, "adventure": true,
}

// sharesOnlyBroadGenre reports whether g's only shared genre with the seed
// is a broad one (Indie, Action, Adventure). Such a row matches millions of
// games and carries nothing seed-specific.
func sharesOnlyBroadGenre(g rawgGame, seedGenres map[string]bool) bool {
	shared := 0
	sharedName := ""
	for _, ref := range g.Genres {
		if seedGenres[strings.ToLower(ref.Name)] {
			shared++
			sharedName = strings.ToLower(ref.Name)
		}
	}
	return shared == 1 && broadGenres[sharedName]
}

// isBundleOrSoundtrack reports whether a studio-catalog row is packaging
// (a soundtrack or a bundle) rather than a distinct game to recommend
// (UAT: "Hollow Knight: Silksong & Soundtrack Bundle").
func isBundleOrSoundtrack(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "soundtrack") || strings.Contains(n, "bundle")
}

// confidentOnly keeps rows with a real rating sample. A shorter confident
// list beats one padded with tiny-sample outliers; when nothing clears the
// floor the input is returned unchanged.
func confidentOnly(gs []rawgGame) []rawgGame {
	out := make([]rawgGame, 0, len(gs))
	for _, g := range gs {
		if g.RatingsCount >= minConfidentRatings {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		return gs
	}
	return out
}

// rankByOverlap orders candidates by shared-genre count, then metacritic,
// then rating-sample size, then rating.
func rankByOverlap(gs []rawgGame, seedGenres map[string]bool) {
	sort.SliceStable(gs, func(i, j int) bool {
		si, sj := scoreSimilarity(gs[i], seedGenres), scoreSimilarity(gs[j], seedGenres)
		if si.Shared != sj.Shared {
			return si.Shared > sj.Shared
		}
		if si.Metacritic != sj.Metacritic {
			return si.Metacritic > sj.Metacritic
		}
		if gs[i].RatingsCount != gs[j].RatingsCount {
			return gs[i].RatingsCount > gs[j].RatingsCount
		}
		return si.Rating > sj.Rating
	})
}

// studioLabel names the seed's credited developers for the studio-tier
// reason. RAWG credits porters and publishers too, and the tier queries all
// of them, so the label lists the first two plus a count rather than
// implying a single studio.
func studioLabel(devs []rawgNamedRef) string {
	names := refNames(devs)
	if len(names) <= 2 {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(names[:2], ", "), len(names)-2)
}

// studioCap bounds the same-studio tier so a prolific studio cannot fill
// the whole list with its own catalog (UAT: witcher 3 returned only CDPR).
func studioCap(limit int) int {
	return (limit + 1) / 2
}

// sharedGenreReason renders the genre-tier reason string.
func sharedGenreReason(g rawgGame, seed rawgGame, seedGenres map[string]bool) string {
	shared := make([]string, 0, len(g.Genres))
	for _, ref := range g.Genres {
		if seedGenres[strings.ToLower(ref.Name)] {
			shared = append(shared, ref.Name)
		}
	}
	if len(shared) == 0 {
		return ""
	}
	noun := "genres"
	if len(shared) == 1 {
		noun = "genre"
	}
	return fmt.Sprintf("shares %d %s with %s (%s)", len(shared), noun, seed.Name, strings.Join(shared, ", "))
}

// seedAdditionIDs lists the seed's own DLC and editions
// (/games/{id}/additions) so the studio tier does not recommend a game's
// expansions as "similar" to it.
func seedAdditionIDs(ctx context.Context, c *client.Client, seedID int) (map[int]bool, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/games/%d/additions", seedID), map[string]string{"page_size": "40"})
	if err != nil {
		return nil, err
	}
	var page struct {
		Results []rawgGame `json:"results"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing /games/%d/additions response: %w", seedID, err)
	}
	ids := make(map[int]bool, len(page.Results))
	for _, g := range page.Results {
		ids[g.ID] = true
	}
	return ids, nil
}

// ----- view -----

type similarResult struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Released     string   `json:"released,omitempty"`
	Rating       float64  `json:"rating"`
	RatingsCount int      `json:"ratings_count"`
	Metacritic   *int     `json:"metacritic,omitempty"`
	Genres       []string `json:"genres"`
	Tier         string   `json:"tier"` // studio | mechanics | genre
	Reason       string   `json:"reason,omitempty"`
	// SteamAppID/HasDemo/DemoAppIDs are filled only under --with-demos.
	SteamAppID int64   `json:"steam_app_id,omitempty"`
	HasDemo    *bool   `json:"has_demo,omitempty"`
	DemoAppIDs []int64 `json:"demo_app_ids,omitempty"`
}

type similarMeta struct {
	Source     string               `json:"source"`
	DataOrigin string               `json:"data_origin"` // tiered-join
	Tiers      []string             `json:"tiers"`
	Seed       string               `json:"seed"`
	SeedID     int                  `json:"seed_id"`
	Note       string               `json:"note,omitempty"`
	Ambiguous  []ambiguousCandidate `json:"ambiguous,omitempty"`
	Count      int                  `json:"count"`
	Limit      int                  `json:"limit"`
}

type similarView struct {
	Meta    similarMeta     `json:"meta"`
	Results []similarResult `json:"results"`
}

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var year string
	var limit int
	var withDemos bool

	cmd := &cobra.Command{
		Use:   "similar <title>",
		Short: "Games like <title>: same studio, defining gameplay tag, then shared genres",
		Long: `Find games like one you name. The seed title is resolved against RAWG
(remake collisions are flagged ambiguous; pin with --year; a bare-numeric
argument is a RAWG id, matching retention and games get), then results are
filled in tiers:

  studio     other games by the seed's developer (at most half the list;
             the seed's own DLC, editions, soundtracks, and bundles excluded)
  mechanics  games carrying the seed's defining gameplay tag (roguelite,
             metroidvania...), picked by tag-neighborhood co-occurrence
  genre      a shared-genre join, only when the earlier tiers run short

Every tier drops tiny rating samples (under 20 ratings) when confident rows
exist. For seeds RAWG knows little about (no gameplay tags, no studio), rows
sharing only a broad genre (Indie, Action, Adventure) are hidden and
meta.note says so. Each row carries its tier and a reason; meta.tiers lists
the tiers that contributed.

RAWG's own /games/{id}/suggested endpoint is business-tier only, so this
join over free-tier endpoints is by design.`,
		Example: strings.Trim(`
  game-goat-pp-cli similar "Hollow Knight"
  game-goat-pp-cli similar "God of War" --year 2018
  game-goat-pp-cli similar "Megabonk" --limit 5 --json --select results.name,results.tier,results.reason
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Hollow Knight",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "similar")
			}
			if len(args) != 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--year <yyyy>]", "similar takes exactly one quoted game title, e.g. similar \"Hollow Knight\"")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2018")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > 20 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --limit <1-20>", "--limit must be between 1 and 20")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			seed, candidates, err := fetchResolvedGameDetail(ctx, cmd, c, flags, args[0], year)
			if err != nil {
				return err
			}
			seedGenres := map[string]bool{}
			for _, ref := range seed.Genres {
				seedGenres[strings.ToLower(ref.Name)] = true
			}

			results := make([]similarResult, 0, limit)
			seen := map[int]bool{seed.ID: true}
			add := func(g rawgGame, tier, reason string) bool {
				if len(results) >= limit || g.ID == 0 || seen[g.ID] || (g.Slug != "" && g.Slug == seed.Slug) {
					return false
				}
				seen[g.ID] = true
				results = append(results, similarResult{
					ID: g.ID, Name: g.Name, Released: g.Released, Rating: g.Rating,
					RatingsCount: g.RatingsCount, Metacritic: g.Metacritic,
					Genres: refNames(g.Genres), Tier: tier, Reason: reason,
				})
				return true
			}
			tiers := make([]string, 0, 3)
			notes := []string{"RAWG's suggested endpoint is business-tier only; results are a tiered join over free-tier data"}

			// Tier 1: same studio, capped, seed's own additions excluded.
			if devIDs := namedRefIDs(seed.Developers); len(devIDs) > 0 {
				excluded, aerr := seedAdditionIDs(ctx, c, seed.ID)
				if aerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read %q's DLC/editions; studio tier may include them: %v\n", seed.Name, aerr)
				}
				gs, _, derr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
					"developers": idCSV(devIDs),
					"ordering":   "-rating",
					"page_size":  strconv.Itoa(limit * 2),
				})
				if derr != nil {
					return derr
				}
				kept := make([]rawgGame, 0, len(gs))
				for _, g := range gs {
					if excluded[g.ID] || isBundleOrSoundtrack(g.Name) {
						continue
					}
					kept = append(kept, g)
				}
				reason := fmt.Sprintf("same studio as %s (%s)", seed.Name, studioLabel(seed.Developers))
				added := 0
				for _, g := range confidentOnly(kept) {
					if added >= studioCap(limit) {
						break
					}
					if add(g, "studio", reason) {
						added++
					}
				}
				if added > 0 {
					tiers = append(tiers, "studio")
				}
			}

			// Tier 2: the seed's defining gameplay tag.
			if len(results) < limit {
				nb, tag, ok, nerr := mechanicsClusterNeighborhood(ctx, cmd, c, flags, seed, limit)
				if nerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: mechanics-tag tier failed: %v\n", nerr)
				} else if ok {
					nb = confidentOnly(nb)
					rankByOverlap(nb, seedGenres)
					tagName := strings.ToLower(tag.Name)
					reason := fmt.Sprintf("plays like %s (%s)", seed.Name, tagName)
					added := 0
					for _, g := range nb {
						if add(g, "mechanics", reason) {
							added++
						}
					}
					if added > 0 {
						tiers = append(tiers, "mechanics")
						notes = append(notes, fmt.Sprintf("defining gameplay tag: %s", tagName))
					}
				}
			}

			// Tier 3: shared-genre join, only when the list is still short.
			// A thin seed (no gameplay tags, no studio) can only reach this
			// tier; hide rows sharing only a broad genre - the popularity
			// canon, not seed-specific matches (UAT F-U15/F-U16).
			thin := len(gameplayTagsByRarity(seed)) == 0 && len(namedRefIDs(seed.Developers)) == 0
			if genreIDs := genreIDList(seed); len(results) < limit && len(genreIDs) > 0 {
				gs, _, gerr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
					"genres":    idCSV(genreIDs),
					"ordering":  "-rating",
					"page_size": strconv.Itoa(limit * 3),
				})
				if gerr != nil {
					return gerr
				}
				gs = confidentOnly(gs)
				rankByOverlap(gs, seedGenres)
				added, hidden := 0, 0
				for _, g := range gs {
					if len(results) >= limit {
						break
					}
					if thin && sharesOnlyBroadGenre(g, seedGenres) {
						hidden++
						continue
					}
					if add(g, "genre", sharedGenreReason(g, seed, seedGenres)) {
						added++
					}
				}
				if added > 0 {
					tiers = append(tiers, "genre")
				}
				if thin {
					notes = append(notes, fmt.Sprintf("RAWG has minimal data on this game (no gameplay tags, no studio); %d weak single-broad-genre matches hidden", hidden))
					fmt.Fprintf(cmd.ErrOrStderr(), "note: RAWG has minimal data on %q (no gameplay tags, no studio) - weak single-broad-genre matches are hidden; anything shown shares a specific genre\n", seed.Name)
				}
			}
			if len(results) == 0 {
				notes = append(notes, "no tier produced a match")
			}
			if withDemos && len(results) > 0 {
				failed, aerr := annotateSimilarSteamDemos(cmd, c, results)
				if aerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam demo annotation unavailable: %v\n", aerr)
				} else if failed > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam demo annotation: %d of %d Steam store-link lookups failed; demo state unknown for those rows\n", failed, len(results))
				}
			}

			view := similarView{
				Meta: similarMeta{
					Source: "live", DataOrigin: "tiered-join", Tiers: tiers,
					Seed: seed.Name, SeedID: seed.ID, Note: strings.Join(notes, "; "),
					Ambiguous: candidates, Count: len(results), Limit: limit,
				},
				Results: results,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(w, "No similar games found for %q. Browse by genre: game-goat-pp-cli discover\n", seed.Name)
				return nil
			}
			tw := newTabWriter(w)
			header := []string{bold("SIMILAR"), bold("TIER"), bold("RATING")}
			if withDemos {
				header = append(header, bold("DEMO"))
			}
			header = append(header, bold("GENRES"), bold("REASON"))
			fmt.Fprintln(tw, strings.Join(header, "\t"))
			for _, r := range results {
				rating := "-"
				if r.Rating > 0 {
					rating = fmt.Sprintf("%.1f (%d)", r.Rating, r.RatingsCount)
				}
				cols := []string{r.Name, r.Tier, rating}
				if withDemos {
					cols = append(cols, steamDemoCell(r.HasDemo))
				}
				cols = append(cols, truncateList(r.Genres, 3), r.Reason)
				fmt.Fprintln(tw, strings.Join(cols, "\t"))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin the title to a release year when remakes share a name (e.g. 2018)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum similar games to return (1-20)")
	cmd.Flags().BoolVar(&withDemos, "with-demos", false, withDemosHelp)
	return cmd
}

// newNovelSimilarCmd is the constructor the generated root registers for
// the 'similar' novel feature.
func newNovelSimilarCmd(flags *rootFlags) *cobra.Command {
	return newSimilarCmd(flags)
}
