// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newRatingsCmd(flags))
	})
}

// ratingsRow is one storefront's ratings. averageRating and ratingCount are
// the exact Lookup values; histogram (5-star first) and histogramTotal come
// from the product page with --histogram (cached separately, so its total
// can differ slightly from ratingCount). fetchedAt is when Apple served the
// Lookup row.
type ratingsRow struct {
	Storefront            string  `json:"storefront"`
	AverageRating         float64 `json:"averageRating"`
	RatingCount           int64   `json:"ratingCount"`
	CurrentVersionAverage float64 `json:"currentVersionAverage"`
	CurrentVersionCount   int64   `json:"currentVersionCount"`
	Version               string  `json:"version"`
	Histogram             []int64 `json:"histogram,omitempty"`
	HistogramTotal        int64   `json:"histogramTotal,omitempty"`
	FetchedAt             string  `json:"fetchedAt"`
}

type ratingsMeta struct {
	App         string   `json:"app"`
	Title       string   `json:"title"`
	Storefronts []string `json:"storefronts"`
	Histogram   bool     `json:"histogram"`
	Unavailable []string `json:"unavailable"`
	Requests    int      `json:"requests"`
	Notes       []string `json:"notes"`
}

func newRatingsCmd(flags *rootFlags) *cobra.Command {
	var countries string
	var histogram bool
	cmd := &cobra.Command{
		Use:   "ratings <app>",
		Short: "Fetch exact rating counts and averages live per storefront, optional star histogram",
		Long: strings.Trim(`
Show an app's ratings per storefront: the exact rating count and average from
the iTunes Lookup API (one request per storefront), the current version's
count and average, and with --histogram the 5-star-first star histogram from
each storefront's product page (one more request each; pages are paced about
20 s apart, across runs, so raise --timeout for more than two storefronts with
--histogram). Every run appends a
ratings snapshot per storefront (source lookup, stamped with Apple's fetch
time, so a cached answer is never recorded twice); traction and impact read
that series.

JSON is {meta, items, fetch_failures}: one item per storefront where the app
is sold; meta lists storefronts where it is not sold. Failed storefronts are
warned on stderr, listed in fetch_failures and never turned into zero rows.
Not sold in any requested storefront exits 3.

Use this for ratings across storefronts; 'app' for one storefront's full
listing, 'compare' for several apps side by side, 'traction' for new ratings
over time.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli ratings 324684580 --json
  app-store-pp-cli ratings 324684580 --countries us,gb,de,jp --histogram --select items.storefront,items.ratingCount,items.histogram`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ratings")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			list, err := s2aStorefronts(cmd, flags, countries, &ref)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newAppStoreClient(flags)
			if err != nil {
				return err
			}
			st, err := openAppStoreStore(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			attachCaptureLog(c, st)

			meta := ratingsMeta{
				App:         s2aRefLabel(ref),
				Storefronts: list,
				Histogram:   histogram,
				Unavailable: make([]string, 0),
				Notes:       make([]string, 0),
			}
			rows := make([]ratingsRow, 0, len(list))
			failures := make([]fetchFailure, 0)
			errs := make([]error, 0)
			// stopped is the throttle or pacing refusal that ended the fan-out
			// early: the rows gathered so far are still printed, then exit 7.
			var stopped error
			for i, cc := range list {
				lk, fetchedAt, found, err := s2aLookupIn(ctx, c, ref, cc)
				if err != nil {
					failures = append(failures, fetchFailure{ID: meta.App, Storefront: cc, Error: err.Error()})
					if isRateLimited(err) || ctx.Err() != nil {
						stopped = err
						failures = append(failures, s2aNotAttempted(meta.App, list[i+1:], cc)...)
						break
					}
					errs = append(errs, err)
					continue
				}
				if !found {
					meta.Unavailable = append(meta.Unavailable, cc)
					continue
				}
				var page *appstore.ProductPage
				if histogram {
					var perr error
					if page, perr = s2aProductPage(ctx, c, lk, cc, ""); perr != nil {
						// The Lookup counts stand; only this storefront's
						// histogram is missing.
						failures = append(failures, fetchFailure{ID: lk.AppID(), Storefront: cc, Error: "histogram: " + perr.Error()})
						if isRateLimited(perr) || ctx.Err() != nil {
							stopped = perr
							failures = append(failures, s2aNotAttempted(lk.AppID(), list[i+1:], cc)...)
						}
					}
				}
				meta.App = lk.AppID()
				if meta.Title == "" {
					meta.Title = lk.TrackName
				}
				s2aWarnStore(cmd, "the "+cc+" ratings snapshot", s2aRecordLookupRatings(ctx, st, cc, lk, fetchedAt))
				row := ratingsRow{
					Storefront:            cc,
					AverageRating:         lk.AverageUserRating,
					RatingCount:           lk.UserRatingCount,
					CurrentVersionAverage: lk.AverageUserRatingForCurrentVersion,
					CurrentVersionCount:   lk.UserRatingCountForCurrentVersion,
					Version:               lk.Version,
					FetchedAt:             appstore.FormatTime(fetchedAt),
				}
				if page != nil {
					s2aWarnStore(cmd, "the "+cc+" listing snapshot", s2aRecordProductPage(ctx, cmd, st, cc, page, lk))
					row.Histogram = append(make([]int64, 0, 5), page.Histogram...)
					row.HistogramTotal = page.TotalRatings
					if len(page.Histogram) == 0 {
						meta.Notes = append(meta.Notes, fmt.Sprintf("%s: the product page shows no star histogram (too few ratings)", cc))
					}
				}
				rows = append(rows, row)
				if stopped != nil {
					break
				}
			}
			meta.Requests = c.Requests()
			if stopped != nil && len(rows) == 0 {
				return s2aStopped(stopped)
			}
			if err := s2aFanoutOutcome(len(list), len(rows), errs, func() error {
				return notFoundErr(fmt.Errorf("no App Store app %s in storefront(s) %s (check the id, or try other --countries)", s2aRefLabel(ref), strings.Join(list, ",")))
			}); err != nil {
				return err
			}
			if len(meta.Unavailable) > 0 {
				meta.Notes = append(meta.Notes, fmt.Sprintf("not sold in: %s", strings.Join(meta.Unavailable, ",")))
			}
			warnFetchFailures(cmd.ErrOrStderr(), len(failures), len(list), "storefronts")
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range meta.Notes {
					s2aPrintln(cmd.ErrOrStderr(), "note: "+n)
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), s2aEnvelope{Meta: meta, Items: rows, FetchFailures: failures}, flags); err != nil {
					return err
				}
				return s2aStopped(stopped)
			}
			w := cmd.OutOrStdout()
			s2aPrintf(w, "%s (%s) ratings\n", cliutil.ScrubTerminal(meta.Title), meta.App)
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				hist := ""
				if len(r.Histogram) == 5 {
					parts := make([]string, 0, 5)
					for _, n := range r.Histogram {
						parts = append(parts, s2aCount(n))
					}
					hist = strings.Join(parts, " / ")
				}
				table = append(table, []string{r.Storefront, s2aRating(r.AverageRating), s2aCount(r.RatingCount), r.Version, hist})
			}
			if err := s2aTable(w, []string{"storefront", "average", "ratings", "version", "5★ / 4★ / 3★ / 2★ / 1★"}, table); err != nil {
				return err
			}
			for _, n := range meta.Notes {
				s2aPrintln(w, n)
			}
			s2aFailuresHuman(w, failures)
			return s2aStopped(stopped)
		},
	}
	cmd.Flags().StringVar(&countries, "countries", "", "Storefronts to read, comma-separated (us,gb,jp) or all; empty uses --country")
	cmd.Flags().BoolVar(&histogram, "histogram", false, "Add the 5-star-first histogram from each storefront's product page (one more request each)")
	s2bDBFlag(cmd)
	return cmd
}
