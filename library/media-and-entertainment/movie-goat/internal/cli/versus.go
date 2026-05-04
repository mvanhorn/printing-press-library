package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb"
)

// versusTitle is one half of the comparison. It carries the canonical TMDb
// fields and a subset of OMDb-derived fields so --select queries against the
// JSON output have stable paths.
type versusTitle struct {
	ID          int           `json:"id"`
	Kind        string        `json:"kind"`
	Title       string        `json:"title"`
	Year        string        `json:"year"`
	Runtime     int           `json:"runtime,omitempty"`
	VoteAverage float64       `json:"vote_average"`
	VoteCount   int           `json:"vote_count"`
	Genres      string        `json:"genres,omitempty"`
	IMDbID      string        `json:"imdb_id,omitempty"`
	Ratings     ratingsValues `json:"ratings"`
	Awards      string        `json:"awards,omitempty"`
	Providers   []tonightProv `json:"providers,omitempty"`
}

type versusOverlap struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	RoleInA string `json:"role_in_a,omitempty"`
	RoleInB string `json:"role_in_b,omitempty"`
}

type versusOutput struct {
	A           versusTitle     `json:"a"`
	B           versusTitle     `json:"b"`
	CastOverlap []versusOverlap `json:"cast_overlap"`
}

func newVersusCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagType string

	cmd := &cobra.Command{
		Use:         "versus <id-or-title-a> <id-or-title-b>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Compare two titles head-to-head: ratings, runtime, cast overlap",
		Long: `Resolve two ids or titles, fetch detail with credits, external_ids, and
watch/providers appended, overlay OMDb (IMDb / Rotten Tomatoes / Metacritic)
when OMDB_API_KEY is set, and emit the side-by-side comparison plus the
top-billed cast overlap (people credited as cast on both titles).`,
		Example: `  movie-goat-pp-cli versus 550 27205
  movie-goat-pp-cli versus "The Dark Knight" "Inception"
  movie-goat-pp-cli versus 1396 60625 --type tv --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("versus requires two titles or ids"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			kind := strings.ToLower(strings.TrimSpace(flagType))
			if kind == "" {
				kind = "movie"
			}
			if kind != "movie" && kind != "tv" {
				return usageErr(fmt.Errorf("--type must be \"movie\" or \"tv\", got %q", flagType))
			}
			region := strings.ToUpper(strings.TrimSpace(flagRegion))
			if region == "" {
				region = "US"
			}
			omdbKey := strings.TrimSpace(os.Getenv("OMDB_API_KEY"))

			loadOne := func(arg string) (versusTitle, *tmdbCredits, error) {
				vt := versusTitle{Kind: kind}
				appendStr := "credits,external_ids,watch/providers"
				switch kind {
				case "movie":
					id, _, rerr := resolveMovieID(c, arg)
					if rerr != nil {
						return vt, nil, classifyAPIError(rerr)
					}
					detail, _, derr := getMovieDetail(c, id, appendStr)
					if derr != nil {
						return vt, nil, classifyAPIError(derr)
					}
					vt.ID = detail.ID
					vt.Title = detail.Title
					if len(detail.ReleaseDate) >= 4 {
						vt.Year = detail.ReleaseDate[:4]
					}
					vt.Runtime = detail.Runtime
					vt.VoteAverage = detail.VoteAverage
					vt.VoteCount = detail.VoteCount
					vt.Genres = genreNames(detail)
					vt.IMDbID = detail.ImdbID
					if detail.ExternalIDs != nil && vt.IMDbID == "" {
						vt.IMDbID = detail.ExternalIDs.IMDbID
					}
					if detail.VoteAverage > 0 {
						vt.Ratings.TMDB = fmt.Sprintf("%.1f", detail.VoteAverage)
					}
					vt.Providers, _, _ = parseAppendedProviders(detail.WatchProviders, region)
					return vt, detail.Credits, nil
				case "tv":
					id, _, rerr := resolveTVID(c, arg)
					if rerr != nil {
						return vt, nil, classifyAPIError(rerr)
					}
					detail, _, derr := getTVDetail(c, id, appendStr)
					if derr != nil {
						return vt, nil, classifyAPIError(derr)
					}
					vt.ID = detail.ID
					vt.Title = detail.Name
					if len(detail.FirstAirDate) >= 4 {
						vt.Year = detail.FirstAirDate[:4]
					}
					if len(detail.EpisodeRunTime) > 0 {
						vt.Runtime = detail.EpisodeRunTime[0]
					}
					vt.VoteAverage = detail.VoteAverage
					vt.VoteCount = detail.VoteCount
					if len(detail.Genres) > 0 {
						names := make([]string, 0, len(detail.Genres))
						for _, g := range detail.Genres {
							names = append(names, g.Name)
						}
						vt.Genres = strings.Join(names, ", ")
					}
					if detail.ExternalIDs != nil {
						vt.IMDbID = detail.ExternalIDs.IMDbID
					}
					if detail.VoteAverage > 0 {
						vt.Ratings.TMDB = fmt.Sprintf("%.1f", detail.VoteAverage)
					}
					vt.Providers, _, _ = parseAppendedProviders(detail.WatchProviders, region)
					return vt, detail.Credits, nil
				}
				return vt, nil, fmt.Errorf("unsupported kind %q", kind)
			}

			a, creditsA, err := loadOne(args[0])
			if err != nil {
				return err
			}
			b, creditsB, err := loadOne(args[1])
			if err != nil {
				return err
			}

			// OMDb enrichment for both titles when key present.
			if omdbKey != "" {
				if a.IMDbID != "" {
					if r, oerr := omdb.Fetch(a.IMDbID, omdbKey); oerr == nil && r != nil {
						populateOMDbRatings(&a, r)
					} else if oerr != nil && omdb.IsRateLimit(oerr) {
						return rateLimitErr(oerr)
					}
				}
				if b.IMDbID != "" {
					if r, oerr := omdb.Fetch(b.IMDbID, omdbKey); oerr == nil && r != nil {
						populateOMDbRatings(&b, r)
					} else if oerr != nil && omdb.IsRateLimit(oerr) {
						return rateLimitErr(oerr)
					}
				}
			}

			overlap := computeCastOverlap(creditsA, creditsB)

			out := versusOutput{A: a, B: b, CastOverlap: overlap}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			col1 := truncate(a.Title, 30)
			col2 := truncate(b.Title, 30)
			fmt.Fprintf(w, "%-22s  %-30s  vs  %-30s\n", "", col1, col2)
			fmt.Fprintln(w, strings.Repeat("-", 90))
			versusRow(w, "Year", a.Year, b.Year)
			versusRow(w, "Runtime", formatRuntimeMinutes(a.Runtime), formatRuntimeMinutes(b.Runtime))
			versusRow(w, "Genres", a.Genres, b.Genres)
			versusRow(w, "TMDb", a.Ratings.TMDB, b.Ratings.TMDB)
			versusRow(w, "IMDb", naIfEmpty(a.Ratings.IMDb), naIfEmpty(b.Ratings.IMDb))
			versusRow(w, "Rotten Tomatoes", naIfEmpty(a.Ratings.RottenTomatoes), naIfEmpty(b.Ratings.RottenTomatoes))
			versusRow(w, "Metacritic", naIfEmpty(a.Ratings.Metacritic), naIfEmpty(b.Ratings.Metacritic))
			versusRow(w, "Awards", truncate(naIfEmpty(a.Awards), 30), truncate(naIfEmpty(b.Awards), 30))
			if len(overlap) > 0 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Cast overlap:")
				for _, p := range overlap {
					fmt.Fprintf(w, "  %s — %s / %s\n", p.Name, orDash(p.RoleInA), orDash(p.RoleInB))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagRegion, "region", "US", "Region for /watch/providers lookup")
	cmd.Flags().StringVar(&flagType, "type", "movie", "Media type: movie or tv")
	return cmd
}

func versusRow(w interface{ Write([]byte) (int, error) }, label, lhs, rhs string) {
	fmt.Fprintf(w, "%-22s  %-30s  vs  %-30s\n", label, truncate(lhs, 30), truncate(rhs, 30))
}

func populateOMDbRatings(vt *versusTitle, r *omdb.Result) {
	if r == nil {
		return
	}
	if v := r.ImdbRating; v != "" && v != "N/A" {
		vt.Ratings.IMDb = v
	}
	if v := r.RatingBySource("Rotten Tomatoes"); v != "" {
		vt.Ratings.RottenTomatoes = v
	}
	if v := r.RatingBySource("Metacritic"); v != "" {
		vt.Ratings.Metacritic = v
	} else if r.Metascore != "" && r.Metascore != "N/A" {
		vt.Ratings.Metacritic = r.Metascore + "/100"
	}
	if r.Awards != "" && r.Awards != "N/A" {
		vt.Awards = r.Awards
	}
}

// computeCastOverlap intersects top-billed cast lists between the two titles.
// Order in detailA is preserved so the output stays deterministic.
func computeCastOverlap(a, b *tmdbCredits) []versusOverlap {
	if a == nil || b == nil {
		return nil
	}
	bByID := make(map[int]string, len(b.Cast))
	for _, m := range b.Cast {
		bByID[m.ID] = m.Character
	}
	var out []versusOverlap
	seen := map[int]bool{}
	for _, m := range a.Cast {
		if charB, ok := bByID[m.ID]; ok && !seen[m.ID] {
			seen[m.ID] = true
			out = append(out, versusOverlap{
				ID:      m.ID,
				Name:    m.Name,
				RoleInA: m.Character,
				RoleInB: charB,
			})
		}
	}
	return out
}

// parseAppendedProviders extracts provider rows from an appended watch/providers
// payload for one region. Returns the provider list, whether the region had
// any flatrate row, and any parse error (callers can ignore on best-effort).
func parseAppendedProviders(raw []byte, region string) ([]tonightProv, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	var resp tmdbWatchProviders
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, false, err
	}
	r, ok := resp.Results[region]
	if !ok {
		return nil, false, nil
	}
	var rows []tonightProv
	hasFlatrate := false
	for _, p := range r.Flatrate {
		hasFlatrate = true
		rows = append(rows, tonightProv{Name: p.ProviderName, Kind: "flatrate"})
	}
	for _, p := range r.Rent {
		rows = append(rows, tonightProv{Name: p.ProviderName, Kind: "rent"})
	}
	for _, p := range r.Buy {
		rows = append(rows, tonightProv{Name: p.ProviderName, Kind: "buy"})
	}
	for _, p := range r.Free {
		rows = append(rows, tonightProv{Name: p.ProviderName, Kind: "free"})
	}
	for _, p := range r.Ads {
		rows = append(rows, tonightProv{Name: p.ProviderName, Kind: "ads"})
	}
	return rows, hasFlatrate, nil
}
