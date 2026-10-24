// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSearchStoreCmd(flags))
	})
}

// searchRow is one ranked store-search result. Rank is the position in
// Apple's result list before any --price filter.
type searchRow struct {
	Rank          int      `json:"rank"`
	AppID         string   `json:"appId"`
	Title         string   `json:"title"`
	DeveloperName string   `json:"developerName,omitempty"`
	Genre         string   `json:"genre,omitempty"`
	Price         string   `json:"price,omitempty"`
	AverageRating *float64 `json:"averageRating,omitempty"`
	RatingCount   *int64   `json:"ratingCount,omitempty"`
	BundleID      string   `json:"bundleId,omitempty"`
	Source        string   `json:"source"`
}

type searchMeta struct {
	Term          string         `json:"term"`
	Storefront    string         `json:"storefront"`
	Platform      string         `json:"platform"`
	Surface       string         `json:"surface"`
	PriceFilter   string         `json:"priceFilter,omitempty"`
	Scanned       int            `json:"scannedResults"`
	ResultCount   int            `json:"resultCount"`
	Returned      int            `json:"returned"`
	CapturedAt    string         `json:"capturedAt,omitempty"`
	FromCache     bool           `json:"fromCache"`
	Recorded      bool           `json:"recorded"`
	Notes         []string       `json:"notes,omitempty"`
	FetchFailures []fetchFailure `json:"fetch_failures,omitempty"`
}

// searchOptions are the validated inputs of one search-store run.
type searchOptions struct {
	Term     string
	Country  string
	Platform string
	Entity   string
	Price    string // "", free, paid
	Limit    int
	Source   string // itunes | web
}

func parseSearchPrice(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "any", "all":
		return "", nil
	case "free":
		return "free", nil
	case "paid":
		return "paid", nil
	}
	return "", usageErr(fmt.Errorf("--price: unknown value %q; use free or paid (omit for both)", raw))
}

func priceMatches(filter string, price float64) bool {
	switch filter {
	case "free":
		return price == 0
	case "paid":
		return price > 0
	}
	return true
}

func newSearchStoreCmd(flags *rootFlags) *cobra.Command {
	var platform, price, source string
	var limit int
	cmd := &cobra.Command{
		Use:   "search-store <term>",
		Short: "Search the App Store for apps by keyword, ranked, with a source label on every row",
		Long: strings.Trim(`
Search one storefront (--country) for apps matching a keyword and return them
in Apple's order. Every row carries its source:

  itunes (default)  the documented iTunes Search API (paced at 20 requests a
                    minute). Its order differs from the on-device App Store.
  web (opt-in)      the apps.apple.com search page, closer to the on-device
                    order. robots.txt disallows this page, so it is used only
                    when you pass --source web explicitly; never by default.

--price free|paid filters Apple's first 200 results locally; rank stays the
position in Apple's unfiltered list. Each search is recorded in the local
keyword_ranks table (no target app) for keyword-history and keyword-share; a
result served from the local cache is not recorded twice.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli search-store "photo editor" --limit 10 --json
  app-store-pp-cli search-store spotify --platform ipad --country gb
  app-store-pp-cli search-store "sudoku" --price paid --limit 20 --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			// Any text is a valid search term: a nonsense term is an empty or
			// broad search (exit 0), not an invalid argument.
			"pp:no-error-path-probe": "true",
			"pp:happy-args":          "term=spotify",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search-store")
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required, e.g. search-store \"photo editor\""))
			}
			o := searchOptions{Term: term}
			var err error
			if o.Platform, o.Entity, err = s2bSearchPlatform(platform); err != nil {
				return err
			}
			if o.Price, err = parseSearchPrice(price); err != nil {
				return err
			}
			if limit < 1 || limit > appstore.SearchMaxLimit {
				return usageErr(fmt.Errorf("--limit must be between 1 and %d (got %d)", appstore.SearchMaxLimit, limit))
			}
			o.Limit = limit
			if o.Source, err = s2bSearchSource(source); err != nil {
				return err
			}
			if o.Country, err = appStoreCountry(flags); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, st, err := s2bOpenLive(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			if o.Source == store.SourceWebSearch {
				return runSearchWeb(ctx, cmd, flags, c, st, o)
			}
			return runSearchITunes(ctx, cmd, flags, c, st, o)
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "iphone", "Store to search: iphone, ipad or mac")
	cmd.Flags().StringVar(&price, "price", "", "Keep only free or paid apps (filters Apple's first 200 results)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Rows to return (1-200)")
	cmd.Flags().StringVar(&source, "source", "itunes", "Result source: itunes (Search API, default) or web (apps.apple.com search page; robots-disallowed, used only on explicit request)")
	s2bDBFlag(cmd)
	return cmd
}

func newSearchMeta(o searchOptions) searchMeta {
	return searchMeta{Term: o.Term, Storefront: o.Country, Platform: o.Platform, Surface: o.Source, PriceFilter: o.Price, Notes: make([]string, 0)}
}

// searchRequestSize is how many results to ask Apple for: --limit, or the
// maximum when a price filter has to scan past non-matching rows.
func searchRequestSize(o searchOptions) int {
	if o.Price != "" {
		return appstore.SearchMaxLimit
	}
	return o.Limit
}

func runSearchITunes(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, o searchOptions) error {
	meta := newSearchMeta(o)
	resp, err := c.Search(ctx, appstore.SearchQuery{Term: o.Term, Country: o.Country, Entity: o.Entity, Limit: searchRequestSize(o)})
	if err != nil {
		return classifyAppStoreErr(err)
	}
	fetchedAt := s2bNowIfZero(resp.Meta.FetchedAt)
	meta.CapturedAt, meta.FromCache = s2bTime(fetchedAt), resp.Meta.FromCache
	meta.Scanned, meta.ResultCount = len(resp.Results), len(resp.Results)
	rows := make([]searchRow, 0)
	ids := make([]string, 0, len(resp.Results))
	for i, r := range resp.Results {
		ids = append(ids, r.AppID())
		if len(rows) >= o.Limit || !priceMatches(o.Price, r.Price) {
			continue
		}
		rows = append(rows, searchRowFromLookup(i+1, r, o.Source))
	}
	if meta.Recorded, err = s2bRecordKeywordRank(s2bWriteCtx(ctx), st, store.KeywordRank{
		CapturedAt: fetchedAt, Storefront: o.Country, Platform: o.Platform, Term: o.Term,
		Source: o.Source, ResultCount: len(resp.Results), TopIDs: ids,
	}); err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the search capture", err)
	}
	return finishSearch(cmd, flags, meta, rows, o)
}

func runSearchWeb(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, o searchOptions) error {
	meta := newSearchMeta(o)
	c.EnableOptInSurfaces()
	page, err := c.WebSearch(ctx, o.Country, o.Platform, o.Term)
	if err != nil {
		return classifyAppStoreErr(err)
	}
	// The observation time is when Apple served the page (a page from the
	// local cache keeps its original time), never the wall clock.
	fetchedAt := s2bNowIfZero(page.FetchedAt)
	meta.CapturedAt, meta.FromCache = s2bTime(fetchedAt), page.FromCache
	ids := page.RankedIDs()
	n := searchRequestSize(o)
	if n > len(ids) {
		n = len(ids)
	}
	meta.Scanned, meta.ResultCount = n, len(ids)
	lockups := make(map[string]appstore.AppLockup, len(page.Lockups))
	for _, l := range page.Lockups {
		lockups[l.AdamID] = l
	}
	h, herr := s2bHydrate(ctx, c, o.Country, ids[:n])
	if herr != nil {
		return classifyAppStoreErr(herr)
	}
	rows := make([]searchRow, 0)
	for i, id := range ids[:n] {
		if len(rows) >= o.Limit {
			break
		}
		row := searchRow{Rank: i + 1, AppID: id, Source: o.Source}
		lk, hydrated := h.ByID[id]
		if hydrated {
			if !priceMatches(o.Price, lk.Price) {
				continue
			}
			row = searchRowFromLookup(i+1, lk, o.Source)
		} else if o.Price != "" {
			continue // price unknown: never guess a filter match
		}
		if l, ok := lockups[id]; ok && row.Title == "" {
			row.Title, row.DeveloperName, row.BundleID = l.Title, l.DeveloperName, l.BundleID
		}
		rows = append(rows, row)
	}
	if len(h.Failures) > 0 {
		meta.FetchFailures = h.Failures
		warnFetchFailures(cmd.ErrOrStderr(), len(h.Failures), len(h.Failures)+len(h.Batches), "Lookup batches")
	}
	if meta.Recorded, err = s2bRecordKeywordRank(s2bWriteCtx(ctx), st, store.KeywordRank{
		CapturedAt: fetchedAt, Storefront: o.Country, Platform: o.Platform, Term: o.Term,
		Source: o.Source, ResultCount: len(ids), TopIDs: ids,
	}); err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the search capture", err)
	} else if !meta.Recorded {
		meta.Notes = append(meta.Notes, "search page served from the local cache (fetched "+meta.CapturedAt+"); this capture is already stored (pass --no-cache for a fresh capture)")
	}
	return finishSearch(cmd, flags, meta, rows, o)
}

func searchRowFromLookup(rank int, r appstore.LookupResult, source string) searchRow {
	return searchRow{
		Rank:          rank,
		AppID:         r.AppID(),
		Title:         r.TrackName,
		DeveloperName: r.ArtistName,
		Genre:         r.PrimaryGenreName,
		Price:         r.FormattedPrice,
		AverageRating: s2bFloat(r.AverageUserRating),
		RatingCount:   s2bInt(r.UserRatingCount),
		BundleID:      r.BundleID,
		Source:        source,
	}
}

func finishSearch(cmd *cobra.Command, flags *rootFlags, meta searchMeta, rows []searchRow, o searchOptions) error {
	meta.Returned = len(rows)
	switch {
	case meta.Scanned == 0:
		meta.Notes = append(meta.Notes, fmt.Sprintf("Apple returned no apps for %q in storefront %s (verified empty)", o.Term, o.Country))
	case len(rows) == 0 && o.Price != "":
		meta.Notes = append(meta.Notes, fmt.Sprintf("scanned the first %d results Apple returned for %q; none is %s", meta.Scanned, o.Term, o.Price))
	case len(rows) < o.Limit && o.Price != "":
		meta.Notes = append(meta.Notes, fmt.Sprintf("only %d of the first %d results are %s", len(rows), meta.Scanned, o.Price))
	}
	return s2bPrint(cmd, flags, meta, rows, meta.Notes)
}
