// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// keywordHistorySerp is one stored search result position of a capture.
type keywordHistorySerp struct {
	Position int    `json:"position"`
	AppID    string `json:"appId"`
	Title    string `json:"title,omitempty"`
}

// keywordHistoryPoint is one stored keyword capture. With --app, Rank is
// that app's position (null when it was not within Depth results); without
// --app it is the capture's own target app and rank, when it had one.
type keywordHistoryPoint struct {
	CapturedAt  time.Time            `json:"capturedAt"`
	Platform    string               `json:"platform"`
	Source      string               `json:"source"`
	AppID       string               `json:"appId,omitempty"`
	Rank        *int                 `json:"rank"`
	Found       bool                 `json:"found"`
	Depth       int                  `json:"depth"`
	ResultCount int                  `json:"resultCount"`
	Top         []keywordHistorySerp `json:"top,omitempty"`
}

type keywordHistoryView struct {
	Term       string                `json:"term"`
	Storefront string                `json:"storefront"`
	Platform   string                `json:"platform,omitempty"`
	Source     string                `json:"source,omitempty"`
	AppID      string                `json:"appId,omitempty"`
	Since      *time.Time            `json:"since,omitempty"`
	Captures   int                   `json:"captures"`
	FoundIn    int                   `json:"foundIn"`
	BestRank   *rankHistoryMark      `json:"bestRank"`
	LatestRank *int                  `json:"latestRank"`
	Change     *int                  `json:"change"`
	Series     []keywordHistoryPoint `json:"series"`
	Note       string                `json:"note,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newKeywordHistoryCmd(flags))
	})
}

func newKeywordHistoryCmd(flags *rootFlags) *cobra.Command {
	var app, platform, source, since string
	var serp, limit int
	cmd := &cobra.Command{
		Use:   "keyword-history <term>",
		Short: "Show the stored keyword-rank series for a search term, with optional SERP replay",
		Long: strings.TrimSpace(`
Read the local keyword captures of one search term in one storefront
(--country) and return them oldest first.

With --app, every point carries that app's position: the stored rank when
the capture targeted the app, else its position in the capture's stored
result list (rank null when it was not within 'depth' results). The summary
gives the best rank, the latest rank and the change since the previous
capture (positive = moved up). An app found in none of the captures returns
an empty series and a note.

--serp N replays the stored top N results of every capture (titles from
stored listings when known), which is the search-results history no paid
tool keeps for free. Without --app or --serp each point shows the capture's
own target and rank.

Reads captures written by 'keyword-rank' and 'search-store'; run them
repeatedly over time to grow the series. Use 'keyword-share' for which apps
hold a set of terms.`),
		Example: strings.Trim(`
  app-store-pp-cli keyword-history spotify --app 324684580 --country us --agent
  app-store-pp-cli keyword-history "music streaming" --serp 10 --since 30d
  app-store-pp-cli keyword-history chatgpt --app 6448311069 --source itunes --json --select bestRank,latestRank`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "term=spotify;--app=324684580",
			// Any term is a valid query: a term never captured is an empty
			// history (exit 0 with a note), not a bad argument.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "keyword-history")
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<term> is required: the search term whose captures to read"))
			}
			if serp < 0 || serp > 200 {
				return usageErr(fmt.Errorf("--serp must be between 0 (off) and 200, got %d", serp))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (all points) or more, got %d", limit))
			}
			cc, err := appStoreCountry(flags)
			if err != nil {
				return err
			}
			appID := ""
			var appRef *appstore.AppRef
			if strings.TrimSpace(app) != "" {
				ref, err := parseAppRef(app)
				if err != nil {
					return usageErr(fmt.Errorf("--app: %w", err))
				}
				// A bundle id is resolved against the store once it is open.
				appRef, appID = &ref, ref.ID
				if appID == "" {
					appID = ref.BundleID
				}
			}
			plat := ""
			if strings.TrimSpace(platform) != "" {
				if plat, err = parsePlatform(platform); err != nil {
					return err
				}
			}
			src, err := s3aKeywordSource(source)
			if err != nil {
				return err
			}
			sinceT, err := s3aSince(since, time.Now())
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := keywordHistoryView{
				Term: strings.ToLower(term), Storefront: cc, Platform: plat, Source: src, AppID: appID,
				Series: make([]keywordHistoryPoint, 0),
			}
			if !sinceT.IsZero() {
				view.Since = &sinceT
			}
			hint := fmt.Sprintf("%s keyword-rank %q --country %s", appStoreCLIName, term, cc)
			if appID != "" {
				hint += " --app " + appID
			}
			hint += "   (repeatedly over time)"
			if s3aMissingMirror(cmd, hint) {
				view.Note = "no local keyword captures yet; run: " + hint
				return emitKeywordHistory(cmd, flags, view)
			}
			st, err := openAppStoreStoreRead(cmd, flags)
			if err != nil {
				return err
			}
			defer s3aCloseStore(st)
			if appRef != nil {
				if appID, view.Note, err = s3aResolveLocalApp(ctx, st, *appRef); err != nil || appID == "" {
					if err != nil {
						return err
					}
					return emitKeywordHistory(cmd, flags, view)
				}
				view.AppID = appID
			}

			caps, err := store.KeywordSeries(ctx, st, store.KeywordFilter{Term: term, Storefront: cc, Platform: plat, Source: src, Since: sinceT})
			if err != nil {
				return apiErr(fmt.Errorf("reading keyword captures: %w", err))
			}
			if len(caps) > 0 {
				warnStale(cmd, flags, caps[len(caps)-1].CapturedAt, time.Now(), fmt.Sprintf("keyword capture of %q", strings.ToLower(term)), strings.TrimSuffix(hint, "   (repeatedly over time)"))
			}
			titles := map[string]string{}
			if serp > 0 && len(caps) > 0 {
				if titles, err = s3aStoredTitles(ctx, st, cc); err != nil {
					return apiErr(fmt.Errorf("reading stored titles: %w", err))
				}
			}
			s3aFillKeywordHistory(&view, caps, appID, serp, titles)
			scope := fmt.Sprintf("%q in %s", view.Term, cc)
			if plat != "" {
				scope += " " + plat
			}
			if src != "" {
				scope += " (" + src + ")"
			}
			switch {
			case view.Captures == 0:
				view.Note = fmt.Sprintf("no stored keyword captures for %s; run: %s", scope, hint)
			case appID != "" && view.FoundIn == 0:
				depth := 0
				for _, k := range caps {
					depth = max(depth, s3aKeywordDepth(k, appID))
				}
				view.Note = fmt.Sprintf("app %s was not found in any of the %d stored captures of %s (looked up to %d results deep)", appID, view.Captures, scope, depth)
			default:
				if mixed := s3aKeywordMix(caps); mixed != "" {
					view.Note = mixed
				}
			}
			if limit > 0 && len(view.Series) > limit {
				view.Series = view.Series[len(view.Series)-limit:]
			}
			return emitKeywordHistory(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "App whose position to report in every capture (numeric id, id123, store URL or a stored bundle id)")
	cmd.Flags().IntVar(&serp, "serp", 0, "Replay the stored top N results of every capture (0 = off, max 200)")
	cmd.Flags().StringVar(&platform, "platform", "", "Only captures for this platform (iphone, ipad, mac ...); default: all")
	cmd.Flags().StringVar(&source, "source", "", "Only captures from this source: itunes (Search API) or web (opt-in web search); default: all")
	cmd.Flags().StringVar(&since, "since", "", "Only captures in this window: a duration such as 30d or 12w, or a date such as 2026-10-01")
	cmd.Flags().IntVar(&limit, "limit", 0, "Keep only the newest N captures in the series (0 = all); the summary covers the whole window")
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI data directory)")
	return cmd
}

// s3aKeywordSource maps --source for keyword history.
func s3aKeywordSource(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "any", "all":
		return "", nil
	case "itunes", "search-api", "searchapi":
		return store.SourceSearchAPI, nil
	case "web":
		return store.SourceWebSearch, nil
	}
	return "", usageErr(fmt.Errorf("--source: unknown keyword source %q; use itunes (Search API) or web (web search page)", s))
}

// s3aKeywordDepth is how many results a capture lets us look an app up in:
// the scanned result count when the capture targeted the app, else the
// length of its stored result list.
func s3aKeywordDepth(k store.KeywordRank, appID string) int {
	if appID != "" && k.AppID == appID {
		return max(k.ResultCount, len(k.TopIDs))
	}
	return len(k.TopIDs)
}

// s3aDistinctKeywordCaptures keeps one capture per (capture time, platform,
// source): one search response is stored once per tracked target (a second
// keyword-rank call is served from the cache with the same capture time), and
// counting it twice zeroes the change. The copy that targeted appID wins.
func s3aDistinctKeywordCaptures(caps []store.KeywordRank, appID string) []store.KeywordRank {
	type key struct {
		at               time.Time
		platform, source string
	}
	idx := map[key]int{}
	out := make([]store.KeywordRank, 0, len(caps))
	for _, k := range caps {
		kk := key{k.CapturedAt.UTC(), k.Platform, k.Source}
		if i, ok := idx[kk]; ok {
			if appID != "" && k.AppID == appID && out[i].AppID != appID {
				out[i] = k
			}
			continue
		}
		idx[kk] = len(out)
		out = append(out, k)
	}
	return out
}

// s3aFillKeywordHistory turns captures (oldest first) into points and the
// summary. With an app that is never found the series stays empty.
func s3aFillKeywordHistory(view *keywordHistoryView, caps []store.KeywordRank, appID string, serp int, titles map[string]string) {
	caps = s3aDistinctKeywordCaptures(caps, appID)
	view.Captures = len(caps)
	points := make([]keywordHistoryPoint, 0, len(caps))
	for _, k := range caps {
		p := keywordHistoryPoint{CapturedAt: k.CapturedAt.UTC(), Platform: k.Platform, Source: k.Source, ResultCount: k.ResultCount}
		target := appID
		if target == "" {
			target = k.AppID
		}
		p.AppID = target
		if target != "" {
			p.Depth = s3aKeywordDepth(k, target)
			if k.AppID == target {
				if k.Rank > 0 {
					p.Rank, p.Found = s3aIntPtr(k.Rank), true
				}
			} else {
				for i, id := range k.TopIDs {
					if id == target {
						p.Rank, p.Found = s3aIntPtr(i+1), true
						break
					}
				}
			}
		} else {
			p.Depth = len(k.TopIDs)
		}
		if serp > 0 {
			p.Top = make([]keywordHistorySerp, 0, min(serp, len(k.TopIDs)))
			for i, id := range k.TopIDs {
				if i >= serp {
					break
				}
				p.Top = append(p.Top, keywordHistorySerp{Position: i + 1, AppID: id, Title: titles[id]})
			}
		}
		if appID != "" && p.Found {
			view.FoundIn++
			if view.BestRank == nil || *p.Rank < view.BestRank.Rank {
				view.BestRank = &rankHistoryMark{CapturedAt: p.CapturedAt, Rank: *p.Rank}
			}
		}
		points = append(points, p)
	}
	if appID != "" && len(points) > 0 {
		last := points[len(points)-1]
		view.LatestRank = last.Rank
		if len(points) > 1 {
			prev := points[len(points)-2]
			if prev.Rank != nil && last.Rank != nil {
				view.Change = s3aIntPtr(*prev.Rank - *last.Rank)
			}
		}
	}
	if appID != "" && view.FoundIn == 0 && serp == 0 {
		return
	}
	view.Series = points
}

// s3aKeywordMix warns when the captures mix platforms or sources, whose
// ranks are not directly comparable.
func s3aKeywordMix(caps []store.KeywordRank) string {
	plats, srcs := map[string]bool{}, map[string]bool{}
	for _, k := range caps {
		plats[k.Platform] = true
		srcs[k.Source] = true
	}
	keys := func(m map[string]bool) string {
		out := make([]string, 0, len(m))
		for k := range m {
			if k == "" {
				k = "(none)"
			}
			out = append(out, k)
		}
		sort.Strings(out)
		return strings.Join(out, ", ")
	}
	parts := []string{}
	if len(srcs) > 1 {
		parts = append(parts, "sources "+keys(srcs)+" (pass --source)")
	}
	if len(plats) > 1 {
		parts = append(parts, "platforms "+keys(plats)+" (pass --platform)")
	}
	if len(parts) == 0 {
		return ""
	}
	return "the captures mix " + strings.Join(parts, " and ") + "; ranks from different sources or platforms are not directly comparable"
}

// s3aStoredTitles maps app ids to their latest stored listing title,
// preferring the given storefront.
func s3aStoredTitles(ctx context.Context, st *store.Store, storefront string) (map[string]string, error) {
	tracked, err := store.TrackedApps(ctx, st, time.Time{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(tracked))
	for _, t := range tracked {
		if t.Title == "" {
			continue
		}
		if _, ok := out[t.AppID]; !ok || t.Storefront == storefront {
			out[t.AppID] = t.Title
		}
	}
	return out, nil
}

func emitKeywordHistory(cmd *cobra.Command, flags *rootFlags, v keywordHistoryView) error {
	return s3aEmit(cmd, flags, v, func(b *bytes.Buffer) {
		head := fmt.Sprintf("%q in %s", s3aCell(v.Term, 60), v.Storefront)
		if v.AppID != "" {
			head = "app " + v.AppID + " for " + head
		}
		fmt.Fprintf(b, "%s: %d capture(s)", head, v.Captures)
		if v.AppID != "" {
			fmt.Fprintf(b, ", found in %d", v.FoundIn)
			if v.BestRank != nil {
				fmt.Fprintf(b, ", best #%d on %s", v.BestRank.Rank, s3aTime(v.BestRank.CapturedAt))
			}
			if v.LatestRank != nil {
				fmt.Fprintf(b, ", latest #%d", *v.LatestRank)
			}
			if v.Change != nil {
				fmt.Fprintf(b, " (%+d)", *v.Change)
			}
		}
		b.WriteByte('\n')
		if len(v.Series) > 0 {
			rows := make([][]string, 0, len(v.Series))
			for _, p := range v.Series {
				top := make([]string, 0, len(p.Top))
				for _, s := range p.Top {
					label := s.AppID
					if s.Title != "" {
						label = s3aCell(s.Title, 24)
					}
					top = append(top, strconv.Itoa(s.Position)+"."+label)
					if len(top) == 5 {
						break
					}
				}
				rows = append(rows, []string{s3aTime(p.CapturedAt), p.AppID, s3aRankText(p.Rank), strconv.Itoa(p.Depth),
					strconv.Itoa(p.ResultCount), p.Source + "/" + p.Platform, strings.Join(top, " ")})
			}
			s3aTable(b, []string{"CAPTURED", "APP", "RANK", "DEPTH", "RESULTS", "SOURCE", "TOP"}, rows)
		}
		if v.Note != "" {
			fmt.Fprintln(b, v.Note)
		}
	})
}
