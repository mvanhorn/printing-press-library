package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
)

// queueCandidate is one row in the next-watch queue.
type queueCandidate struct {
	ID          int           `json:"id"`
	Kind        string        `json:"kind"`
	Title       string        `json:"title"`
	Year        string        `json:"year"`
	VoteAverage float64       `json:"vote_average"`
	VoteCount   int           `json:"vote_count"`
	Score       float64       `json:"score"`
	Sources     []string      `json:"sources"` // "recommendation" | "similar"
	Providers   []tonightProv `json:"providers,omitempty"`
}

type queueOutput struct {
	Queue []queueCandidate `json:"queue"`
}

func newQueueCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagProviders string
	var flagRegion string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "queue",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Suggest next-watch picks derived from your watchlist's recommendations and similars",
		Long: `For each watchlist row, fetch /<kind>/{id}/recommendations and /<kind>/{id}/similar,
union the results, drop anything already on your watchlist, optionally filter
by streaming providers, and rank by vote_average × log(vote_count + 1).

Useful when you've finished one row of your watchlist and want to know what
the algorithm thinks you'd watch next based on the rest of the list.`,
		Example: `  movie-goat-pp-cli queue
  movie-goat-pp-cli queue --limit 10
  movie-goat-pp-cli queue --providers "Netflix" --region US --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}
			s, err := openStoreForWatchlist(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.WatchlistList(cmd.Context())
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				out := queueOutput{Queue: []queueCandidate{}}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty — nothing to derive a queue from. Add titles with `watchlist add`.")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Mark watchlist members for exclusion.
			inWatchlist := map[string]bool{}
			for _, e := range entries {
				inWatchlist[fmt.Sprintf("%s-%d", e.Kind, e.TMDBID)] = true
			}

			// 2. For each row, fetch recommendations + similar (top 10 each).
			type fetchSource struct {
				watchlistID   int
				watchlistKind string
			}
			sources := make([]fetchSource, 0, len(entries))
			for _, e := range entries {
				sources = append(sources, fetchSource{watchlistID: e.TMDBID, watchlistKind: e.Kind})
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()

			type relRow struct {
				ID           int     `json:"id"`
				Title        string  `json:"title"`
				Name         string  `json:"name"`
				ReleaseDate  string  `json:"release_date"`
				FirstAirDate string  `json:"first_air_date"`
				VoteAverage  float64 `json:"vote_average"`
				VoteCount    int     `json:"vote_count"`
			}
			type relPayload struct {
				Results []relRow `json:"results"`
			}
			type harvest struct {
				kind string
				recs []relRow
				sims []relRow
			}
			results, errs := cliutil.FanoutRun(ctx, sources,
				func(s fetchSource) string { return fmt.Sprintf("%s/%d", s.watchlistKind, s.watchlistID) },
				func(_ context.Context, src fetchSource) (harvest, error) {
					h := harvest{kind: src.watchlistKind}
					for _, suffix := range []string{"recommendations", "similar"} {
						path := fmt.Sprintf("/%s/%d/%s", src.watchlistKind, src.watchlistID, suffix)
						data, err := c.Get(path, map[string]string{})
						if err != nil {
							return h, err
						}
						var p relPayload
						if jerr := json.Unmarshal(data, &p); jerr != nil {
							return h, jerr
						}
						rows := p.Results
						if len(rows) > 10 {
							rows = rows[:10]
						}
						if suffix == "recommendations" {
							h.recs = append(h.recs, rows...)
						} else {
							h.sims = append(h.sims, rows...)
						}
					}
					return h, nil
				})
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			// 3. Aggregate + dedupe by (kind, id).
			byKey := map[string]*queueCandidate{}
			addRow := func(kind string, r relRow, source string) {
				key := fmt.Sprintf("%s-%d", kind, r.ID)
				if inWatchlist[key] {
					return
				}
				cand, ok := byKey[key]
				if !ok {
					cand = &queueCandidate{
						ID:          r.ID,
						Kind:        kind,
						Title:       firstNonEmpty(r.Title, r.Name),
						VoteAverage: r.VoteAverage,
						VoteCount:   r.VoteCount,
					}
					switch {
					case len(r.ReleaseDate) >= 4:
						cand.Year = r.ReleaseDate[:4]
					case len(r.FirstAirDate) >= 4:
						cand.Year = r.FirstAirDate[:4]
					}
					byKey[key] = cand
				}
				if !containsString(cand.Sources, source) {
					cand.Sources = append(cand.Sources, source)
				}
			}
			for _, h := range results {
				for _, r := range h.Value.recs {
					addRow(h.Value.kind, r, "recommendation")
				}
				for _, r := range h.Value.sims {
					addRow(h.Value.kind, r, "similar")
				}
			}

			candidates := make([]queueCandidate, 0, len(byKey))
			for _, v := range byKey {
				v.Score = v.VoteAverage * math.Log1p(float64(v.VoteCount))
				candidates = append(candidates, *v)
			}

			// 4. Optional providers filter.
			if flagProviders != "" && len(candidates) > 0 {
				region := strings.ToUpper(strings.TrimSpace(flagRegion))
				if region == "" {
					region = "US"
				}
				want := parseProviderList(flagProviders)
				type provHit struct {
					id    int
					kind  string
					provs []tonightProv
					match bool
				}
				ctx2, cancel2 := context.WithTimeout(cmd.Context(), 90*time.Second)
				defer cancel2()
				apiClient := c
				results, errs := cliutil.FanoutRun(ctx2, candidates,
					func(qc queueCandidate) string { return qc.Title },
					func(_ context.Context, qc queueCandidate) (provHit, error) {
						provs, match, perr := fetchProviderInfo(apiClient, qc.Kind, qc.ID, region, want)
						return provHit{id: qc.ID, kind: qc.Kind, provs: provs, match: match}, perr
					})
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				byID := make(map[string]provHit, len(results))
				for _, r := range results {
					byID[fmt.Sprintf("%s-%d", r.Value.kind, r.Value.id)] = r.Value
				}
				kept := candidates[:0]
				for _, qc := range candidates {
					hit, ok := byID[fmt.Sprintf("%s-%d", qc.Kind, qc.ID)]
					if !ok || !hit.match {
						continue
					}
					qc.Providers = hit.provs
					kept = append(kept, qc)
				}
				candidates = kept
			}

			// 5. Sort by score desc, take top N.
			sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
			if len(candidates) > limit {
				candidates = candidates[:limit]
			}

			// Stable Sources order for deterministic JSON.
			for i := range candidates {
				sort.Strings(candidates[i].Sources)
			}

			out := queueOutput{Queue: candidates}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			if len(candidates) == 0 {
				fmt.Fprintln(w, "Queue empty. Try removing --providers, expanding --limit, or adding more titles to your watchlist.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "Title\tYear\tKind\tRating\tVotes\tScore\tSources")
			for _, c := range candidates {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%d\t%.1f\t%s\n",
					truncate(c.Title, 40), orDash(c.Year), c.Kind,
					c.VoteAverage, c.VoteCount, c.Score,
					strings.Join(c.Sources, ","))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Top N candidates to return")
	cmd.Flags().StringVar(&flagProviders, "providers", "", "Comma-separated provider names to require")
	cmd.Flags().StringVar(&flagRegion, "region", "US", "Region for /watch/providers lookup")
	cmd.Flags().StringVar(&flagDB, "db", "", "Override SQLite path")
	return cmd
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
