package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb"
)

// careerCredit is one row in the filmography table.
type careerCredit struct {
	ID         int     `json:"id"`
	Kind       string  `json:"kind"` // movie | tv
	Title      string  `json:"title"`
	Year       string  `json:"year"`
	Role       string  `json:"role"` // "actor" | job string for crew
	Character  string  `json:"character,omitempty"`
	Job        string  `json:"job,omitempty"`
	RatingTMDB float64 `json:"rating_tmdb"`
	RatingIMDb string  `json:"rating_imdb,omitempty"`
	RatingRT   string  `json:"rating_rt,omitempty"`
	Runtime    string  `json:"runtime,omitempty"`
}

type careerOutput struct {
	Person  careerPerson   `json:"person"`
	Credits []careerCredit `json:"credits"`
}

type careerPerson struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Birthday string `json:"birthday,omitempty"`
	KnownFor string `json:"known_for,omitempty"`
}

const careerOMDBCap = 50

func newCareerCmd(flags *rootFlags) *cobra.Command {
	var flagSince int
	var flagRole string

	cmd := &cobra.Command{
		Use:         "career <person-id-or-name>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Explore a person's filmography with optional OMDb rating overlay",
		Long: `Resolve a TMDb person id (numeric) or name (search), pull combined_credits,
filter by role and --since year, then optionally enrich the first --limit
credits with OMDb's IMDb / Rotten Tomatoes ratings.

Roles:
  actor    cast credits
  director crew where job = "Director"
  dp       crew where job in {"Director of Photography", "Cinematography"}
  writer   crew where department = "Writing"
  crew     all crew credits
  (default) cast + crew

Set OMDB_API_KEY to enable IMDb / Rotten Tomatoes enrichment. To respect
OMDb's per-day quota, no more than 50 credits are enriched per call.`,
		Example: `  movie-goat-pp-cli career 525
  movie-goat-pp-cli career "Greta Gerwig" --role director --since 2010
  movie-goat-pp-cli career "Roger Deakins" --role dp --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			query := strings.Join(args, " ")
			role := strings.ToLower(strings.TrimSpace(flagRole))
			switch role {
			case "", "actor", "director", "dp", "writer", "crew":
			default:
				return usageErr(fmt.Errorf("--role must be one of actor|director|dp|writer|crew, got %q", flagRole))
			}

			// 1. Resolve person id.
			var personID int
			var personName string
			if id, perr := strconv.Atoi(query); perr == nil {
				personID = id
			} else {
				p, err := searchPersonByName(c, query)
				if err != nil {
					return classifyAPIError(err)
				}
				personID = p.ID
				personName = p.DisplayTitle()
			}

			// 2. Fetch combined credits (single call via append_to_response).
			path := fmt.Sprintf("/person/%d", personID)
			data, err := c.Get(path, map[string]string{"append_to_response": "combined_credits"})
			if err != nil {
				return classifyAPIError(err)
			}
			var person tmdbPersonDetail
			if err := json.Unmarshal(data, &person); err != nil {
				return fmt.Errorf("parsing person: %w", err)
			}
			if personName == "" {
				personName = person.Name
			}

			// 3. Filter by role and assemble credits.
			credits := assembleCareerCredits(person.CombinedCredits, role)

			// 4. --since filter.
			if flagSince > 0 {
				kept := credits[:0]
				for _, cr := range credits {
					y, _ := strconv.Atoi(cr.Year)
					if y >= flagSince {
						kept = append(kept, cr)
					}
				}
				credits = kept
			}

			// 5. Optional OMDb enrichment, capped at careerOMDBCap.
			omdbKey := strings.TrimSpace(os.Getenv("OMDB_API_KEY"))
			if omdbKey != "" {
				cap := len(credits)
				if cap > careerOMDBCap {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: enriching only first %d of %d credits with OMDb to respect rate limits\n", careerOMDBCap, cap)
					cap = careerOMDBCap
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
				defer cancel()
				type imdbInfo struct {
					id         int
					kind       string
					imdbID     string
					ratingIMDb string
					ratingRT   string
					runtime    string
				}
				// First pass: resolve external_ids for each enriched credit.
				toFetch := credits[:cap]
				results, errs := cliutil.FanoutRun(ctx, toFetch,
					func(cr careerCredit) string { return cr.Title },
					func(_ context.Context, cr careerCredit) (imdbInfo, error) {
						info := imdbInfo{id: cr.ID, kind: cr.Kind}
						switch cr.Kind {
						case "movie":
							d, _, derr := getMovieDetail(c, cr.ID, "external_ids")
							if derr != nil {
								return info, derr
							}
							info.imdbID = d.ImdbID
							if d.ExternalIDs != nil && info.imdbID == "" {
								info.imdbID = d.ExternalIDs.IMDbID
							}
							if d.Runtime > 0 {
								info.runtime = formatRuntimeMinutes(d.Runtime)
							}
						case "tv":
							d, _, derr := getTVDetail(c, cr.ID, "external_ids")
							if derr != nil {
								return info, derr
							}
							if d.ExternalIDs != nil {
								info.imdbID = d.ExternalIDs.IMDbID
							}
							if len(d.EpisodeRunTime) > 0 {
								info.runtime = formatRuntimeMinutes(d.EpisodeRunTime[0])
							}
						}
						if info.imdbID == "" {
							return info, nil
						}
						res, oerr := omdb.Fetch(info.imdbID, omdbKey)
						if oerr != nil && omdb.IsRateLimit(oerr) {
							return info, oerr
						}
						if oerr != nil {
							return info, nil
						}
						if res != nil {
							if v := res.ImdbRating; v != "" && v != "N/A" {
								info.ratingIMDb = v
							}
							if v := res.RatingBySource("Rotten Tomatoes"); v != "" {
								info.ratingRT = v
							}
						}
						return info, nil
					})
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				byID := make(map[string]imdbInfo, len(results))
				for _, r := range results {
					byID[fmt.Sprintf("%s-%d", r.Value.kind, r.Value.id)] = r.Value
				}
				for i := range credits[:cap] {
					info, ok := byID[fmt.Sprintf("%s-%d", credits[i].Kind, credits[i].ID)]
					if !ok {
						continue
					}
					credits[i].RatingIMDb = info.ratingIMDb
					credits[i].RatingRT = info.ratingRT
					credits[i].Runtime = info.runtime
				}
			}

			// 6. Sort by year desc.
			sort.SliceStable(credits, func(i, j int) bool {
				return credits[i].Year > credits[j].Year
			})

			out := careerOutput{
				Person: careerPerson{
					ID:       personID,
					Name:     personName,
					Birthday: person.Birthday,
					KnownFor: person.KnownFor,
				},
				Credits: credits,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, personName)
			fmt.Fprintln(w, strings.Repeat("=", len(personName)))
			if person.KnownFor != "" {
				fmt.Fprintf(w, "Known for: %s\n", person.KnownFor)
			}
			if person.Birthday != "" {
				fmt.Fprintf(w, "Born: %s\n", person.Birthday)
			}
			fmt.Fprintln(w)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "Year\tTitle\tRole\tTMDb\tIMDb\tRT\tRuntime")
			for _, cr := range credits {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%s\t%s\t%s\n",
					orDash(cr.Year),
					truncate(cr.Title, 40),
					truncate(cr.displayRole(), 30),
					cr.RatingTMDB,
					orDash(cr.RatingIMDb),
					orDash(cr.RatingRT),
					orDash(cr.Runtime))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&flagSince, "since", 0, "Drop credits before this year")
	cmd.Flags().StringVar(&flagRole, "role", "", "Filter by role: actor | director | dp | writer | crew")
	return cmd
}

func (c careerCredit) displayRole() string {
	if c.Job != "" {
		return c.Job
	}
	if c.Character != "" {
		return c.Character
	}
	return c.Role
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// assembleCareerCredits walks combined_credits and emits one careerCredit per
// (id, kind, role-bucket) tuple. role determines which buckets to read; the
// default ("") means cast + crew, dedup'd by (id, kind, job/character).
func assembleCareerCredits(cc *tmdbCombinedCredits, role string) []careerCredit {
	if cc == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []careerCredit

	addRow := func(cr careerCredit, dedupKey string) {
		if seen[dedupKey] {
			return
		}
		seen[dedupKey] = true
		out = append(out, cr)
	}

	// Cast bucket (actor)
	if role == "" || role == "actor" {
		for _, e := range cc.Cast {
			cr := careerCredit{
				ID:         e.ID,
				Kind:       e.MediaType,
				Title:      e.DisplayTitle(),
				Year:       e.Year(),
				Role:       "actor",
				Character:  e.Character,
				RatingTMDB: e.VoteAverage,
			}
			addRow(cr, fmt.Sprintf("cast-%s-%d", cr.Kind, cr.ID))
		}
	}
	// Crew bucket — applies to director, dp, writer, crew, and default.
	if role != "actor" {
		for _, e := range cc.Crew {
			match := false
			switch role {
			case "director":
				match = e.Job == "Director"
			case "dp":
				match = e.Job == "Director of Photography" || e.Job == "Cinematography"
			case "writer":
				match = e.Department == "Writing"
			case "crew", "":
				match = true
			}
			if !match {
				continue
			}
			cr := careerCredit{
				ID:         e.ID,
				Kind:       e.MediaType,
				Title:      e.DisplayTitle(),
				Year:       e.Year(),
				Role:       strings.ToLower(strings.TrimSpace(e.Department)),
				Job:        e.Job,
				RatingTMDB: e.VoteAverage,
			}
			if cr.Role == "" {
				cr.Role = "crew"
			}
			addRow(cr, fmt.Sprintf("crew-%s-%d-%s", cr.Kind, cr.ID, e.Job))
		}
	}
	return out
}
