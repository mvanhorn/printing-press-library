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
		addNovelCommandIfAbsent(root, newTopCmd(flags))
	})
}

// topRow is one ranked chart row. Fields the source did not provide are
// omitted rather than zero: subtitle and editorsChoice exist only for the
// chart page's 25 lockups, ratings only for apps the Lookup API returned.
type topRow struct {
	Rank          int      `json:"rank"`
	AppID         string   `json:"appId"`
	Title         string   `json:"title"`
	Subtitle      string   `json:"subtitle,omitempty"`
	DeveloperName string   `json:"developerName,omitempty"`
	Genre         string   `json:"genre,omitempty"`
	Price         string   `json:"price,omitempty"`
	AverageRating *float64 `json:"averageRating,omitempty"`
	RatingCount   *int64   `json:"ratingCount,omitempty"`
	EditorsChoice *bool    `json:"editorsChoice,omitempty"`
}

// topMeta describes where a chart came from and what was stored.
type topMeta struct {
	Storefront     string         `json:"storefront"`
	Platform       string         `json:"platform"`
	Chart          string         `json:"chart"`
	GenreID        string         `json:"genreId"`
	Genre          string         `json:"genre"`
	AgeBandID      string         `json:"ageBandId,omitempty"`
	AgeBand        string         `json:"ageBand,omitempty"`
	Surface        string         `json:"surface"`
	SourceURL      string         `json:"sourceUrl"`
	CapturedAt     string         `json:"capturedAt"`
	FromCache      bool           `json:"fromCache"`
	RanksAvailable int            `json:"ranksAvailable"`
	RowLimit       int            `json:"rowLimit"`
	Requested      int            `json:"requested"`
	Returned       int            `json:"returned"`
	Hydrated       int            `json:"hydrated"`
	LookupMissing  []string       `json:"lookupMissing,omitempty"`
	SnapshotKey    store.ChartKey `json:"snapshotKey"`
	SnapshotRows   int            `json:"snapshotRows"`
	RatingsStored  int            `json:"ratingsStored"`
	Notes          []string       `json:"notes,omitempty"`
	FetchFailures  []fetchFailure `json:"fetch_failures,omitempty"`
}

// topOptions are the validated inputs of one top run.
type topOptions struct {
	Country  string
	Chart    string
	Platform string
	Genre    appstore.Genre
	Limit    int
	AgeBand  *appstore.KidsAgeBand
}

// ageBandID is the Kids age band of the chart ("" for the overall chart).
func (o topOptions) ageBandID() string {
	if o.AgeBand != nil {
		return o.AgeBand.ID
	}
	return ""
}

// snapshotKey is the chart_snapshots series key a run writes: a Kids chart
// shares genre 36 with the overall chart and is told apart by its age band
// (store.ChartKey.AgeBand), so it never joins the overall genre-36 series.
// movers, rank-history and breakouts read it back with the same --age-band.
func (o topOptions) snapshotKey(source string) store.ChartKey {
	return store.ChartKey{Storefront: o.Country, Platform: o.Platform, GenreID: o.Genre.ID, AgeBand: o.ageBandID(), Chart: o.Chart, Source: source}
}

func parseTopOptions(flags *rootFlags, chart, genre, platform, ageBand string, limit int) (topOptions, error) {
	var o topOptions
	cc, err := appStoreCountry(flags)
	if err != nil {
		return o, err
	}
	o.Country = cc
	c, err := appstore.NormalizeChart(chart)
	if err != nil {
		return o, usageErr(fmt.Errorf("--chart: %w", err))
	}
	o.Chart = c
	if o.Platform, err = parseChartPlatform(platform); err != nil {
		return o, err
	}
	if o.Genre, err = resolveGenre(genre); err != nil {
		return o, err
	}
	if limit < 1 || limit > s2bWebChartRows {
		return o, usageErr(fmt.Errorf("--limit must be between 1 and %d (got %d)", s2bWebChartRows, limit))
	}
	o.Limit = limit
	// An unset or empty --age-band is the overall chart; only an explicit
	// band (all, 5-under, 6-8, 9-11) selects a Kids chart.
	band, err := resolveAgeBand(ageBand)
	if err != nil {
		return o, err
	}
	if err := checkChartAgeBand(band, o.Genre, o.Platform, o.Chart); err != nil {
		return o, err
	}
	if band.ID != "" {
		o.AgeBand = &band
	}
	return o, nil
}

func newTopCmd(flags *rootFlags) *cobra.Command {
	var chart, genre, platform, ageBand string
	var limit int
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show an App Store top chart (free/paid: 200 ranks; grossing: 100) and record a snapshot",
		Long: strings.Trim(`
Fetch one App Store top chart for a storefront (--country), platform and
genre, hydrate the rows with exact rating counts from the iTunes Lookup API,
and record the full chart as a local snapshot for movers, rank-history,
breakouts and traction.

Sources:
  top-free, top-paid   the apps.apple.com chart page (robots-allowed): 200
                       ranks per chart, iphone, ipad and mac. One page plus
                       one Lookup request per 150 rows returned.
  top-grossing         Apple's legacy iTunes RSS feed: at most 100 rows, said
                       so in meta.notes when --limit asks for more. Mac
                       grossing feeds have been empty since 2025.

The snapshot always covers every rank the source returned (200 or 100),
whatever --limit is, so captures stay comparable. A chart served from the
local cache (pages are cached 15 minutes) is stored at its original fetch
time, never as a second capture. A Kids chart (--age-band, genre 36) is its
own series: genre 36 plus the age band (meta.snapshotKey), never mixed into
the overall genre-36 chart; read it back with movers, rank-history or
breakouts --age-band. Without --age-band (or with an empty value) top reads
the overall chart.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli top --chart top-free --genre games --limit 25 --json
  app-store-pp-cli top --chart top-paid --genre 6014 --country pk
  app-store-pp-cli top --chart top-grossing --platform ipad --limit 100 --agent
  app-store-pp-cli top --chart top-free --age-band 6-8 --limit 50 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "top")
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("top takes no positional arguments (got %q); pick the chart with --chart, --genre and --platform", args[0]))
			}
			opts, err := parseTopOptions(flags, chart, genre, platform, ageBand, limit)
			if err != nil {
				return err
			}
			opts.Limit = capForDogfood(opts.Limit, dogfoodMaxChartLimit)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, st, err := s2bOpenLive(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			if opts.Chart == "top-grossing" {
				return runTopGrossing(ctx, cmd, flags, c, st, opts)
			}
			return runTopWeb(ctx, cmd, flags, c, st, opts)
		},
	}
	cmd.Flags().StringVar(&chart, "chart", "top-free", "Chart: top-free or top-paid (web chart page, 200 ranks) or top-grossing (legacy RSS, 100 rows)")
	cmd.Flags().StringVar(&genre, "genre", appstore.AllGenreID, "Genre id or name: 36 All Apps, 6014 Games, puzzle, 'developer tools' (see 'categories list')")
	cmd.Flags().StringVar(&platform, "platform", "iphone", "Chart platform: iphone, ipad or mac")
	cmd.Flags().IntVar(&limit, "limit", 100, "Rows to return, 1-200 (top-grossing returns at most 100); the snapshot always stores every rank")
	cmd.Flags().StringVar(&ageBand, "age-band", "", "Kids chart age band (with --genre 36): all (All Kids), 5-under, 6-8 or 9-11 (ids 0, 10000, 10001, 10002); unset = the overall chart")
	s2bDBFlag(cmd)
	return cmd
}

func newTopMeta(o topOptions, source, sourceURL string) topMeta {
	m := topMeta{
		Storefront:  o.Country,
		Platform:    o.Platform,
		Chart:       o.Chart,
		GenreID:     o.Genre.ID,
		Genre:       o.Genre.Name,
		Surface:     source,
		SourceURL:   sourceURL,
		Requested:   o.Limit,
		SnapshotKey: o.snapshotKey(source),
		Notes:       make([]string, 0),
	}
	if o.AgeBand != nil {
		m.AgeBandID = o.AgeBand.ID
		m.AgeBand = o.AgeBand.Name
	}
	return m
}

// runTopWeb serves top-free and top-paid from the web chart page.
func runTopWeb(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, o topOptions) error {
	ageBandID := ""
	if o.AgeBand != nil {
		ageBandID = o.AgeBand.ID
	}
	pageURL := appstore.ChartsPageURL(o.Country, o.Platform, o.Genre.ID, o.Chart, ageBandID)
	meta := newTopMeta(o, store.SourceWebChart, pageURL)
	meta.RowLimit = s2bWebChartRows
	model, err := c.PageOfKind(ctx, pageURL, appstore.KindChartsPage)
	if err != nil {
		return classifyAppStoreErr(err)
	}
	page, err := appstore.ParseChartsPage(model)
	if err != nil {
		return apiErr(fmt.Errorf("parsing the chart page %s: %w", pageURL, err))
	}
	fetchedAt := s2bNowIfZero(model.FetchedAt)
	meta.CapturedAt = s2bTime(fetchedAt)
	meta.FromCache = model.FromCache
	rows := make([]topRow, 0)
	seg, ok := page.Segment(o.Chart)
	ids := make([]string, 0)
	ranks := make([]int, 0)
	if ok {
		for _, r := range seg.RankedApps() {
			ids = append(ids, r.ID)
			ranks = append(ranks, r.Rank)
		}
	}
	meta.RanksAvailable = len(ids)
	if len(ids) == 0 {
		if ok {
			meta.Notes = append(meta.Notes, "Apple's chart page listed no ranked apps for this chart (verified empty); no snapshot was recorded")
		} else {
			meta.Notes = append(meta.Notes, fmt.Sprintf("the chart page has no %s segment; no snapshot was recorded", o.Chart))
		}
		return s2bPrint(cmd, flags, meta, rows, meta.Notes)
	}
	n := o.Limit
	if n > len(ids) {
		n = len(ids)
	}
	lockups := make(map[string]appstore.AppLockup, len(seg.Lockups))
	for _, l := range seg.Lockups {
		lockups[l.AdamID] = l
	}
	h, herr := s2bHydrate(ctx, c, o.Country, ids[:n])
	for i, id := range ids[:n] {
		rows = append(rows, buildTopRow(ranks[i], id, lockups, h))
	}

	// The snapshot covers every rank on the page, titled where known.
	snapRows := make([]store.ChartRow, 0, len(ids))
	for i, id := range ids {
		r := store.ChartRow{Rank: ranks[i], AppID: id}
		if l, ok := lockups[id]; ok {
			r.Title, r.DeveloperName = l.Title, l.DeveloperName
		}
		if lk, ok := h.ByID[id]; ok {
			if r.Title == "" {
				r.Title = lk.TrackName
			}
			if r.DeveloperName == "" {
				r.DeveloperName = lk.ArtistName
			}
		}
		snapRows = append(snapRows, r)
	}
	wctx := s2bWriteCtx(ctx)
	if err := store.RecordChart(wctx, st, store.ChartSnapshot{
		CapturedAt: fetchedAt,
		Storefront: o.Country,
		Platform:   o.Platform,
		GenreID:    o.Genre.ID,
		AgeBand:    o.ageBandID(),
		Chart:      o.Chart,
		Source:     store.SourceWebChart,
		RowLimit:   s2bWebChartRows,
		Rows:       snapRows,
	}); err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the chart snapshot", err)
	} else {
		meta.SnapshotRows = len(snapRows)
	}
	if meta.RatingsStored, err = s2bRecordLookupRatings(wctx, st, o.Country, h); err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the ratings snapshots", err)
	}
	if herr != nil {
		// The chart and the completed batches are stored and printed; the
		// stop still exits 7.
		meta.Notes = append(meta.Notes, "the Lookup hydration stopped early (rate limited or paced), so rows past the last completed batch have no exact rating counts: "+herr.Error())
	}
	finishTopMeta(cmd, &meta, rows, h, len(ids))
	if model.FromCache {
		meta.Notes = append(meta.Notes, "chart page served from the local cache (fetched "+meta.CapturedAt+"); the snapshot is stored at that time, not as a new capture")
	}
	if err := s2bPrint(cmd, flags, meta, rows, meta.Notes); err != nil {
		return err
	}
	return s2aStopped(herr)
}

// runTopGrossing serves top-grossing from the legacy RSS feed (100 rows max).
func runTopGrossing(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, o topOptions) error {
	feedName, err := appstore.ChartFeedName(o.Chart, o.Platform)
	if err != nil {
		return usageErr(err)
	}
	genreParam := o.Genre.ID
	if genreParam == appstore.AllGenreID {
		genreParam = "" // the overall feed takes no genre segment
	}
	feedURL := appstore.ChartFeedURL(o.Country, feedName, genreParam, s2bRSSChartRows)
	meta := newTopMeta(o, store.SourceRSS, feedURL)
	meta.RowLimit = s2bRSSChartRows
	meta.Notes = append(meta.Notes, fmt.Sprintf("top-grossing comes from Apple's legacy iTunes RSS feed, which returns at most %d rows", s2bRSSChartRows))
	if o.Limit > s2bRSSChartRows {
		meta.Notes = append(meta.Notes, fmt.Sprintf("--limit %d was capped at %d: Apple's grossing feed has no deeper ranks", o.Limit, s2bRSSChartRows))
	}
	feed, err := c.ChartRSS(ctx, o.Country, feedName, genreParam, s2bRSSChartRows)
	if err != nil {
		return classifyAppStoreErr(err)
	}
	fetchedAt := s2bNowIfZero(feed.Meta.FetchedAt)
	meta.CapturedAt = s2bTime(fetchedAt)
	meta.FromCache = feed.Meta.FromCache
	meta.RanksAvailable = len(feed.Entries)
	rows := make([]topRow, 0)
	if len(feed.Entries) == 0 {
		note := "Apple's RSS feed " + feedName + " returned no rows (verified empty); no snapshot was recorded"
		if o.Platform == "mac" {
			note += "; the Mac grossing feeds have been empty since 2025"
		}
		meta.Notes = append(meta.Notes, note)
		return s2bPrint(cmd, flags, meta, rows, meta.Notes)
	}
	n := o.Limit
	if n > len(feed.Entries) {
		n = len(feed.Entries)
	}
	ids := make([]string, 0, n)
	for _, e := range feed.Entries[:n] {
		ids = append(ids, e.AppID)
	}
	h, herr := s2bHydrate(ctx, c, o.Country, ids)
	for _, e := range feed.Entries[:n] {
		row := topRow{Rank: e.Rank, AppID: e.AppID, Title: e.Name, DeveloperName: e.Artist, Genre: e.Genre, Price: rssPrice(e)}
		if lk, ok := h.ByID[e.AppID]; ok {
			applyLookupToTopRow(&row, lk)
		}
		rows = append(rows, row)
	}
	snapRows := make([]store.ChartRow, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		snapRows = append(snapRows, store.ChartRow{Rank: e.Rank, AppID: e.AppID, Title: e.Name, DeveloperName: e.Artist})
	}
	wctx := s2bWriteCtx(ctx)
	if err := store.RecordChart(wctx, st, store.ChartSnapshot{
		CapturedAt: fetchedAt,
		Storefront: o.Country,
		Platform:   o.Platform,
		GenreID:    o.Genre.ID,
		AgeBand:    o.ageBandID(),
		Chart:      o.Chart,
		Source:     store.SourceRSS,
		RowLimit:   s2bRSSChartRows,
		Rows:       snapRows,
	}); err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the chart snapshot", err)
	} else {
		meta.SnapshotRows = len(snapRows)
	}
	if meta.RatingsStored, err = s2bRecordLookupRatings(wctx, st, o.Country, h); err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the ratings snapshots", err)
	}
	if herr != nil {
		// The chart and the completed batches are stored and printed; the
		// stop still exits 7.
		meta.Notes = append(meta.Notes, "the Lookup hydration stopped early (rate limited or paced), so rows past the last completed batch have no exact rating counts: "+herr.Error())
	}
	finishTopMeta(cmd, &meta, rows, h, len(feed.Entries))
	if feed.Meta.FromCache {
		meta.Notes = append(meta.Notes, "feed served from the local cache (fetched "+meta.CapturedAt+"); the snapshot is stored at that time, not as a new capture")
	}
	if err := s2bPrint(cmd, flags, meta, rows, meta.Notes); err != nil {
		return err
	}
	return s2aStopped(herr)
}

// buildTopRow merges a chart-page lockup (first 25 ranks) and the Lookup
// result for one ranked id. Lookup wins for exact counts, genre and price.
func buildTopRow(rank int, id string, lockups map[string]appstore.AppLockup, h *s2bHydration) topRow {
	row := topRow{Rank: rank, AppID: id}
	if l, ok := lockups[id]; ok {
		row.Title = l.Title
		row.Subtitle = l.Subtitle
		row.DeveloperName = l.DeveloperName
		row.EditorsChoice = s2bBool(l.EditorsChoice)
		switch {
		case l.IsFree:
			row.Price = "Free"
		case l.PriceText != "":
			row.Price = l.PriceText
		}
		if l.Rating > 0 {
			row.AverageRating = s2bFloat(l.Rating)
		}
	}
	if h != nil {
		if lk, ok := h.ByID[id]; ok {
			applyLookupToTopRow(&row, lk)
		}
	}
	return row
}

func applyLookupToTopRow(row *topRow, lk appstore.LookupResult) {
	if row.Title == "" {
		row.Title = lk.TrackName
	}
	if row.DeveloperName == "" {
		row.DeveloperName = lk.ArtistName
	}
	if lk.PrimaryGenreName != "" {
		row.Genre = lk.PrimaryGenreName
	}
	if lk.FormattedPrice != "" {
		row.Price = lk.FormattedPrice
	}
	row.AverageRating = s2bFloat(lk.AverageUserRating)
	row.RatingCount = s2bInt(lk.UserRatingCount)
}

// rssPrice turns the RSS price label ("Get" for free apps) into a price.
func rssPrice(e appstore.ChartEntry) string {
	if e.Amount == 0 {
		return "Free"
	}
	return e.Price
}

// finishTopMeta fills the hydration counts and warnings shared by both
// sources.
func finishTopMeta(cmd *cobra.Command, meta *topMeta, rows []topRow, h *s2bHydration, available int) {
	meta.Returned = len(rows)
	for _, r := range rows {
		if r.RatingCount != nil {
			meta.Hydrated++
		}
	}
	if h == nil {
		return
	}
	if len(h.Missing) > 0 {
		meta.LookupMissing = h.Missing
		meta.Notes = append(meta.Notes, fmt.Sprintf("the Lookup API returned nothing for %d charted app(s) in this storefront; their rows carry chart-page fields only", len(h.Missing)))
	}
	if len(h.Failures) > 0 {
		meta.FetchFailures = h.Failures
		total := len(h.Failures) + len(h.Batches)
		warnFetchFailures(cmd.ErrOrStderr(), len(h.Failures), total, "Lookup batches")
	}
	if meta.Surface == store.SourceWebChart && available < meta.Requested {
		meta.Notes = append(meta.Notes, fmt.Sprintf("this chart ranks only %d apps; all of them are returned", available))
	}
}
