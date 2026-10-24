// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newKeywordRankCmd(flags))
	})
}

// keywordRankMaxScan is how many results one capture examines: the Search
// API's maximum for a single request.
const keywordRankMaxScan = appstore.SearchMaxLimit

// keywordTopRow is one of the top-N results of a keyword capture.
type keywordTopRow struct {
	Rank          int      `json:"rank"`
	AppID         string   `json:"appId"`
	Title         string   `json:"title"`
	DeveloperName string   `json:"developerName,omitempty"`
	AverageRating *float64 `json:"averageRating,omitempty"`
	RatingCount   *int64   `json:"ratingCount,omitempty"`
	TitleMatch    bool     `json:"titleMatch"`
}

// keywordSignals are transparent difficulty inputs, not a score: how many
// results Apple returned, the top-N rating counts (null where unknown) and
// the share of top-N titles containing every word of the term.
type keywordSignals struct {
	ResultCount     int      `json:"resultCount"`
	TopN            int      `json:"topN"`
	TopRatingCounts []*int64 `json:"topRatingCounts"`
	TitleMatches    int      `json:"titleMatches"`
	TitleMatchShare float64  `json:"titleMatchShare"`
}

type keywordRankApp struct {
	AppID    string `json:"appId,omitempty"`
	BundleID string `json:"bundleId,omitempty"`
	Title    string `json:"title,omitempty"`
}

type keywordRankResult struct {
	Term        string          `json:"term"`
	Storefront  string          `json:"storefront"`
	Platform    string          `json:"platform"`
	Source      string          `json:"source"`
	App         *keywordRankApp `json:"app"`
	Rank        *int            `json:"rank"`
	ResultCount int             `json:"resultCount"`
	Scanned     int             `json:"scanned"`
	Top         []keywordTopRow `json:"top"`
	Signals     keywordSignals  `json:"signals"`
	CapturedAt  string          `json:"capturedAt,omitempty"`
	FromCache   bool            `json:"fromCache"`
	Recorded    bool            `json:"recorded"`
	Notes       []string        `json:"notes,omitempty"`
}

// keywordRankOptions are the validated inputs of one capture.
type keywordRankOptions struct {
	Term     string
	Country  string
	Platform string
	Entity   string
	Source   string
	Target   *appstore.AppRef
	Serp     int
}

// keywordCandidate is one ranked result before formatting.
type keywordCandidate struct {
	ID       string
	BundleID string
	Lookup   *appstore.LookupResult
	Lockup   *appstore.AppLockup
}

func newKeywordRankCmd(flags *rootFlags) *cobra.Command {
	var app, source, platform string
	var serp int
	cmd := &cobra.Command{
		Use:   "keyword-rank <term>",
		Short: "Find where an app ranks in App Store search for a term, and record the data point",
		Long: strings.Trim(`
Run one live store search for a term, report where the target app (--app)
ranks among the first 200 results, and record the capture (rank, result count
and the ranked id list) for keyword-history and keyword-share. rank is null
when the app is not in the scanned results or no --app is given.

signals are transparent keyword-difficulty inputs, never an invented score:
resultCount (results Apple returned, at most 200, not a total match count),
topRatingCounts (rating counts of the --serp top results) and titleMatchShare
(share of those titles containing every word of the term).

Sources: itunes (default) is the documented Search API (one request, paced at
20 a minute; its order differs from the device). web (opt-in) reads the
apps.apple.com search page, closer to the on-device order; robots.txt
disallows that page, so it is used only when you pass --source web.
A capture served from the local cache (Search API answers are cached for up
to 15 minutes) is not recorded twice; pass --no-cache for a fresh data point.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli keyword-rank spotify --app 324684580 --json
  app-store-pp-cli keyword-rank "music streaming" --app com.spotify.client --serp 20 --country gb
  app-store-pp-cli keyword-rank "podcast player" --serp 10 --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			// Any text is a valid search term: a nonsense term is an empty or
			// broad search (exit 0), not an invalid argument.
			"pp:no-error-path-probe": "true",
			"pp:happy-args":          "term=spotify;--app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "keyword-rank")
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required, e.g. keyword-rank spotify --app 324684580"))
			}
			o := keywordRankOptions{Term: term}
			var err error
			if o.Platform, o.Entity, err = s2bSearchPlatform(platform); err != nil {
				return err
			}
			if o.Source, err = s2bSearchSource(source); err != nil {
				return err
			}
			if serp < 1 || serp > keywordRankMaxScan {
				return usageErr(fmt.Errorf("--serp must be between 1 and %d (got %d)", keywordRankMaxScan, serp))
			}
			o.Serp = serp
			if strings.TrimSpace(app) != "" {
				ref, err := parseAppRef(app)
				if err != nil {
					return usageErr(fmt.Errorf("--app: %w", err))
				}
				o.Target = &ref
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
			return runKeywordRank(ctx, cmd, flags, c, st, o)
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Target app to locate: numeric id (324684580), id324684580, App Store URL or bundle id")
	cmd.Flags().StringVar(&source, "source", "itunes", "Result source: itunes (Search API, default) or web (apps.apple.com search page; robots-disallowed, used only on explicit request)")
	cmd.Flags().StringVar(&platform, "platform", "iphone", "Store to search: iphone, ipad or mac")
	cmd.Flags().IntVar(&serp, "serp", 10, "Top results to return and compute signals over (1-200)")
	s2bDBFlag(cmd)
	return cmd
}

func runKeywordRank(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, o keywordRankOptions) error {
	out := keywordRankResult{
		Term: o.Term, Storefront: o.Country, Platform: o.Platform, Source: o.Source,
		Top: make([]keywordTopRow, 0), Notes: make([]string, 0),
		Signals: keywordSignals{TopRatingCounts: make([]*int64, 0)},
	}
	var cands []keywordCandidate
	var capturedAt time.Time
	if o.Source == store.SourceWebSearch {
		c.EnableOptInSurfaces()
		page, err := c.WebSearch(ctx, o.Country, o.Platform, o.Term)
		if err != nil {
			return classifyAppStoreErr(err)
		}
		// Observation time: when Apple served the page (kept by the cache).
		out.FromCache = page.FromCache
		capturedAt = s2bNowIfZero(page.FetchedAt)
		lockups := make(map[string]appstore.AppLockup, len(page.Lockups))
		for _, l := range page.Lockups {
			lockups[l.AdamID] = l
		}
		ids := page.RankedIDs()
		if len(ids) > keywordRankMaxScan {
			ids = ids[:keywordRankMaxScan]
		}
		top := ids
		if len(top) > o.Serp {
			top = top[:o.Serp]
		}
		h, err := s2bHydrate(ctx, c, o.Country, top)
		if err != nil {
			return classifyAppStoreErr(err)
		}
		if len(h.Failures) > 0 {
			warnFetchFailures(cmd.ErrOrStderr(), len(h.Failures), len(h.Failures)+len(h.Batches), "Lookup batches")
		}
		for _, id := range ids {
			cand := keywordCandidate{ID: id}
			if l, ok := lockups[id]; ok {
				l := l
				cand.Lockup, cand.BundleID = &l, l.BundleID
			}
			if lk, ok := h.ByID[id]; ok {
				lk := lk
				cand.Lookup, cand.BundleID = &lk, lk.BundleID
			}
			cands = append(cands, cand)
		}
	} else {
		resp, err := c.Search(ctx, appstore.SearchQuery{Term: o.Term, Country: o.Country, Entity: o.Entity, Limit: keywordRankMaxScan})
		if err != nil {
			return classifyAppStoreErr(err)
		}
		capturedAt = s2bNowIfZero(resp.Meta.FetchedAt)
		out.FromCache = resp.Meta.FromCache
		for i := range resp.Results {
			r := resp.Results[i]
			cands = append(cands, keywordCandidate{ID: r.AppID(), BundleID: r.BundleID, Lookup: &r})
		}
	}
	out.CapturedAt = s2bTime(capturedAt)
	out.ResultCount, out.Scanned = len(cands), len(cands)

	words := s2bTermWords(o.Term)
	ids := make([]string, 0, len(cands))
	for i, cand := range cands {
		ids = append(ids, cand.ID)
		if i >= o.Serp {
			continue
		}
		row := keywordTopRow{Rank: i + 1, AppID: cand.ID}
		if cand.Lockup != nil {
			row.Title, row.DeveloperName = cand.Lockup.Title, cand.Lockup.DeveloperName
		}
		if cand.Lookup != nil {
			if row.Title == "" {
				row.Title = cand.Lookup.TrackName
			}
			if row.DeveloperName == "" {
				row.DeveloperName = cand.Lookup.ArtistName
			}
			row.AverageRating = s2bFloat(cand.Lookup.AverageUserRating)
			row.RatingCount = s2bInt(cand.Lookup.UserRatingCount)
		}
		row.TitleMatch = s2bTitleMatches(row.Title, words)
		if row.TitleMatch {
			out.Signals.TitleMatches++
		}
		out.Top = append(out.Top, row)
		out.Signals.TopRatingCounts = append(out.Signals.TopRatingCounts, row.RatingCount)
	}
	out.Signals.ResultCount = out.ResultCount
	out.Signals.TopN = len(out.Top)
	if out.Signals.TopN > 0 {
		out.Signals.TitleMatchShare = math.Round(float64(out.Signals.TitleMatches)/float64(out.Signals.TopN)*1000) / 1000
	}
	if len(cands) == 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("Apple returned no apps for %q in storefront %s (verified empty)", o.Term, o.Country))
	}

	targetID := ""
	if o.Target != nil {
		out.App = &keywordRankApp{AppID: o.Target.ID, BundleID: o.Target.BundleID}
		for i, cand := range cands {
			if (o.Target.ID != "" && cand.ID == o.Target.ID) || (o.Target.BundleID != "" && strings.EqualFold(cand.BundleID, o.Target.BundleID)) {
				rank := i + 1
				out.Rank = &rank
				out.App.AppID = cand.ID
				if cand.Lookup != nil {
					out.App.Title, out.App.BundleID = cand.Lookup.TrackName, cand.Lookup.BundleID
				} else if cand.Lockup != nil {
					out.App.Title = cand.Lockup.Title
				}
				break
			}
		}
		if out.App.AppID == "" {
			// A bundle id that is not in the results: resolve it so the
			// stored data point names the numeric app id.
			lk, err := lookupAppRef(ctx, c, *o.Target, o.Country)
			if err != nil {
				return err
			}
			out.App.AppID, out.App.Title, out.App.BundleID = lk.AppID(), lk.TrackName, lk.BundleID
			// Web-search ids past the hydrated top carry no bundle id: match the
			// resolved numeric id instead.
			for i, cand := range cands {
				if cand.ID == out.App.AppID {
					rank := i + 1
					out.Rank = &rank
					break
				}
			}
		}
		targetID = out.App.AppID
		if out.Rank == nil && len(cands) > 0 {
			out.Notes = append(out.Notes, fmt.Sprintf("app %s is not in the first %d results Apple returned for %q in storefront %s", targetID, len(cands), o.Term, o.Country))
		}
	}

	// The store writes each observation once: a response served from the
	// local cache keeps its fetch time and is already stored.
	rank := 0
	if out.Rank != nil {
		rank = *out.Rank
	}
	recorded, err := s2bRecordKeywordRank(s2bWriteCtx(ctx), st, store.KeywordRank{
		CapturedAt: capturedAt, Storefront: o.Country, Platform: o.Platform, Term: o.Term,
		Source: o.Source, AppID: targetID, Rank: rank, ResultCount: len(cands), TopIDs: ids,
	})
	if err != nil {
		s2bStoreFailed(cmd, &out.Notes, "the keyword capture", err)
	}
	out.Recorded = recorded
	if !recorded && err == nil {
		out.Notes = append(out.Notes, "this capture (served from the local cache, fetched "+out.CapturedAt+") is already stored; pass --no-cache for a fresh data point")
	}

	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, out, flags)
	}
	switch {
	case o.Target == nil:
		_, _ = fmt.Fprintf(w, "%q in %s (%s, %s): %d results\n", o.Term, o.Country, o.Platform, o.Source, out.ResultCount)
	case out.Rank != nil:
		_, _ = fmt.Fprintf(w, "%s ranks #%d for %q in %s (%s, %s; %d results)\n", s2bAppLabel(out.App), *out.Rank, o.Term, o.Country, o.Platform, o.Source, out.ResultCount)
	default:
		_, _ = fmt.Fprintf(w, "%s is not in the first %d results for %q in %s (%s, %s)\n", s2bAppLabel(out.App), out.ResultCount, o.Term, o.Country, o.Platform, o.Source)
	}
	if len(out.Top) > 0 {
		if err := printRows(cmd, flags, out.Top); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(w, "signals: title match %d/%d (%.0f%%)\n", out.Signals.TitleMatches, out.Signals.TopN, out.Signals.TitleMatchShare*100)
	s2bPrintNotes(cmd, out.Notes)
	return nil
}

func s2bAppLabel(a *keywordRankApp) string {
	if a == nil {
		return ""
	}
	if a.Title != "" {
		return fmt.Sprintf("%s (%s)", cliutil.ScrubTerminal(a.Title), a.AppID)
	}
	return a.AppID
}
