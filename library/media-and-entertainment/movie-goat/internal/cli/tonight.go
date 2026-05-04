package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
)

// tonightPick is the row shape emitted by `movie tonight`. Capacity is the
// composite (vote_average × log(popularity+1)) — using log dampens the
// runaway "this week's blockbuster" popularity bump that would otherwise
// drown out high-rated catalog titles.
type tonightPick struct {
	ID          int           `json:"id"`
	Kind        string        `json:"kind"`
	Title       string        `json:"title"`
	Year        string        `json:"year"`
	VoteAverage float64       `json:"vote_average"`
	Popularity  float64       `json:"popularity"`
	Runtime     int           `json:"runtime,omitempty"`
	Providers   []tonightProv `json:"providers,omitempty"`
	Score       float64       `json:"score"`
	GenreIDs    []int         `json:"genre_ids,omitempty"`
}

type tonightProv struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "flatrate" | "rent" | "buy" | "free" | "ads"
}

func newTonightCmd(flags *rootFlags) *cobra.Command {
	var flagMood string
	var flagMaxRuntime int
	var flagProviders string
	var flagRegion string
	var flagType string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "tonight",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Pick what to watch tonight from trending + popular titles",
		Long: `Combines /trending and /popular into one ranked picks list. Optional
filters: --mood (genre name like "thriller" or numeric id), --max-runtime
(drops anything longer), --providers (only keep titles streamable on a
listed provider in --region).

Filtering by --max-runtime or --providers fans out one /movie/{id} or
/movie/{id}/watch/providers call per candidate, so use --limit to bound
work when those flags are set.`,
		Example: `  movie-goat-pp-cli tonight
  movie-goat-pp-cli tonight --mood thriller
  movie-goat-pp-cli tonight --max-runtime 120 --limit 5
  movie-goat-pp-cli tonight --providers "Netflix,Hulu" --region US
  movie-goat-pp-cli tonight --type tv --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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
			limit := flagLimit
			if limit <= 0 {
				limit = 5
			}

			region := strings.ToUpper(strings.TrimSpace(flagRegion))
			if region == "" {
				region = "US"
			}

			// 1. Trending today
			trendingPath := fmt.Sprintf("/trending/%s/day", kind)
			trendingData, err := c.Get(trendingPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}
			var trending tmdbSearchResponse
			if err := json.Unmarshal(trendingData, &trending); err != nil {
				return fmt.Errorf("parsing trending: %w", err)
			}

			// 2. Popular
			popularPath := fmt.Sprintf("/%s/popular", kind)
			popularData, err := c.Get(popularPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}
			var popular tmdbSearchResponse
			if err := json.Unmarshal(popularData, &popular); err != nil {
				return fmt.Errorf("parsing popular: %w", err)
			}

			// 3. Combine + dedupe
			seen := make(map[int]bool)
			picks := make([]tonightPick, 0, len(trending.Results)+len(popular.Results))
			merge := func(src []tmdbSearchResult) {
				for _, r := range src {
					if seen[r.ID] {
						continue
					}
					seen[r.ID] = true
					p := tonightPick{
						ID:          r.ID,
						Kind:        kind,
						Title:       r.DisplayTitle(),
						Year:        r.Year(),
						VoteAverage: r.VoteAverage,
						Popularity:  r.Popularity,
						GenreIDs:    append([]int(nil), r.GenreIDs...),
					}
					p.Score = scoreTonight(r.VoteAverage, r.Popularity)
					picks = append(picks, p)
				}
			}
			merge(trending.Results)
			merge(popular.Results)

			// 4. Mood filter (genre id or genre name).
			if flagMood != "" {
				wantID, err := resolveMoodID(c, kind, flagMood)
				if err != nil {
					return err
				}
				filtered := picks[:0]
				for _, p := range picks {
					for _, g := range p.GenreIDs {
						if g == wantID {
							filtered = append(filtered, p)
							break
						}
					}
				}
				picks = filtered
			}

			// 5. --max-runtime: fan-out per-title detail to fetch runtime.
			if flagMaxRuntime > 0 && kind == "movie" {
				ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
				defer cancel()
				type runtimeRow struct {
					id      int
					runtime int
				}
				results, errs := cliutil.FanoutRun(ctx, picks,
					func(p tonightPick) string { return p.Title },
					func(_ context.Context, p tonightPick) (runtimeRow, error) {
						detail, _, derr := getMovieDetail(c, p.ID, "")
						if derr != nil {
							return runtimeRow{id: p.ID}, derr
						}
						return runtimeRow{id: p.ID, runtime: detail.Runtime}, nil
					})
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				runtimes := make(map[int]int, len(results))
				for _, r := range results {
					runtimes[r.Value.id] = r.Value.runtime
				}
				kept := picks[:0]
				for _, p := range picks {
					rt, ok := runtimes[p.ID]
					if !ok {
						// Could not fetch runtime: drop conservatively.
						continue
					}
					if rt > 0 && rt <= flagMaxRuntime {
						p.Runtime = rt
						kept = append(kept, p)
					}
				}
				picks = kept
			}

			// 6. Providers filter.
			if flagProviders != "" {
				want := parseProviderList(flagProviders)
				ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
				defer cancel()
				type provRow struct {
					id    int
					provs []tonightProv
					match bool
				}
				results, errs := cliutil.FanoutRun(ctx, picks,
					func(p tonightPick) string { return p.Title },
					func(_ context.Context, p tonightPick) (provRow, error) {
						provs, match, ferr := fetchProviderInfo(c, kind, p.ID, region, want)
						return provRow{id: p.ID, provs: provs, match: match}, ferr
					})
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				byID := make(map[int]provRow, len(results))
				for _, r := range results {
					byID[r.Value.id] = r.Value
				}
				kept := picks[:0]
				for _, p := range picks {
					row, ok := byID[p.ID]
					if !ok || !row.match {
						continue
					}
					p.Providers = row.provs
					kept = append(kept, p)
				}
				picks = kept
			}

			// 7. Sort by score desc, take top N.
			sort.SliceStable(picks, func(i, j int) bool { return picks[i].Score > picks[j].Score })
			if len(picks) > limit {
				picks = picks[:limit]
			}

			out := struct {
				Results []tonightPick `json:"results"`
			}{Results: picks}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Tonight's Top Picks")
			fmt.Fprintln(w, "===================")
			if len(picks) == 0 {
				fmt.Fprintln(w, "No picks matched your filters. Try removing --mood, --providers, or --max-runtime.")
				return nil
			}
			for i, p := range picks {
				header := fmt.Sprintf("%d. %s", i+1, p.Title)
				if p.Year != "" {
					header += " (" + p.Year + ")"
				}
				fmt.Fprintln(w, header)
				fmt.Fprintf(w, "   rating: %.1f  popularity: %.0f  score: %.1f\n", p.VoteAverage, p.Popularity, p.Score)
				if p.Runtime > 0 {
					fmt.Fprintf(w, "   runtime: %s\n", formatRuntimeMinutes(p.Runtime))
				}
				if len(p.Providers) > 0 {
					names := make([]string, 0, len(p.Providers))
					for _, pp := range p.Providers {
						names = append(names, fmt.Sprintf("%s (%s)", pp.Name, pp.Kind))
					}
					fmt.Fprintf(w, "   providers: %s\n", strings.Join(names, ", "))
				}
				fmt.Fprintln(w)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagMood, "mood", "", "Filter by genre name (\"thriller\") or numeric TMDb genre id")
	cmd.Flags().IntVar(&flagMaxRuntime, "max-runtime", 0, "Drop picks whose runtime exceeds N minutes (movies only)")
	cmd.Flags().StringVar(&flagProviders, "providers", "", "Comma-separated provider names; keep only titles streamable on one of these")
	cmd.Flags().StringVar(&flagRegion, "region", "US", "Region for --providers lookup (TMDb watch/providers region)")
	cmd.Flags().StringVar(&flagType, "type", "movie", "Media type: movie or tv")
	cmd.Flags().IntVar(&flagLimit, "limit", 5, "Maximum picks to return")
	return cmd
}

// scoreTonight is the composite ranking. Logging popularity dampens the
// "blockbuster of the week" bump so high-vote catalog titles can compete.
func scoreTonight(vote, popularity float64) float64 {
	if popularity < 0 {
		popularity = 0
	}
	return vote * (1 + math.Log1p(popularity))
}

func resolveMoodID(c *client.Client, kind, mood string) (int, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(mood)); err == nil {
		return id, nil
	}
	id, err := resolveGenreIDByName(c, kind, mood)
	if err != nil {
		return 0, usageErr(err)
	}
	return id, nil
}

func parseProviderList(csv string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[strings.ToLower(p)] = struct{}{}
	}
	return out
}

// fetchProviderInfo loads /<kind>/{id}/watch/providers and returns the
// region's provider rows alongside whether any flatrate/rent/buy/free/ads
// provider name matches `want` (case-insensitive). Empty `want` means
// "unconditional pass" — kept here for shared use across tonight + watchlist.
func fetchProviderInfo(c *client.Client, kind string, id int, region string, want map[string]struct{}) ([]tonightProv, bool, error) {
	path := fmt.Sprintf("/%s/%d/watch/providers", kind, id)
	data, err := c.Get(path, map[string]string{})
	if err != nil {
		return nil, false, err
	}
	var resp tmdbWatchProviders
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, fmt.Errorf("parsing watch/providers: %w", err)
	}
	r, ok := resp.Results[region]
	if !ok {
		// No data for this region; treat as no match.
		if len(want) == 0 {
			return nil, true, nil
		}
		return nil, false, nil
	}
	var rows []tonightProv
	for _, p := range r.Flatrate {
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
	if len(want) == 0 {
		return rows, true, nil
	}
	for _, p := range rows {
		if _, hit := want[strings.ToLower(p.Name)]; hit {
			return rows, true, nil
		}
	}
	return rows, false, nil
}
