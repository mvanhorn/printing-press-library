package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newCareerCmd(flags *rootFlags) *cobra.Command {
	var flagSort string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "career <person-name-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Explore a person's complete filmography with stats and highlights",
		Long: `Look up an actor, director, or crew member's full career across movies and TV.
Shows a filmography table sorted by year, rating, or popularity, plus career summary stats.`,
		Example: `  movie-goat-pp-cli career "Christopher Nolan"
  movie-goat-pp-cli career "Cate Blanchett" --sort rating
  movie-goat-pp-cli career 17419 --sort popularity
  movie-goat-pp-cli career "Denis Villeneuve" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "GET /search/person?query=%s\nGET /person/<id>?append_to_response=combined_credits\n", strings.Join(args, " "))
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			query := strings.Join(args, " ")
			var personID int
			var personName string

			// Check if numeric ID
			if id, err := strconv.Atoi(query); err == nil {
				personID = id
			} else {
				// Search for person
				result, err := searchPersonByName(c, query)
				if err != nil {
					return classifyAPIError(fmt.Errorf("searching for %q: %w", query, err))
				}
				personID = result.ID
				personName = result.DisplayTitle()
			}

			// Fetch person details with combined credits
			path := fmt.Sprintf("/person/%d", personID)
			data, err := c.Get(path, map[string]string{
				"append_to_response": "combined_credits",
			})
			if err != nil {
				return classifyAPIError(err)
			}

			var person tmdbPersonDetail
			if err := json.Unmarshal(data, &person); err != nil {
				return fmt.Errorf("parsing person details: %w", err)
			}
			if personName == "" {
				personName = person.Name
			}

			// Build filmography from cast + crew credits
			type filmEntry struct {
				ID         int     `json:"id"`
				Title      string  `json:"title"`
				Year       string  `json:"year"`
				Role       string  `json:"role"`
				Rating     float64 `json:"vote_average"`
				VoteCount  int     `json:"vote_count"`
				MediaType  string  `json:"media_type"`
				Popularity float64 `json:"popularity"`
			}

			var filmography []filmEntry
			seen := make(map[string]bool) // dedup by id+mediatype+role

			if person.CombinedCredits != nil {
				for _, credit := range person.CombinedCredits.Cast {
					key := fmt.Sprintf("%d-%s-cast", credit.ID, credit.MediaType)
					if seen[key] {
						continue
					}
					seen[key] = true
					filmography = append(filmography, filmEntry{
						ID:         credit.ID,
						Title:      credit.DisplayTitle(),
						Year:       credit.Year(),
						Role:       credit.Character,
						Rating:     credit.VoteAverage,
						VoteCount:  credit.VoteCount,
						MediaType:  credit.MediaType,
						Popularity: credit.Popularity,
					})
				}
				for _, credit := range person.CombinedCredits.Crew {
					key := fmt.Sprintf("%d-%s-%s", credit.ID, credit.MediaType, credit.Job)
					if seen[key] {
						continue
					}
					seen[key] = true
					filmography = append(filmography, filmEntry{
						ID:         credit.ID,
						Title:      credit.DisplayTitle(),
						Year:       credit.Year(),
						Role:       credit.Job,
						Rating:     credit.VoteAverage,
						VoteCount:  credit.VoteCount,
						MediaType:  credit.MediaType,
						Popularity: credit.Popularity,
					})
				}
			}

			// Sort filmography
			switch flagSort {
			case "rating":
				sort.Slice(filmography, func(i, j int) bool {
					return filmography[i].Rating > filmography[j].Rating
				})
			case "popularity":
				sort.Slice(filmography, func(i, j int) bool {
					return filmography[i].Popularity > filmography[j].Popularity
				})
			default: // "year" — most recent first
				sort.Slice(filmography, func(i, j int) bool {
					if filmography[i].Year == filmography[j].Year {
						return filmography[i].Popularity > filmography[j].Popularity
					}
					return filmography[i].Year > filmography[j].Year
				})
			}

			// Apply limit
			if flagLimit > 0 && len(filmography) > flagLimit {
				filmography = filmography[:flagLimit]
			}

			// Compute summary stats
			totalCredits := len(filmography)
			var totalRating float64
			ratedCount := 0
			yearCredits := make(map[string]int)
			var topRated filmEntry

			for _, f := range filmography {
				if f.Rating > 0 && f.VoteCount >= 10 {
					totalRating += f.Rating
					ratedCount++
					if f.Rating > topRated.Rating {
						topRated = f
					}
				}
				if f.Year != "" {
					yearCredits[f.Year]++
				}
			}

			avgRating := 0.0
			if ratedCount > 0 {
				avgRating = totalRating / float64(ratedCount)
			}

			peakYear := ""
			peakCount := 0
			for y, cnt := range yearCredits {
				if cnt > peakCount {
					peakCount = cnt
					peakYear = y
				}
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				output := map[string]any{
					"person_id":      personID,
					"name":           personName,
					"biography":      person.Biography,
					"birthday":       person.Birthday,
					"deathday":       person.Deathday,
					"place_of_birth": person.PlaceOfBirth,
					"known_for":      person.KnownFor,
					"filmography":    filmography,
					"summary": map[string]any{
						"total_credits":     totalCredits,
						"average_rating":    fmt.Sprintf("%.1f", avgRating),
						"peak_year":         peakYear,
						"peak_year_credits": peakCount,
						"top_rated":         topRated.Title,
						"top_rating":        topRated.Rating,
					},
				}
				outData, _ := json.Marshal(output)
				if flags.compact {
					outData = compactFields(json.RawMessage(outData))
				}
				if flags.selectFields != "" {
					outData = filterFields(json.RawMessage(outData), flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(outData), true)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", personName)
			fmt.Fprintf(w, "%s\n", strings.Repeat("=", len(personName)))
			if person.KnownFor != "" {
				fmt.Fprintf(w, "Known for: %s\n", person.KnownFor)
			}
			if person.Birthday != "" {
				bday := person.Birthday
				if person.Deathday != "" {
					bday += " - " + person.Deathday
				}
				fmt.Fprintf(w, "Born: %s\n", bday)
			}
			if person.PlaceOfBirth != "" {
				fmt.Fprintf(w, "From: %s\n", person.PlaceOfBirth)
			}
			fmt.Fprintln(w)

			// Summary
			fmt.Fprintln(w, "Career Summary:")
			fmt.Fprintf(w, "  Total credits:  %d\n", totalCredits)
			if ratedCount > 0 {
				fmt.Fprintf(w, "  Average rating: %.1f/10\n", avgRating)
			}
			if peakYear != "" {
				fmt.Fprintf(w, "  Peak year:      %s (%d credits)\n", peakYear, peakCount)
			}
			if topRated.Title != "" {
				fmt.Fprintf(w, "  Top rated:      %s (%.1f)\n", topRated.Title, topRated.Rating)
			}
			fmt.Fprintln(w)

			// Filmography table
			fmt.Fprintln(w, "Filmography:")
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "Year\tTitle\tRole\tRating\tType")
			for _, f := range filmography {
				year := f.Year
				if year == "" {
					year = "TBA"
				}
				mtype := "Movie"
				if f.MediaType == "tv" {
					mtype = "TV"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%s\n",
					year, truncate(f.Title, 35), truncate(f.Role, 25), f.Rating, mtype)
			}
			tw.Flush()

			return nil
		},
	}

	cmd.Flags().StringVar(&flagSort, "sort", "year", "Sort filmography by: year, rating, or popularity")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Limit number of entries shown (0 = all)")

	return cmd
}
