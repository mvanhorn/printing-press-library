// games_search.go — hand-written Slice A novel command for games search.
// pp:data-source auto — live RAWG search by default; --data-source local
// reads the synced SQLite mirror once 'game-goat-pp-cli sync' has run.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"

	"github.com/spf13/cobra"
)

// ----- shared RAWG /games decoding and bounded view rows (Slice A) -----

// rawgNamedRef is a RAWG {id,name,slug} reference object (genre, tag, store...).
type rawgNamedRef struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	GamesCount int    `json:"games_count,omitempty"` // present on /games/{id} tags; 0 on genre refs
}

// rawgPlatformEntry is one entry of the /games platforms array.
type rawgPlatformEntry struct {
	Platform rawgNamedRef `json:"platform"`
}

// rawgStoreEntry is one entry of the /games stores array.
type rawgStoreEntry struct {
	Store rawgNamedRef `json:"store"`
}

// rawgGame is the subset of a RAWG /games object Slice A surfaces. List
// responses populate the core fields; detail responses add the rest.
type rawgGame struct {
	ID              int                 `json:"id"`
	Slug            string              `json:"slug"`
	Name            string              `json:"name"`
	Released        string              `json:"released"`
	Rating          float64             `json:"rating"`
	RatingsCount    int                 `json:"ratings_count"`
	Added           int                 `json:"added"`
	Metacritic      *int                `json:"metacritic"`
	Playtime        int                 `json:"playtime"`
	Genres          []rawgNamedRef      `json:"genres"`
	Tags            []rawgNamedRef      `json:"tags"`
	Platforms       []rawgPlatformEntry `json:"platforms"`
	Stores          []rawgStoreEntry    `json:"stores"`
	Developers      []rawgNamedRef      `json:"developers"`
	Publishers      []rawgNamedRef      `json:"publishers"`
	EsrbRating      *rawgNamedRef       `json:"esrb_rating"`
	Website         string              `json:"website"`
	BackgroundImage string              `json:"background_image"`
}

// gameRow is the bounded per-game output row shared by the Slice A views.
type gameRow struct {
	ID           int      `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Released     string   `json:"released"`
	Rating       float64  `json:"rating"`
	RatingsCount int      `json:"ratings_count"`
	Added        int      `json:"added"`
	Metacritic   *int     `json:"metacritic"`
	Playtime     int      `json:"playtime"`
	Genres       []string `json:"genres"`
	Platforms    []string `json:"platforms"`
	// SteamAppID/HasDemo/DemoAppIDs are filled only under discover --with-demos.
	SteamAppID int64   `json:"steam_app_id,omitempty"`
	HasDemo    *bool   `json:"has_demo,omitempty"`
	DemoAppIDs []int64 `json:"demo_app_ids,omitempty"`
}

func refNames(refs []rawgNamedRef) []string {
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Name != "" {
			names = append(names, r.Name)
		}
	}
	return names
}

func toGameRow(g rawgGame) gameRow {
	platforms := make([]string, 0, len(g.Platforms))
	for _, p := range g.Platforms {
		if p.Platform.Name != "" {
			platforms = append(platforms, p.Platform.Name)
		}
	}
	return gameRow{
		ID:           g.ID,
		Slug:         g.Slug,
		Name:         g.Name,
		Released:     g.Released,
		Rating:       g.Rating,
		RatingsCount: g.RatingsCount,
		Added:        g.Added,
		Metacritic:   g.Metacritic,
		Playtime:     g.Playtime,
		Genres:       refNames(g.Genres),
		Platforms:    platforms,
	}
}

// normalizeGameTitle folds case, whitespace, and edge punctuation so remake
// disambiguation can group "God of War" (2005) with "GOD OF WAR" (2018).
// A trailing "(YYYY)" parenthetical — RAWG's convention for same-named
// releases ("DOOM" vs "DOOM (2016)") — is stripped so the shared base
// title compares equal. This key is a comparison key only; display names
// are never modified.
func normalizeGameTitle(s string) string {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	s = strings.Trim(s, " \t:;,.-–—")
	if i := strings.LastIndex(s, "("); i >= 0 && strings.HasSuffix(s, ")") {
		if inner := strings.TrimSpace(s[i+1 : len(s)-1]); isYearValue(inner) {
			s = strings.TrimSpace(s[:i])
		}
	}
	return s
}

// ambiguousCandidate is one same-named game in a remake-aware ambiguity group.
type ambiguousCandidate struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Released string `json:"released,omitempty"`
}

// fetchGamesResults runs the generated live/local resolver against GET /games
// and decodes the unwrapped results array. Shared by every Slice A command;
// strategy is "auto" (live first, local fallback) or "live".
func fetchGamesResults(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, strategy string, params map[string]string) ([]rawgGame, string, error) {
	data, prov, err := resolvePaginatedReadWithStrategy(ctx, c, flags, strategy, "games", "/games", params, nil, false, "page", "page", "page_size", 0, "", "", "results", cmd.ErrOrStderr())
	if err != nil {
		return nil, "", classifyAPIErrorOnly(err)
	}
	games, err := decodeRawgGames(data)
	if err != nil {
		return nil, prov.Source, err
	}
	return games, prov.Source, nil
}

// decodeRawgGames accepts the bare results array the resolver unwraps, and
// defensively also a full {count,results} envelope.
func decodeRawgGames(data json.RawMessage) ([]rawgGame, error) {
	var games []rawgGame
	if err := json.Unmarshal(data, &games); err == nil {
		return games, nil
	}
	var envelope struct {
		Results []rawgGame `json:"results"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil {
		return envelope.Results, nil
	}
	return nil, fmt.Errorf("parsing RAWG /games response: unexpected JSON shape")
}

// fetchGameByID fetches one live game detail record by RAWG id.
func fetchGameByID(ctx context.Context, c *client.Client, id int) (rawgGame, error) {
	data, err := c.Get(ctx, "/games/"+strconv.Itoa(id), nil)
	if err != nil {
		return rawgGame{}, classifyAPIErrorOnly(err)
	}
	var g rawgGame
	if err := json.Unmarshal(data, &g); err != nil {
		return rawgGame{}, fmt.Errorf("parsing RAWG game detail for id %d: %w", id, err)
	}
	if g.ID == 0 && g.Name == "" {
		return rawgGame{}, notFoundErr(fmt.Errorf("no game with RAWG id %d; find ids with 'game-goat-pp-cli games search'", id))
	}
	return g, nil
}

// resolveExactTitleMatches fetches the top RAWG search results for title,
// ranks an exact-title match first, filters to normalized exact matches
// (plus any --year pin), and returns them alongside the full ranked
// search list (for top-hit fallback) and nearby titles for error hints.
// The caller decides ambiguity policy.
func resolveExactTitleMatches(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, title, year string) ([]rawgGame, []rawgGame, []string, error) {
	games, _, err := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
		"search":    title,
		"page_size": "5",
	})
	if err != nil {
		return nil, nil, nil, err
	}
	games = rankExactTitleFirst(title, games)
	normalized := normalizeGameTitle(title)
	exact := make([]rawgGame, 0, len(games))
	for _, g := range games {
		if normalizeGameTitle(g.Name) == normalized {
			exact = append(exact, g)
		}
	}
	if year != "" {
		exact = filterByReleaseYear(exact, year)
	}
	nearby := make([]string, 0, len(games))
	for _, g := range games {
		if g.Name != "" {
			nearby = append(nearby, g.Name)
		}
	}
	return exact, games, nearby, nil
}

// obscureExactAddedFloor: an exact-title search hit below this RAWG
// library-add count has too little community data to be confidently what
// the user meant (UAT F-U14: "halo" exact-matched an itch RPG with
// added=3 while Halo Infinite sat at 7888 further down the results).
const obscureExactAddedFloor = 100

// franchiseDominanceMultiple / franchiseMinAdded: a franchise continuation
// must beat the obscure exact hit by this multiple AND clear this floor to
// take over resolution — both must hold, so a 433-vs-10 win (Zelda II over
// the junk "Zelda") qualifies but near-ties never do.
const franchiseDominanceMultiple = 10
const franchiseMinAdded = 200

// bestKnownGame returns the hit with the strongest RAWG community data
// (highest added count). RAWG's search ordering is relevance-shaped, not
// popularity-shaped — its first hit can be a 3-add obscure title.
func bestKnownGame(games []rawgGame) rawgGame {
	if len(games) == 0 {
		return rawgGame{}
	}
	best := games[0]
	for _, g := range games[1:] {
		if g.Added > best.Added {
			best = g
		}
	}
	return best
}

// isFranchiseContinuation reports whether name continues query at a word
// boundary ("halo" -> "halo infinite", "halo: reach"); "halo" -> "halo's
// adventure" does not (apostrophe is not a boundary).
func isFranchiseContinuation(query, name string) bool {
	q := normalizeGameTitle(query)
	n := normalizeGameTitle(name)
	if len(n) <= len(q) || !strings.HasPrefix(n, q) {
		return false
	}
	rest := n[len(q):]
	return strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, ":")
}

// franchiseOverride reports the franchise continuation that should take
// resolution over an obscure exact-title hit: it must dominate the exact
// hit's community data. Returns ok=false when the exact hit is confident,
// no continuation exists, or the best continuation is not dominant.
func franchiseOverride(exact rawgGame, ranked []rawgGame, query string) (rawgGame, bool) {
	if exact.Added >= obscureExactAddedFloor {
		return rawgGame{}, false
	}
	best := rawgGame{}
	for _, g := range ranked {
		if g.ID == exact.ID || !isFranchiseContinuation(query, g.Name) {
			continue
		}
		if g.Added > best.Added {
			best = g
		}
	}
	if best.ID == 0 {
		return rawgGame{}, false
	}
	if best.Added < exact.Added*franchiseDominanceMultiple || best.Added < franchiseMinAdded {
		return rawgGame{}, false
	}
	return best, true
}

// nearbyMatchHint builds the no-exact-match error tail: nearby titles plus
// the caller-specific action hint.
func nearbyMatchHint(nearby []string, tail string) string {
	if len(nearby) == 0 {
		return tail
	}
	return fmt.Sprintf("nearby matches: %s; %s", strings.Join(nearby, ", "), tail)
}

// rankExactTitleFirst moves exact normalized-title matches ahead of RAWG's
// fuzzy relevance order, preserving order within each partition.
func rankExactTitleFirst(query string, games []rawgGame) []rawgGame {
	q := normalizeGameTitle(query)
	if q == "" {
		return games
	}
	exact := make([]rawgGame, 0, len(games))
	rest := make([]rawgGame, 0, len(games))
	for _, g := range games {
		if normalizeGameTitle(g.Name) == q {
			exact = append(exact, g)
		} else {
			rest = append(rest, g)
		}
	}
	return append(exact, rest...)
}

// filterByReleaseYear keeps games released in the given year (remake pin).
func filterByReleaseYear(games []rawgGame, year string) []rawgGame {
	prefix := year + "-"
	out := make([]rawgGame, 0, len(games))
	for _, g := range games {
		if strings.HasPrefix(g.Released, prefix) {
			out = append(out, g)
		}
	}
	return out
}

// ambiguousCandidates returns every game whose normalized title is shared by
// two or more results — remakes reusing a name across release years.
func ambiguousCandidates(games []rawgGame) []ambiguousCandidate {
	byTitle := make(map[string][]rawgGame, len(games))
	order := make([]string, 0)
	for _, g := range games {
		key := normalizeGameTitle(g.Name)
		if _, seen := byTitle[key]; !seen {
			order = append(order, key)
		}
		byTitle[key] = append(byTitle[key], g)
	}
	out := make([]ambiguousCandidate, 0)
	for _, key := range order {
		if len(byTitle[key]) < 2 {
			continue
		}
		for _, g := range byTitle[key] {
			out = append(out, ambiguousCandidate{ID: g.ID, Name: g.Name, Released: g.Released})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// writeAmbiguousNotice lists same-named candidates on stderr so humans (and
// agents watching stderr) can pin --year or fetch by id.
func writeAmbiguousNotice(w io.Writer, candidates []ambiguousCandidate) {
	fmt.Fprintf(w, "ambiguous: %d results share a name across release years (remakes); pin with --year or fetch by id:\n", len(candidates))
	for _, c := range candidates {
		released := c.Released
		if released == "" {
			released = "unknown year"
		}
		fmt.Fprintf(w, "  %d  %s (%s)\n", c.ID, c.Name, released)
	}
}

func isYearValue(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// usageErrWithJSON emits a machine-readable error envelope under --json, then
// returns the exit-2 usage error (mirrors the generated commands' pattern).
func usageErrWithJSON(cmd *cobra.Command, flags *rootFlags, usage, msg string) error {
	if flags.asJSON {
		if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"error": msg,
			"usage": usage,
		}, flags); printErr != nil {
			return printErr
		}
	}
	return usageErr(fmt.Errorf("%s\nUsage: %s", msg, usage))
}

// truncateList joins at most max values, noting how many were cut.
func truncateList(values []string, max int) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) <= max {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:max], ", ") + fmt.Sprintf(" +%d", len(values)-max)
}

// gameTableRows renders bounded cells for the generated human table.
func gameTableRows(rows []gameRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		metacritic := ""
		if r.Metacritic != nil {
			metacritic = strconv.Itoa(*r.Metacritic)
		}
		rating := ""
		if r.Rating > 0 {
			rating = fmt.Sprintf("%.1f", r.Rating)
		}
		playtime := ""
		if r.Playtime > 0 {
			playtime = strconv.Itoa(r.Playtime) + "h"
		}
		item := map[string]any{
			"id":         r.ID,
			"name":       r.Name,
			"released":   r.Released,
			"rating":     rating,
			"metacritic": metacritic,
			"playtime":   playtime,
			"genres":     truncateList(r.Genres, 3),
			"platforms":  truncateList(r.Platforms, 3),
		}
		items = append(items, item)
	}
	return items
}

// gamesListMeta/gamesListView are the shared {meta,results} envelope for the
// thin browse commands (popular, top-rated, upcoming).
type gamesListMeta struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type gamesListView struct {
	Meta    gamesListMeta `json:"meta"`
	Results []gameRow     `json:"results"`
}

// renderGamesListView prints the shared browse-command envelope in the active
// output mode. Empty results stay [] in machine mode.
func renderGamesListView(cmd *cobra.Command, flags *rootFlags, games []rawgGame, source string) error {
	rows := make([]gameRow, 0, len(games))
	for _, g := range games {
		rows = append(rows, toGameRow(g))
	}
	if source == "" {
		source = "live"
	}
	view := gamesListView{
		Meta:    gamesListMeta{Source: source, Count: len(rows)},
		Results: rows,
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No games found.")
		return nil
	}
	return printAutoTable(cmd.OutOrStdout(), gameTableRows(rows))
}

// ----- games search -----

type gamesSearchMeta struct {
	Source     string               `json:"source"`
	Query      string               `json:"query,omitempty"`
	Year       string               `json:"year,omitempty"`
	ResolvedBy string               `json:"resolved_by,omitempty"`
	Count      int                  `json:"count"`
	Ambiguous  []ambiguousCandidate `json:"ambiguous,omitempty"`
}

type gamesSearchView struct {
	Meta    gamesSearchMeta `json:"meta"`
	Results []gameRow       `json:"results"`
}

func newGamesSearchCmd(flags *rootFlags) *cobra.Command {
	var year string
	var limit int
	var searchID string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search games by title or keyword",
		Long: `Search the RAWG database by title or keyword. Exact-title matches rank
first; games that reuse a name across release years (remakes such as God of
War 2005 vs 2018) are flagged as ambiguous on stderr and in meta.ambiguous.
Pin a remake with --year, or fetch one game directly with --id.`,
		Example: strings.Trim(`
  game-goat-pp-cli games search "Hollow Knight"
  game-goat-pp-cli games search "Yakuza Kiwami" --year 2016 --json
  game-goat-pp-cli games search --id 23445
  game-goat-pp-cli games search "Elden Ring" --limit 5 --json --select results.name
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "query=Hollow Knight",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "games search")
			}
			if len(args) == 0 && searchID == "" {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <query>", "a search query or --id is required")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" [query] --year <yyyy>", "--year must be a 4-digit release year, e.g. 2016")
			}
			view := gamesSearchView{Results: make([]gameRow, 0)}
			if searchID != "" {
				id, ok := parseGameID(strings.TrimSpace(searchID))
				if !ok {
					return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" [query] --id <rawg-id>", "--id must be a numeric RAWG game id (find one with 'games search')")
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				game, err := fetchGameByID(cmd.Context(), c, id)
				if err != nil {
					return err
				}
				view.Meta = gamesSearchMeta{Source: "live", ResolvedBy: "id", Count: 1}
				view.Results = append(view.Results, toGameRow(game))
			} else {
				query := strings.Join(args, " ")
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				games, source, err := fetchGamesResults(cmd.Context(), cmd, c, flags, "auto", map[string]string{
					"search":    query,
					"page_size": strconv.Itoa(limit),
				})
				if err != nil {
					return err
				}
				games = rankExactTitleFirst(query, games)
				if year != "" {
					games = filterByReleaseYear(games, year)
				}
				candidates := ambiguousCandidates(games)
				if source == "" {
					source = "live"
				}
				meta := gamesSearchMeta{Source: source, Query: query, Count: len(games)}
				if year != "" {
					meta.Year = year
				}
				if candidates != nil {
					meta.Ambiguous = candidates
					writeAmbiguousNotice(cmd.ErrOrStderr(), candidates)
				}
				rows := make([]gameRow, 0, len(games))
				for _, g := range games {
					rows = append(rows, toGameRow(g))
				}
				view.Meta = meta
				view.Results = rows
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No games matched the search.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), gameTableRows(view.Results))
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin results to a release year (remake disambiguation, e.g. 2016)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of games to return (RAWG caps pages at 40)")
	cmd.Flags().StringVar(&searchID, "id", "", "Look up one game by RAWG id instead of a text query")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		gamesCmd, _, err := root.Find([]string{"games"})
		if err == nil {
			addNovelCommandIfAbsent(gamesCmd, newGamesSearchCmd(flags))
		}
	})
}
