package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/client"
)

// tmdbSearchResult represents a single result from TMDb search endpoints.
type tmdbSearchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Overview      string  `json:"overview"`
	MediaType     string  `json:"media_type"`
	Popularity    float64 `json:"popularity"`
	PosterPath    string  `json:"poster_path"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	GenreIDs      []int   `json:"genre_ids"`
	// KnownFor is only populated by /search/person; it discriminates between
	// people who share a name.
	KnownFor string `json:"known_for_department"`
}

// DisplayTitle returns the appropriate title for either movies or TV shows.
func (r *tmdbSearchResult) DisplayTitle() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

// Year returns the year from the release date or first air date.
func (r *tmdbSearchResult) Year() string {
	d := r.ReleaseDate
	if d == "" {
		d = r.FirstAirDate
	}
	if len(d) >= 4 {
		return d[:4]
	}
	return ""
}

// tmdbSearchResponse represents the envelope from TMDb search/discover endpoints.
type tmdbSearchResponse struct {
	Page         int                `json:"page"`
	Results      []tmdbSearchResult `json:"results"`
	TotalPages   int                `json:"total_pages"`
	TotalResults int                `json:"total_results"`
}

// tmdbMovieDetail represents a detailed movie response from TMDb /movie/{id}.
type tmdbMovieDetail struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	ReleaseDate string  `json:"release_date"`
	Runtime     int     `json:"runtime"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	Budget      int64   `json:"budget"`
	Revenue     int64   `json:"revenue"`
	Popularity  float64 `json:"popularity"`
	Tagline     string  `json:"tagline"`
	Status      string  `json:"status"`
	ImdbID      string  `json:"imdb_id"`
	PosterPath  string  `json:"poster_path"`
	Genres      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCompanies []struct {
		Name string `json:"name"`
	} `json:"production_companies"`
	BelongsToCollection *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"belongs_to_collection"`
	ExternalIDs     *tmdbExternalIDs    `json:"external_ids"`
	Credits         *tmdbCredits        `json:"credits"`
	WatchProviders  json.RawMessage     `json:"watch/providers"`
	Videos          json.RawMessage     `json:"videos"`
	Recommendations *tmdbSearchResponse `json:"recommendations"`
}

// tmdbExternalIDs is the append_to_response=external_ids payload for movies and TV.
type tmdbExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	TVDbID      int    `json:"tvdb_id"`
	WikidataID  string `json:"wikidata_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

// tmdbTVDetail represents a detailed TV show response from TMDb /tv/{id}.
type tmdbTVDetail struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	Overview         string  `json:"overview"`
	FirstAirDate     string  `json:"first_air_date"`
	LastAirDate      string  `json:"last_air_date"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Popularity       float64 `json:"popularity"`
	Tagline          string  `json:"tagline"`
	Status           string  `json:"status"`
	Type             string  `json:"type"`
	NumberOfSeasons  int     `json:"number_of_seasons"`
	NumberOfEpisodes int     `json:"number_of_episodes"`
	EpisodeRunTime   []int   `json:"episode_run_time"`
	PosterPath       string  `json:"poster_path"`
	Genres           []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ExternalIDs    *tmdbExternalIDs `json:"external_ids"`
	Credits        *tmdbCredits     `json:"credits"`
	WatchProviders json.RawMessage  `json:"watch/providers"`
	// IMDbID is set by getTVDetail when external_ids is appended; it mirrors
	// ExternalIDs.IMDbID for callers that don't want to dereference.
	IMDbID string `json:"-"`
}

// tmdbCredits represents the credits response.
type tmdbCredits struct {
	Cast []tmdbCastMember `json:"cast"`
	Crew []tmdbCrewMember `json:"crew"`
}

// tmdbCastMember represents a single cast member.
type tmdbCastMember struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Character   string  `json:"character"`
	Order       int     `json:"order"`
	Popularity  float64 `json:"popularity"`
	ProfilePath string  `json:"profile_path"`
}

// tmdbCrewMember represents a single crew member.
type tmdbCrewMember struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Job         string  `json:"job"`
	Department  string  `json:"department"`
	Popularity  float64 `json:"popularity"`
	ProfilePath string  `json:"profile_path"`
}

// tmdbPersonDetail represents a detailed person response.
type tmdbPersonDetail struct {
	ID              int                  `json:"id"`
	Name            string               `json:"name"`
	Biography       string               `json:"biography"`
	Birthday        string               `json:"birthday"`
	Deathday        string               `json:"deathday"`
	PlaceOfBirth    string               `json:"place_of_birth"`
	ProfilePath     string               `json:"profile_path"`
	KnownFor        string               `json:"known_for_department"`
	Popularity      float64              `json:"popularity"`
	CombinedCredits *tmdbCombinedCredits `json:"combined_credits"`
}

// tmdbCombinedCredits contains cast and crew credits across movies and TV.
type tmdbCombinedCredits struct {
	Cast []tmdbCombinedCreditEntry `json:"cast"`
	Crew []tmdbCombinedCreditEntry `json:"crew"`
}

// tmdbCombinedCreditEntry represents one credit.
type tmdbCombinedCreditEntry struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	Character    string  `json:"character"`
	Job          string  `json:"job"`
	Department   string  `json:"department"`
	MediaType    string  `json:"media_type"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	Popularity   float64 `json:"popularity"`
	EpisodeCount int     `json:"episode_count"`
	PosterPath   string  `json:"poster_path"`
	Overview     string  `json:"overview"`
}

// DisplayTitle returns the appropriate title.
func (e *tmdbCombinedCreditEntry) DisplayTitle() string {
	if e.Title != "" {
		return e.Title
	}
	return e.Name
}

// Year returns the year from the release/air date.
func (e *tmdbCombinedCreditEntry) Year() string {
	d := e.ReleaseDate
	if d == "" {
		d = e.FirstAirDate
	}
	if len(d) >= 4 {
		return d[:4]
	}
	return ""
}

// tmdbWatchProviders is the watch/providers response structure.
type tmdbWatchProviders struct {
	Results map[string]tmdbRegionProviders `json:"results"`
}

// tmdbRegionProviders contains providers for one region.
type tmdbRegionProviders struct {
	Link     string         `json:"link"`
	Flatrate []tmdbProvider `json:"flatrate"`
	Rent     []tmdbProvider `json:"rent"`
	Buy      []tmdbProvider `json:"buy"`
	Free     []tmdbProvider `json:"free"`
	Ads      []tmdbProvider `json:"ads"`
}

// tmdbProvider represents a single streaming/rental provider.
type tmdbProvider struct {
	ProviderID      int    `json:"provider_id"`
	ProviderName    string `json:"provider_name"`
	LogoPath        string `json:"logo_path"`
	DisplayPriority int    `json:"display_priority"`
}

// PATCH(title-resolution-must-signal-ambiguity: warn + --year/inline-year) —
// /search/* returns results ordered by TMDb's own relevance ranking, which is
// not exposed as a field and does not track either vote_count or the
// popularity value in the payload — for "Sabrina" the 1954 original leads the
// 1995 remake on both (1373 vs 703 ratings, 4.25 vs 3.60 popularity) and is
// still returned second. So a title shared by an original and a remake
// silently resolved to whichever the ranker preferred, by a rule we cannot
// inspect. Everything from here to resolveTVID exists to make that choice
// visible and overridable.

// inlineYearRe matches a trailing "(YYYY)" qualifier on a title argument, e.g.
// `Sabrina (1954)`. Only 1800-2999 is accepted so that titles ending in a
// parenthesised non-year (`Brazil (Director's Cut)`) are left alone.
var inlineYearRe = regexp.MustCompile(`^(.*?)\s*\(((?:1[89]|2[0-9])[0-9]{2})\)$`)

// splitInlineYear splits a trailing "(YYYY)" qualifier off a title argument.
// Returns the bare title and the year, or the original string and "" when no
// qualifier is present.
func splitInlineYear(arg string) (string, string) {
	m := inlineYearRe.FindStringSubmatch(strings.TrimSpace(arg))
	if m == nil || strings.TrimSpace(m[1]) == "" {
		return arg, ""
	}
	return strings.TrimSpace(m[1]), m[2]
}

// normalizeTitle folds a title for exact-match comparison: lowercased,
// apostrophes dropped, every other separator collapsed to a single space.
// "Sabrina" and "sabrina" match, as do "Spider-Man"/"Spider Man" and
// "Ocean's Eleven"/"Oceans Eleven"; "Sabrina" and "Sabrina Goes to Rome" do not.
func normalizeTitle(s string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '\'' || r == '’':
			// Apostrophes join rather than separate.
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingSpace && b.Len() > 0 {
				b.WriteRune(' ')
			}
			pendingSpace = false
			b.WriteRune(r)
		default:
			pendingSpace = true
		}
	}
	return b.String()
}

// exactTitleMatches returns the results whose display title (or original title)
// equals the query after normalization. The first element is the one TMDb
// ranked highest, i.e. the one the CLI would pick.
func exactTitleMatches(results []tmdbSearchResult, query string) []tmdbSearchResult {
	want := normalizeTitle(query)
	if want == "" {
		return nil
	}
	matches := make([]tmdbSearchResult, 0, len(results))
	for _, r := range results {
		alt := r.OriginalTitle
		if alt == "" {
			alt = r.OriginalName
		}
		if normalizeTitle(r.DisplayTitle()) == want || normalizeTitle(alt) == want {
			matches = append(matches, r)
		}
	}
	return matches
}

// notableVoteFloor is the rating count above which a same-titled alternative is
// worth telling the user about. TMDb carries a long tail of unrated shorts and
// home videos sharing famous titles ("Inception" 1980, "The Matrix" 2004); a
// notice that fires on every lookup is a notice nobody reads.
const notableVoteFloor = 50

// maxListedAlternatives caps how many alternatives the notice prints before
// deferring to `movies search`.
const maxListedAlternatives = 5

// personPopularityRatio is the share of the chosen person's popularity an
// same-named alternative must reach to be worth mentioning. People carry no
// vote counts and no year, so popularity is the only signal available; TMDb's
// person index holds dozens of near-zero namesakes for any well-known actor.
const personPopularityRatio = 0.25

// notableAlternatives filters the non-chosen exact matches down to the ones a
// user could plausibly have meant: either independently well-rated, or better
// rated than the entry TMDb's ranking put first. minVotes is 0 for searches
// with no meaningful vote data (people), which fall back to a
// relative-popularity gate instead.
func notableAlternatives(matches []tmdbSearchResult, minVotes int) []tmdbSearchResult {
	if len(matches) < 2 {
		return nil
	}
	chosen := matches[0]
	out := make([]tmdbSearchResult, 0, len(matches)-1)
	for _, r := range matches[1:] {
		if minVotes <= 0 {
			if r.Popularity >= chosen.Popularity*personPopularityRatio {
				out = append(out, r)
			}
			continue
		}
		if r.VoteCount >= minVotes || r.VoteCount >= chosen.VoteCount {
			out = append(out, r)
		}
	}
	return out
}

// describeResult renders "Title (Year)" for a notice line, falling back to the
// person's known-for department when there is no date to discriminate on.
func describeResult(r tmdbSearchResult) string {
	if y := r.Year(); y != "" {
		return fmt.Sprintf("%s (%s)", r.DisplayTitle(), y)
	}
	if r.KnownFor != "" {
		return fmt.Sprintf("%s (%s)", r.DisplayTitle(), r.KnownFor)
	}
	return r.DisplayTitle()
}

// ambiguousCandidate is one entry in an ambiguity record: enough to identify
// the title or person and to compare it against the entry that was chosen.
//
// Popularity is TMDb's own trending score, passed through as reported. It is
// NOT the order /search/* returned these in — that ranking is proprietary and
// unexposed, and it disagrees with this field often enough that the "Sabrina"
// case which motivated this record is one (the 1954 entry leads on popularity
// and on votes, and still came back second). It is included because it is the
// only comparable signal for people, who have no vote counts.
type ambiguousCandidate struct {
	TMDBID     int     `json:"tmdb_id"`
	Title      string  `json:"title"`
	Year       string  `json:"year,omitempty"`
	VoteCount  int     `json:"vote_count"`
	Popularity float64 `json:"popularity,omitempty"`
	KnownFor   string  `json:"known_for,omitempty"`
}

// Signals recorded on ambiguityMeta.Signal.
const (
	// signalBetterRated means the entry TMDb's search ranked first is not the
	// one with the most ratings — the failure mode that made `ratings
	// "Sabrina"` return the 1995 remake over the 1954 original. The comparison
	// is on vote_count, the only ranking input we can actually read.
	signalBetterRated = "alternative_better_rated"
	// signalMultipleMatches means several plausible entries share the query,
	// but the chosen one is also the best-rated.
	signalMultipleMatches = "multiple_exact_matches"
)

// ambiguityMeta is the machine-readable twin of the stderr notice. It is
// emitted under meta.ambiguous so a consumer that only reads stdout — a cron
// job, a scheduled script, anything running with 2>/dev/null — can still tell
// that the title it asked for resolved to a ranked guess.
type ambiguityMeta struct {
	Query        string               `json:"query"`
	Kind         string               `json:"kind"`
	MatchCount   int                  `json:"match_count"`
	Signal       string               `json:"signal"`
	Chosen       ambiguousCandidate   `json:"chosen"`
	Alternatives []ambiguousCandidate `json:"alternatives"`
	Hint         string               `json:"hint"`
}

// toCandidate projects a search result onto the ambiguity record shape.
func toCandidate(r tmdbSearchResult) ambiguousCandidate {
	return ambiguousCandidate{
		TMDBID:     r.ID,
		Title:      r.DisplayTitle(),
		Year:       r.Year(),
		VoteCount:  r.VoteCount,
		Popularity: r.Popularity,
		KnownFor:   r.KnownFor,
	}
}

// noteAmbiguity is the single entry point for an ambiguous resolution. It
// records the ambiguity for the JSON output and, unless --quiet, prints the
// human notice to stderr.
//
// The two channels are deliberately independent: --quiet is about terminal
// chatter, while the JSON field is a fact about how the result was resolved
// that a machine consumer needs either way. yearHint is the flag name the
// calling command exposes, or "" for commands that only accept the inline
// "(YYYY)" form.
func noteAmbiguity(flags *rootFlags, kindLabel, query string, matches []tmdbSearchResult, minVotes int, yearHint string) {
	alts := notableAlternatives(matches, minVotes)
	if len(alts) == 0 {
		return
	}
	chosen := matches[0]

	hint := fmt.Sprintf("Disambiguate with a %q suffix or the TMDb id.", "title (YYYY)")
	if yearHint != "" {
		hint = fmt.Sprintf("Disambiguate with %s <YYYY>, a %q suffix, or the TMDb id.", yearHint, "title (YYYY)")
	}
	// notableAlternatives preserves TMDb's relevance order, so the best-rated
	// alternative can sit anywhere in the list. Move it to the front: the
	// signal check below, the better-rated line in the stderr notice, and the
	// JSON alternatives all treat position 0 as the strongest contender.
	// notableAlternatives returns a fresh slice, so the swap is safe.
	best := 0
	for i := 1; i < len(alts); i++ {
		if alts[i].VoteCount > alts[best].VoteCount {
			best = i
		}
	}
	alts[0], alts[best] = alts[best], alts[0]

	signal := signalMultipleMatches
	if alts[0].VoteCount > chosen.VoteCount {
		signal = signalBetterRated
	}

	if flags != nil {
		rec := ambiguityMeta{
			Query:      query,
			Kind:       kindLabel,
			MatchCount: len(alts) + 1,
			Signal:     signal,
			Chosen:     toCandidate(chosen),
			Hint:       hint,
		}
		// The JSON record carries every notable alternative; only the stderr
		// notice truncates, because a terminal has a reader and a parser
		// doesn't.
		for _, r := range alts {
			rec.Alternatives = append(rec.Alternatives, toCandidate(r))
		}
		flags.ambiguities = append(flags.ambiguities, rec)
	}

	if flags != nil && flags.quiet {
		return
	}
	printAmbiguityNotice(os.Stderr, kindLabel, query, chosen, alts, signal, hint)
}

// printAmbiguityNotice writes the human-facing disambiguation notice. It only
// ever writes to the given stderr-like writer — stdout stays reserved for the
// response document.
func printAmbiguityNotice(w io.Writer, kindLabel, query string, chosen tmdbSearchResult, alts []tmdbSearchResult, signal, hint string) {
	fmt.Fprintf(w, "warn: %q matches %d %s on TMDb; using id %d — %s.\n",
		query, len(alts)+1, kindLabel, chosen.ID, describeResult(chosen))
	// The trap worth naming out loud: /search ranks by a relevance score we
	// cannot see, so a remake can outrank a better-rated original.
	if signal == signalBetterRated {
		fmt.Fprintf(w, "      TMDb's search relevance put it first, but %s has more ratings (%d vs %d).\n",
			describeResult(alts[0]), alts[0].VoteCount, chosen.VoteCount)
	}
	fmt.Fprintln(w, "      Other matches:")
	shown := alts
	if len(shown) > maxListedAlternatives {
		shown = shown[:maxListedAlternatives]
	}
	for _, r := range shown {
		fmt.Fprintf(w, "        %d  %s\n", r.ID, describeResult(r))
	}
	if rest := len(alts) - len(shown); rest > 0 {
		fmt.Fprintf(w, "        … and %d more\n", rest)
	}
	fmt.Fprintf(w, "      %s\n", hint)
}

// searchByTitle is the shared movie/tv title search. year is optional and is
// pushed down to TMDb via yearParam rather than post-filtered, so the
// constraint applies before TMDb's ranking truncates the page.
func searchByTitle(c *client.Client, flags *rootFlags, path, yearParam, kindLabel, noun, title, year, yearHint string) (int, string, error) {
	params := map[string]string{"query": title}
	if year != "" {
		params[yearParam] = year
	}
	data, err := c.Get(path, params)
	if err != nil {
		return 0, "", fmt.Errorf("searching for %q: %w", title, err)
	}
	var resp tmdbSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, "", fmt.Errorf("parsing %s search results: %w", kindLabel, err)
	}
	if len(resp.Results) == 0 {
		if year != "" {
			return 0, "", fmt.Errorf("no %s found for %q released in %s", noun, title, year)
		}
		return 0, "", fmt.Errorf("no %s found for %q", noun, title)
	}
	// A year constraint already narrowed the field; only warn when the caller
	// gave us nothing to disambiguate with.
	if year == "" {
		noteAmbiguity(flags, kindLabel, title, exactTitleMatches(resp.Results, title), notableVoteFloor, yearHint)
	}
	r := resp.Results[0]
	return r.ID, r.DisplayTitle(), nil
}

// searchMovieByTitle searches TMDb for a movie by title and returns the top
// result's ID. year is optional ("" for none) and maps to TMDb's
// primary_release_year.
func searchMovieByTitle(c *client.Client, flags *rootFlags, title, year, yearHint string) (int, string, error) {
	return searchByTitle(c, flags, "/search/movie", "primary_release_year", "titles", "movies", title, year, yearHint)
}

// searchTVByTitle searches TMDb for a TV show by title and returns the top
// result. year is optional and maps to TMDb's first_air_date_year.
func searchTVByTitle(c *client.Client, flags *rootFlags, title, year, yearHint string) (int, string, error) {
	return searchByTitle(c, flags, "/search/tv", "first_air_date_year", "shows", "TV shows", title, year, yearHint)
}

// searchPersonByName searches TMDb for a person by name and returns the top
// result, warning on stderr when several people share the name.
func searchPersonByName(c *client.Client, flags *rootFlags, name string) (*tmdbSearchResult, error) {
	data, err := c.Get("/search/person", map[string]string{"query": name})
	if err != nil {
		return nil, fmt.Errorf("searching for person %q: %w", name, err)
	}
	var resp tmdbSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("no person found for %q", name)
	}
	noteAmbiguity(flags, "people", name, exactTitleMatches(resp.Results, name), 0, "")
	return &resp.Results[0], nil
}

// resolveMovieID resolves a string argument to a TMDb movie ID. If the
// argument is numeric, returns it directly; otherwise searches by title. year
// is the value of the command's --year flag ("" when unset or unsupported); an
// inline "Title (YYYY)" qualifier on arg is honored when year is empty.
func resolveMovieID(c *client.Client, flags *rootFlags, arg, year, yearHint string) (int, string, error) {
	if id, err := strconv.Atoi(arg); err == nil {
		return id, "", nil
	}
	title := arg
	if year == "" {
		title, year = splitInlineYear(arg)
	}
	id, name, err := searchMovieByTitle(c, flags, title, year, yearHint)
	if err != nil && year != "" && title != arg {
		// The "(YYYY)" suffix may have been part of the real title rather than
		// a qualifier — retry with the argument exactly as the user typed it.
		return searchMovieByTitle(c, flags, arg, "", yearHint)
	}
	return id, name, err
}

// resolveTVID resolves a string argument to a TMDb TV ID. See resolveMovieID
// for the year/inline-qualifier semantics.
func resolveTVID(c *client.Client, flags *rootFlags, arg, year, yearHint string) (int, string, error) {
	if id, err := strconv.Atoi(arg); err == nil {
		return id, "", nil
	}
	title := arg
	if year == "" {
		title, year = splitInlineYear(arg)
	}
	id, name, err := searchTVByTitle(c, flags, title, year, yearHint)
	if err != nil && year != "" && title != arg {
		return searchTVByTitle(c, flags, arg, "", yearHint)
	}
	return id, name, err
}

// validateYearFlag rejects a --year value that isn't a plausible 4-digit year.
func validateYearFlag(raw string) (string, error) {
	y := strings.TrimSpace(raw)
	if y == "" {
		return "", nil
	}
	n, err := strconv.Atoi(y)
	if err != nil || len(y) != 4 || n < 1800 || n > 2999 {
		return "", usageErr(fmt.Errorf("--year must be a 4-digit year, got %q", raw))
	}
	return y, nil
}

// getMovieDetail fetches full movie details from TMDb. The raw bytes are also
// returned so callers needing access to fields not modeled in tmdbMovieDetail
// (e.g. raw watch/providers payload) can re-parse the relevant subtree.
func getMovieDetail(c *client.Client, movieID int, appendToResponse string) (*tmdbMovieDetail, json.RawMessage, error) {
	path := fmt.Sprintf("/movie/%d", movieID)
	params := map[string]string{}
	if appendToResponse != "" {
		params["append_to_response"] = appendToResponse
	}
	data, err := c.Get(path, params)
	if err != nil {
		return nil, nil, err
	}
	var detail tmdbMovieDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, data, fmt.Errorf("parsing movie detail: %w", err)
	}
	// When external_ids was appended but the response shape doesn't decode
	// into ExternalIDs (TMDb sometimes returns it embedded vs flat), fall back
	// to the imdb_id field at the top level — that's always populated.
	if detail.ExternalIDs == nil && detail.ImdbID != "" {
		detail.ExternalIDs = &tmdbExternalIDs{IMDbID: detail.ImdbID}
	}
	return &detail, data, nil
}

// getTVDetail fetches full TV show details. mirrors getMovieDetail.
func getTVDetail(c *client.Client, tvID int, appendToResponse string) (*tmdbTVDetail, json.RawMessage, error) {
	path := fmt.Sprintf("/tv/%d", tvID)
	params := map[string]string{}
	if appendToResponse != "" {
		params["append_to_response"] = appendToResponse
	}
	data, err := c.Get(path, params)
	if err != nil {
		return nil, nil, err
	}
	var detail tmdbTVDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, data, fmt.Errorf("parsing tv detail: %w", err)
	}
	if detail.ExternalIDs != nil {
		detail.IMDbID = detail.ExternalIDs.IMDbID
	}
	return &detail, data, nil
}

// genreNames returns a comma-separated string of genre names from a movie detail.
func genreNames(detail *tmdbMovieDetail) string {
	if detail == nil {
		return ""
	}
	names := make([]string, 0, len(detail.Genres))
	for _, g := range detail.Genres {
		names = append(names, g.Name)
	}
	return strings.Join(names, ", ")
}

// formatMoney formats a number as a dollar amount (e.g. $150,000,000).
func formatMoney(amount int64) string {
	if amount == 0 {
		return "N/A"
	}
	s := fmt.Sprintf("%d", amount)
	n := len(s)
	if n <= 3 {
		return "$" + s
	}
	var result strings.Builder
	result.WriteString("$")
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// formatRuntimeMinutes returns a humanized runtime string ("1h 42m" for 102).
// Returns "N/A" for zero/negative values.
func formatRuntimeMinutes(mins int) string {
	if mins <= 0 {
		return "N/A"
	}
	h := mins / 60
	m := mins % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// resolveGenreIDByName resolves a genre name (case-insensitive) to a TMDb genre ID
// for the given mediaType ("movie" or "tv"). Returns 0 and an error if not found.
func resolveGenreIDByName(c *client.Client, mediaType, name string) (int, error) {
	path := fmt.Sprintf("/genre/%s/list", mediaType)
	data, err := c.Get(path, map[string]string{})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Genres []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing genre list: %w", err)
	}
	want := strings.ToLower(name)
	for _, g := range resp.Genres {
		if strings.ToLower(g.Name) == want {
			return g.ID, nil
		}
	}
	return 0, fmt.Errorf("genre %q not found in TMDb %s genre list", name, mediaType)
}
