// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newReviewsCmd(flags))
	})
}

// apiReviewsPageSize is what one opt-in /api reviews request returns at most.
const apiReviewsPageSize = 20

// reviewRow is one customer review. Votes exist only in the RSS feed and
// developer responses only in the opt-in /api source; absent means the
// source does not carry the field.
type reviewRow struct {
	ReviewID          string `json:"reviewId"`
	Storefront        string `json:"storefront"`
	Rating            int    `json:"rating"`
	Version           string `json:"version,omitempty"`
	Title             string `json:"title"`
	Body              string `json:"body"`
	Author            string `json:"author"`
	ReviewedAt        string `json:"reviewedAt"`
	VoteSum           *int   `json:"voteSum,omitempty"`
	VoteCount         *int   `json:"voteCount,omitempty"`
	DeveloperResponse string `json:"developerResponse,omitempty"`
}

// reviewPageMeta is the outcome of one fetched page.
type reviewPageMeta struct {
	Storefront string `json:"storefront"`
	Sort       string `json:"sort"`
	Page       int    `json:"page"`
	Offset     *int   `json:"offset,omitempty"`
	Outcome    string `json:"outcome"`
	Entries    int    `json:"entries"`
	Oldest     string `json:"oldest,omitempty"`
	Newest     string `json:"newest,omitempty"`
	FromCache  bool   `json:"fromCache"`
}

type reviewsMeta struct {
	AppID          string            `json:"appId"`
	Title          string            `json:"title,omitempty"`
	Surface        string            `json:"surface"`
	Storefronts    []string          `json:"storefronts"`
	Sorts          []string          `json:"sorts"`
	PagesRequested int               `json:"pagesRequested"`
	Pages          []reviewPageMeta  `json:"pages"`
	Fetched        int               `json:"fetched"`
	Unique         int               `json:"unique"`
	Returned       int               `json:"returned"`
	Stored         store.UpsertStats `json:"stored"`
	Oldest         string            `json:"oldest,omitempty"`
	Newest         string            `json:"newest,omitempty"`
	Notes          []string          `json:"notes,omitempty"`
	FetchFailures  []fetchFailure    `json:"fetch_failures,omitempty"`
}

// reviewsOptions are the validated inputs of one reviews run.
type reviewsOptions struct {
	Ref         appstore.AppRef
	Storefronts []string
	LookupCC    string
	Sorts       []string // recent, helpful
	Pages       int
	Limit       int
	Source      string // rss | api
}

func parseReviewSorts(raw string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "both", "all":
		return []string{"recent", "helpful"}, nil
	case "recent", "mostrecent", "newest":
		return []string{"recent"}, nil
	case "helpful", "mosthelpful":
		return []string{"helpful"}, nil
	}
	return nil, usageErr(fmt.Errorf("--sort: unknown value %q; use recent, helpful or both", raw))
}

func parseReviewSource(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "rss":
		return store.SourceRSS, nil
	case "api":
		return store.SourceAPI, nil
	}
	return "", usageErr(fmt.Errorf("--source: unknown value %q; use rss (default) or api (opt-in)", raw))
}

func parseReviewsOptions(cmd *cobra.Command, flags *rootFlags, args []string, countries, sortRaw, source string, pages, limit int) (reviewsOptions, error) {
	var o reviewsOptions
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		_ = cmd.Usage()
		return o, usageErr(fmt.Errorf("an app is required: a numeric id (6448311069), id6448311069, an App Store URL or a bundle id"))
	}
	if len(args) > 1 {
		return o, usageErr(fmt.Errorf("reviews takes one app per run (got %d); use --countries for several storefronts", len(args)))
	}
	ref, err := parseAppRef(args[0])
	if err != nil {
		return o, err
	}
	o.Ref = ref
	if o.Source, err = parseReviewSource(source); err != nil {
		return o, err
	}
	if o.Source == store.SourceAPI && !flagChanged(cmd, "sort") {
		sortRaw = "recent" // one /api request per page unless both sorts are asked for
	}
	if o.Sorts, err = parseReviewSorts(sortRaw); err != nil {
		return o, err
	}
	if pages < 1 || pages > appstore.ReviewsMaxPage {
		return o, usageErr(fmt.Errorf("--pages must be between 1 and %d (got %d)", appstore.ReviewsMaxPage, pages))
	}
	o.Pages = capForDogfood(pages, dogfoodMaxReviewPages)
	if limit < 0 {
		return o, usageErr(fmt.Errorf("--limit must be 0 (all) or more (got %d)", limit))
	}
	o.Limit = limit
	if o.Storefronts, err = countriesFlag(flags, countries); err != nil {
		return o, err
	}
	if strings.TrimSpace(countries) == "" {
		if o.LookupCC, err = refCountry(cmd, flags, ref); err != nil {
			return o, err
		}
		o.Storefronts = []string{o.LookupCC}
	} else {
		o.LookupCC = o.Storefronts[0]
	}
	return o, nil
}

func newReviewsCmd(flags *rootFlags) *cobra.Command {
	var countries, sortRaw, source string
	var pages, limit int
	cmd := &cobra.Command{
		Use:   "reviews <app>",
		Short: "Fetch an app's customer reviews (most recent and most helpful), deduplicated and stored",
		Long: strings.Trim(`
Fetch customer reviews for one app from Apple's customer-reviews RSS feed:
both sorts (most recent, most helpful) by default, 50 reviews per page, up to
10 pages per sort, for each storefront in --countries. Reviews seen under
both sorts are returned once; every page is stored in the local reviews table
(full-text indexed for search and review-digest), so reruns update rather
than duplicate. meta.pages records each page's outcome (ok or verified-empty)
and date span.

--source api (opt-in) reads the apps.apple.com/api reviews resource instead:
20 reviews per page, newest by default, and it adds developer responses.
robots.txt disallows that surface, so it is used only when you pass
--source api explicitly; never by default.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli reviews 6448311069 --pages 2 --json
  app-store-pp-cli reviews 324684580 --countries us,gb --sort recent --limit 100
  app-store-pp-cli reviews com.spotify.client --sort helpful --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580;--sort=recent;--pages=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reviews")
			}
			o, err := parseReviewsOptions(cmd, flags, args, countries, sortRaw, source, pages, limit)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, st, err := s2bOpenLive(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			return runReviews(ctx, cmd, flags, c, st, o)
		},
	}
	cmd.Flags().StringVar(&countries, "countries", "", "Storefronts to fetch, comma-separated (us,gb,jp) or all; default: --country")
	cmd.Flags().StringVar(&sortRaw, "sort", "both", "recent, helpful or both (default both; --source api defaults to recent)")
	cmd.Flags().IntVar(&pages, "pages", 1, "Pages per sort and storefront, 1-10 (RSS: 50 reviews a page; api: 20)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum reviews to return after merging (0 = all fetched); every fetched page is still stored")
	cmd.Flags().StringVar(&source, "source", "rss", "Review source: rss (default) or api (apps.apple.com/api, adds developer responses; robots-disallowed, used only on explicit request)")
	s2bDBFlag(cmd)
	return cmd
}

// reviewPage is one fetched page normalized across sources.
type reviewPage struct {
	reviews   []appstore.Review
	meta      reviewPageMeta
	storeSort string
	seenAt    time.Time
	more      bool
}

func runReviews(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, o reviewsOptions) error {
	if o.Source == store.SourceAPI {
		c.EnableOptInSurfaces()
	}
	meta := reviewsMeta{
		Surface:        o.Source,
		Storefronts:    o.Storefronts,
		Sorts:          o.Sorts,
		PagesRequested: o.Pages,
		Pages:          make([]reviewPageMeta, 0),
		Notes:          make([]string, 0),
		FetchFailures:  make([]fetchFailure, 0),
	}
	appID := o.Ref.ID
	looked := false
	var lk appstore.LookupResult // set when a Lookup ran; its rating count explains an empty feed
	if appID == "" {
		var err error
		if lk, err = lookupAppRef(ctx, c, o.Ref, o.LookupCC); err != nil {
			return err
		}
		appID, meta.Title, looked = lk.AppID(), lk.TrackName, true
	}
	meta.AppID = appID

	type key struct{ storefront, id string }
	seen := map[key]int{}
	rows := make([]reviewRow, 0)
	wctx := s2bWriteCtx(ctx)
	attempted := 0
	emptyStorefronts := make([]string, 0)
	sortGaps := make([]string, 0) // "us (recent)": a sort served empty while the other sort had reviews
	// stopped: a throttle, pacing refusal or expired deadline ended the run
	// early; the reviews gathered so far still print, then the error's exit.
	storeWarned := false // a failed local write is reported once
	var stopped error
storefronts:
	for si, cc := range o.Storefronts {
		before := len(rows)
		failedHere := false
		var emptyFirst []string // sorts whose first page Apple served empty here
		for _, sortName := range o.Sorts {
			for page := 1; page <= o.Pages; page++ {
				attempted++
				p, err := fetchReviewPage(ctx, c, o.Source, cc, appID, sortName, page)
				if err != nil {
					meta.FetchFailures = append(meta.FetchFailures, fetchFailure{
						ID: fmt.Sprintf("%s %s page %d", appID, sortName, page), Storefront: cc, Error: err.Error(),
					})
					if isRateLimited(err) || ctx.Err() != nil {
						stopped = err
						meta.FetchFailures = append(meta.FetchFailures, s2aNotAttempted(appID, o.Storefronts[si+1:], cc)...)
						break storefronts
					}
					failedHere = true
					break // deeper pages of this sort would fail the same way
				}
				meta.Pages = append(meta.Pages, p.meta)
				if page == 1 && p.meta.Outcome == appstore.OutcomeVerifiedEmpty {
					emptyFirst = append(emptyFirst, sortName)
				}
				meta.Fetched += len(p.reviews)
				stats, err := store.UpsertReviews(wctx, st, store.ReviewUpsert{
					Storefront: cc, AppID: appID, Sort: p.storeSort, Source: o.Source, SeenAt: p.seenAt, Reviews: p.reviews,
				})
				if err != nil {
					if !storeWarned {
						s2bStoreFailed(cmd, &meta.Notes, "the reviews", err)
						storeWarned = true
					}
				}
				meta.Stored.Inserted += stats.Inserted
				meta.Stored.Updated += stats.Updated
				meta.Stored.Unchanged += stats.Unchanged
				for _, r := range p.reviews {
					k := key{cc, r.ID}
					if i, dup := seen[k]; dup {
						if rows[i].DeveloperResponse == "" && r.DeveloperResponse != "" {
							rows[i].DeveloperResponse = r.DeveloperResponse
						}
						continue
					}
					seen[k] = len(rows)
					rows = append(rows, reviewRowFrom(cc, o.Source, r))
				}
				if !p.more {
					break
				}
			}
		}
		if len(o.Sorts) > 1 {
			part := rows[before:]
			sort.SliceStable(part, func(i, j int) bool { return part[i].ReviewedAt > part[j].ReviewedAt })
		}
		if len(rows) == before && !failedHere {
			emptyStorefronts = append(emptyStorefronts, cc)
		} else if len(rows) > before {
			for _, s := range emptyFirst {
				sortGaps = append(sortGaps, fmt.Sprintf("%s (%s)", cc, s))
			}
		}
	}
	if stopped != nil && len(meta.Pages) == 0 {
		return classifyAppStoreErr(stopped)
	}
	if len(meta.FetchFailures) > 0 {
		if len(meta.Pages) == 0 {
			f := meta.FetchFailures[0]
			return apiErr(fmt.Errorf("every review fetch failed (%d of %d); first: %s in %s: %s", len(meta.FetchFailures), attempted, f.ID, f.Storefront, f.Error))
		}
		warnFetchFailures(cmd.ErrOrStderr(), len(meta.FetchFailures), attempted, "review pages")
	}
	meta.Unique = len(rows)
	if len(rows) == 0 && len(meta.FetchFailures) == 0 && !looked {
		// No reviews anywhere: tell a missing app (exit 3) from an app
		// without written reviews.
		var err error
		if lk, err = lookupAppRef(ctx, c, o.Ref, o.LookupCC); err != nil {
			return err
		}
		meta.Title = lk.TrackName
	}
	if len(emptyStorefronts) > 0 {
		meta.Notes = append(meta.Notes, emptyReviewsNote(appID, emptyStorefronts, o, lk))
	}
	if len(sortGaps) > 0 {
		meta.Notes = append(meta.Notes, fmt.Sprintf("Apple served an empty first page for app %s in %s while the other sort had reviews; Apple's reviews feeds are sometimes empty for a while, so run it again later for the reviews that sort adds", appID, strings.Join(sortGaps, ", ")))
	}
	for _, r := range rows {
		if r.ReviewedAt == "" {
			continue
		}
		if meta.Oldest == "" || r.ReviewedAt < meta.Oldest {
			meta.Oldest = r.ReviewedAt
		}
		if meta.Newest == "" || r.ReviewedAt > meta.Newest {
			meta.Newest = r.ReviewedAt
		}
	}
	if o.Limit > 0 && len(rows) > o.Limit {
		rows = rows[:o.Limit]
		meta.Notes = append(meta.Notes, fmt.Sprintf("returned %d of %d reviews (--limit); all %d were stored", o.Limit, meta.Unique, meta.Unique))
	}
	meta.Returned = len(rows)
	if len(meta.FetchFailures) == 0 {
		meta.FetchFailures = nil
	}
	if err := s2bPrint(cmd, flags, meta, rows, meta.Notes); err != nil {
		return err
	}
	return s2aStopped(stopped)
}

// emptyReviewsNote explains storefronts whose feeds returned no reviews. When
// a Lookup ran in one of them and counts ratings there, the app is sold and
// rated in that storefront, so the empty feed is Apple's (on 2026-10-24
// Spotify's most-recent feed in us came back empty while its most-helpful
// feed still served 50 reviews).
func emptyReviewsNote(appID string, empty []string, o reviewsOptions, lk appstore.LookupResult) string {
	where := strings.Join(empty, ", ")
	if lk.UserRatingCount > 0 && slices.Contains(empty, o.LookupCC) {
		hint := ""
		if len(o.Sorts) == 1 {
			hint = " or try --sort both"
		}
		return fmt.Sprintf("Apple returned no reviews for app %s in %s (verified empty), but Lookup counts %d ratings in %s: Apple's reviews feeds are sometimes empty for a while, so run it again later%s", appID, where, lk.UserRatingCount, o.LookupCC, hint)
	}
	return fmt.Sprintf("Apple returned no reviews for app %s in %s (verified empty: no written reviews in this feed, or the app is not sold there)", appID, where)
}

// fetchReviewPage fetches one page from the RSS feed or the opt-in /api.
func fetchReviewPage(ctx context.Context, c *appstore.Client, source, cc, appID, sortName string, page int) (reviewPage, error) {
	if source == store.SourceAPI {
		offset := (page - 1) * apiReviewsPageSize
		p, err := c.APIReviews(ctx, cc, appID, offset, apiReviewsPageSize, sortName)
		if err != nil {
			return reviewPage{}, err
		}
		outcome := appstore.OutcomeOK
		if len(p.Reviews) == 0 {
			outcome = appstore.OutcomeVerifiedEmpty
		}
		oldest, newest := reviewDateSpan(p.Reviews)
		off := offset
		return reviewPage{
			reviews:   p.Reviews,
			storeSort: sortName,
			// The observation time is when Apple served the page (kept by
			// the cache), never the wall clock.
			seenAt: s2bNowIfZero(p.FetchedAt),
			more:   len(p.Reviews) >= apiReviewsPageSize && p.Next != "",
			meta: reviewPageMeta{
				Storefront: cc, Sort: sortName, Page: page, Offset: &off, Outcome: outcome,
				Entries: len(p.Reviews), Oldest: oldest, Newest: newest, FromCache: p.FromCache,
			},
		}, nil
	}
	feed, err := c.ReviewsRSS(ctx, cc, appID, page, sortName)
	if err != nil {
		return reviewPage{}, err
	}
	return reviewPage{
		reviews:   feed.Reviews,
		storeSort: feed.Sort,
		seenAt:    s2bNowIfZero(feed.Meta.FetchedAt),
		more:      len(feed.Reviews) >= 50,
		meta: reviewPageMeta{
			Storefront: cc, Sort: sortName, Page: page, Outcome: feed.Outcome,
			Entries: len(feed.Reviews), Oldest: feed.Oldest, Newest: feed.Newest, FromCache: feed.Meta.FromCache,
		},
	}, nil
}

func reviewRowFrom(cc, source string, r appstore.Review) reviewRow {
	row := reviewRow{
		ReviewID:          r.ID,
		Storefront:        cc,
		Rating:            r.Rating,
		Version:           r.Version,
		Title:             r.Title,
		Body:              r.Body,
		Author:            r.Author,
		ReviewedAt:        r.ReviewedAt,
		DeveloperResponse: r.DeveloperResponse,
	}
	if source == store.SourceRSS {
		sum, count := r.VoteSum, r.VoteCount
		row.VoteSum, row.VoteCount = &sum, &count
	}
	return row
}

// reviewDateSpan returns the oldest and newest reviewedAt of a page.
func reviewDateSpan(reviews []appstore.Review) (oldest, newest string) {
	for _, r := range reviews {
		if r.ReviewedAt == "" {
			continue
		}
		if oldest == "" || r.ReviewedAt < oldest {
			oldest = r.ReviewedAt
		}
		if newest == "" || r.ReviewedAt > newest {
			newest = r.ReviewedAt
		}
	}
	return oldest, newest
}
