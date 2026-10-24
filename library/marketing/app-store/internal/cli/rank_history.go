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

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// rankHistoryMark is one notable rank: first seen, peak or last seen.
type rankHistoryMark struct {
	CapturedAt time.Time `json:"capturedAt"`
	Rank       int       `json:"rank"`
}

// rankHistoryPoint is the app's rank in one stored capture of the chart;
// Rank is null (and InChart false) when the capture did not include it.
type rankHistoryPoint struct {
	CapturedAt time.Time `json:"capturedAt"`
	Rank       *int      `json:"rank"`
	InChart    bool      `json:"inChart"`
	Source     string    `json:"source"`
	RowLimit   int       `json:"rowLimit"`
}

// rankHistoryElsewhere is another stored chart that includes the app.
type rankHistoryElsewhere struct {
	Storefront string    `json:"storefront"`
	Platform   string    `json:"platform"`
	GenreID    string    `json:"genreId"`
	AgeBand    string    `json:"ageBand,omitempty"`
	Chart      string    `json:"chart"`
	Source     string    `json:"source"`
	Captures   int       `json:"captures"`
	BestRank   int       `json:"bestRank"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

type rankHistoryView struct {
	AppID      string                 `json:"appId"`
	Title      string                 `json:"title,omitempty"`
	Storefront string                 `json:"storefront"`
	Platform   string                 `json:"platform"`
	GenreID    string                 `json:"genreId"`
	Genre      string                 `json:"genre,omitempty"`
	AgeBand    string                 `json:"ageBand,omitempty"`
	Chart      string                 `json:"chart"`
	Source     string                 `json:"source,omitempty"`
	Since      *time.Time             `json:"since,omitempty"`
	Captures   int                    `json:"captures"`
	Charted    int                    `json:"charted"`
	FirstSeen  *rankHistoryMark       `json:"firstSeen"`
	Peak       *rankHistoryMark       `json:"peak"`
	LastSeen   *rankHistoryMark       `json:"lastSeen"`
	Latest     *rankHistoryPoint      `json:"latest"`
	Series     []rankHistoryPoint     `json:"series"`
	Elsewhere  []rankHistoryElsewhere `json:"elsewhere,omitempty"`
	Note       string                 `json:"note,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newRankHistoryCmd(flags))
	})
}

func newRankHistoryCmd(flags *rootFlags) *cobra.Command {
	var chart, genre, platform, ageBand, source, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "rank-history <app>",
		Short: "Show one app's stored rank series in a chart: first seen, peak, last seen",
		Long: strings.TrimSpace(`
Read the local chart snapshots of one chart (storefront from --country, plus
--chart, --genre and --platform) and return one app's rank in every stored
capture: first seen, peak (best rank), last seen, the latest capture and the
series. A capture that did not include the app has rank null and
inChart false; when the app is in none of the captures the series is []
and 'elsewhere' lists the stored charts that do include it (a Kids chart
there carries its ageBand). --age-band reads a Kids chart's own series (as
captured by 'top --age-band'); without it the overall chart is read.

<app> is the numeric App Store id, id123, a store URL or a bundle id (resolved
from the stored listing snapshots, no network). Reads snapshots
written by 'top'; run 'top' for the chart repeatedly over time to grow the
series. --since limits the window, --limit keeps the newest points (the
summary still covers the whole window).

Use this for one app's rank trajectory. For the whole-chart diff between two
captures use 'movers'; for demand proxies per storefront use 'traction'.`),
		Example: strings.Trim(`
  app-store-pp-cli rank-history 324684580 --chart top-free --genre 36 --country us --agent
  app-store-pp-cli rank-history 553834731 --chart top-grossing --genre games --since 30d
  app-store-pp-cli rank-history https://apps.apple.com/gb/app/id324684580 --json --select peak,lastSeen
  app-store-pp-cli rank-history 324684580 --chart top-free --age-band 9-11 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank-history")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<app> is required: a numeric App Store id such as 324684580 (or id123, a store URL or a stored bundle id)"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("rank-history takes one app, got %d arguments", len(args)))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (all points) or more, got %d", limit))
			}
			ref, appID, cc, err := s3aLocalApp(cmd, flags, args[0])
			if err != nil {
				return err
			}
			spec, err := s3aResolveChart(flags, chart, genre, platform, ageBand, source)
			if err != nil {
				return err
			}
			spec.Key.Storefront = cc
			sinceT, err := s3aSince(since, time.Now())
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := rankHistoryView{
				AppID: appID, Storefront: cc, Platform: spec.Key.Platform, GenreID: spec.Key.GenreID,
				Genre: spec.GenreName, AgeBand: spec.Key.AgeBand, Chart: spec.Key.Chart, Source: spec.Key.Source,
				Series: make([]rankHistoryPoint, 0),
			}
			if !sinceT.IsZero() {
				view.Since = &sinceT
			}
			hint := spec.topHint() + "   (repeatedly over time)"
			if s3aMissingMirror(cmd, hint) {
				view.Note = "no local chart snapshots yet; run: " + hint
				return emitRankHistory(cmd, flags, view)
			}
			st, err := openAppStoreStoreRead(cmd, flags)
			if err != nil {
				return err
			}
			defer s3aCloseStore(st)
			if appID, view.Note, err = s3aResolveLocalApp(ctx, st, ref); err != nil || appID == "" {
				if err != nil {
					return err
				}
				return emitRankHistory(cmd, flags, view)
			}
			view.AppID = appID

			caps, err := store.ChartCaptures(ctx, st, spec.Key)
			if err != nil {
				return apiErr(fmt.Errorf("reading chart captures: %w", err))
			}
			if len(caps) > 0 {
				warnStale(cmd, flags, caps[len(caps)-1].CapturedAt, time.Now(), "capture of "+spec.label(), spec.topHint())
			}
			inWindow := make([]store.ChartCapture, 0, len(caps))
			for _, c := range caps {
				if sinceT.IsZero() || !c.CapturedAt.Before(sinceT) {
					inWindow = append(inWindow, c)
				}
			}
			rows, err := store.ChartRows(ctx, st, store.ChartQuery{
				Storefronts: []string{cc}, Platform: spec.Key.Platform, GenreID: spec.Key.GenreID, AgeBand: spec.Key.AgeBand,
				Chart: spec.Key.Chart, Source: spec.Key.Source, AppID: appID, Since: sinceT,
			})
			if err != nil {
				return apiErr(fmt.Errorf("reading chart rows: %w", err))
			}
			s3aFillRankHistory(&view, inWindow, rows)
			if view.Charted == 0 {
				elsewhere, err := s3aChartsElsewhere(ctx, st, appID, spec.Key)
				if err != nil {
					return apiErr(fmt.Errorf("reading chart rows: %w", err))
				}
				view.Elsewhere = elsewhere
				window := ""
				if !sinceT.IsZero() {
					window = " since " + sinceT.Format(time.RFC3339)
				}
				if view.Captures == 0 {
					view.Note = fmt.Sprintf("no stored captures of %s%s; run: %s", spec.label(), window, spec.topHint())
				} else {
					view.Note = fmt.Sprintf("app %s is not in any of the %d stored captures of %s%s", appID, view.Captures, spec.label(), window)
				}
				if len(elsewhere) > 0 {
					view.Note += fmt.Sprintf("; it appears in %d other stored chart(s), listed under elsewhere", len(elsewhere))
				}
			} else if view.Latest != nil && !view.Latest.InChart && view.LastSeen != nil {
				view.Note = fmt.Sprintf("not in the latest capture (%s); last seen at #%d on %s",
					view.Latest.CapturedAt.Format(time.RFC3339), view.LastSeen.Rank, view.LastSeen.CapturedAt.Format(time.RFC3339))
			}
			if limit > 0 && len(view.Series) > limit {
				view.Series = view.Series[len(view.Series)-limit:]
			}
			return emitRankHistory(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&chart, "chart", "top-free", "Chart: top-free, top-paid or top-grossing")
	cmd.Flags().StringVar(&genre, "genre", "36", "Genre id or name (36 All Apps, 6014 or games, puzzle, ...)")
	cmd.Flags().StringVar(&platform, "platform", "iphone", "Chart platform: iphone, ipad or mac")
	cmd.Flags().StringVar(&ageBand, "age-band", "", "Kids chart age band (with --genre 36): all (All Kids), 5-under, 6-8 or 9-11; unset = the overall chart")
	cmd.Flags().StringVar(&source, "source", "", "Only captures from this source: web (chart page, 200 ranks) or rss (feed, 100 ranks)")
	cmd.Flags().StringVar(&since, "since", "", "Only captures in this window: a duration such as 30d or 12w, or a date such as 2026-10-01")
	cmd.Flags().IntVar(&limit, "limit", 0, "Keep only the newest N series points (0 = all); the summary covers the whole window")
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI data directory)")
	return cmd
}

// s3aFillRankHistory builds the series (one point per capture once the app
// has charted at all) and the first-seen / peak / last-seen summary.
func s3aFillRankHistory(view *rankHistoryView, caps []store.ChartCapture, rows []store.ChartRowRecord) {
	type hit struct {
		rank  int
		title string
	}
	key := func(t time.Time, source string) string { return strconv.FormatInt(t.Unix(), 10) + "|" + source }
	hits := make(map[string]hit, len(rows))
	var titleAt time.Time
	for _, r := range rows {
		k := key(r.CapturedAt, r.Source)
		if cur, ok := hits[k]; !ok || r.Rank < cur.rank {
			hits[k] = hit{rank: r.Rank, title: r.Title}
		}
		if r.Title != "" && !r.CapturedAt.Before(titleAt) {
			view.Title, titleAt = r.Title, r.CapturedAt
		}
	}
	view.Captures = len(caps)
	points := make([]rankHistoryPoint, 0, len(caps))
	for _, c := range caps {
		p := rankHistoryPoint{CapturedAt: c.CapturedAt.UTC(), Source: c.Source, RowLimit: c.RowLimit}
		if h, ok := hits[key(c.CapturedAt, c.Source)]; ok {
			p.Rank, p.InChart = s3aIntPtr(h.rank), true
			mark := &rankHistoryMark{CapturedAt: p.CapturedAt, Rank: h.rank}
			view.Charted++
			if view.FirstSeen == nil {
				view.FirstSeen = mark
			}
			if view.Peak == nil || h.rank < view.Peak.Rank {
				view.Peak = mark
			}
			view.LastSeen = mark
		}
		points = append(points, p)
	}
	if view.Charted == 0 {
		return
	}
	view.Series = points
	last := points[len(points)-1]
	view.Latest = &last
}

// s3aChartsElsewhere lists the other stored charts (any storefront, overall
// and Kids charts alike) that include the app, most recently seen first, at
// most ten. A Kids chart is a different chart from the overall one of its
// genre, so the age band is part of the key.
func s3aChartsElsewhere(ctx context.Context, st *store.Store, appID string, exclude store.ChartKey) ([]rankHistoryElsewhere, error) {
	rows, err := store.ChartRows(ctx, st, store.ChartQuery{AppID: appID, AgeBand: store.AnyAgeBand})
	if err != nil {
		return nil, err
	}
	byKey := map[string]*rankHistoryElsewhere{}
	seenCapture := map[string]bool{}
	for _, r := range rows {
		if r.Storefront == exclude.Storefront && r.Platform == exclude.Platform && r.GenreID == exclude.GenreID &&
			r.AgeBand == exclude.AgeBand && r.Chart == exclude.Chart && (exclude.Source == "" || r.Source == exclude.Source) {
			continue
		}
		k := strings.Join([]string{r.Storefront, r.Platform, r.GenreID, r.AgeBand, r.Chart, r.Source}, "|")
		e, ok := byKey[k]
		if !ok {
			e = &rankHistoryElsewhere{Storefront: r.Storefront, Platform: r.Platform, GenreID: r.GenreID, AgeBand: r.AgeBand, Chart: r.Chart, Source: r.Source, BestRank: r.Rank}
			byKey[k] = e
		}
		ck := k + "|" + strconv.FormatInt(r.CapturedAt.Unix(), 10)
		if !seenCapture[ck] {
			seenCapture[ck] = true
			e.Captures++
		}
		if r.Rank < e.BestRank {
			e.BestRank = r.Rank
		}
		if r.CapturedAt.After(e.LastSeenAt) {
			e.LastSeenAt = r.CapturedAt.UTC()
		}
	}
	out := make([]rankHistoryElsewhere, 0, len(byKey))
	for _, e := range byKey {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastSeenAt.Equal(out[j].LastSeenAt) {
			return out[i].LastSeenAt.After(out[j].LastSeenAt)
		}
		if out[i].Captures != out[j].Captures {
			return out[i].Captures > out[j].Captures
		}
		return out[i].BestRank < out[j].BestRank
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out, nil
}

func emitRankHistory(cmd *cobra.Command, flags *rootFlags, v rankHistoryView) error {
	return s3aEmit(cmd, flags, v, func(b *bytes.Buffer) {
		name := v.AppID
		if v.Title != "" {
			name += " " + s3aCell(v.Title, 60)
		}
		genre := strings.TrimSpace(v.GenreID + " " + v.Genre)
		if band := ageBandLabel(v.AgeBand); band != "" {
			genre += " (" + band + ")"
		}
		fmt.Fprintf(b, "%s in %s %s %s %s\n", name, v.Storefront, v.Platform, genre, v.Chart)
		fmt.Fprintf(b, "captures: %d, charted in: %d\n", v.Captures, v.Charted)
		for _, m := range []struct {
			label string
			mark  *rankHistoryMark
		}{{"first seen", v.FirstSeen}, {"peak", v.Peak}, {"last seen", v.LastSeen}} {
			if m.mark != nil {
				fmt.Fprintf(b, "%s: #%d on %s\n", m.label, m.mark.Rank, s3aTime(m.mark.CapturedAt))
			}
		}
		if len(v.Series) > 0 {
			rows := make([][]string, 0, len(v.Series))
			for _, p := range v.Series {
				rows = append(rows, []string{s3aTime(p.CapturedAt), s3aRankText(p.Rank), fmt.Sprintf("%s/%d", p.Source, p.RowLimit)})
			}
			s3aTable(b, []string{"CAPTURED", "RANK", "SOURCE"}, rows)
		}
		for _, e := range v.Elsewhere {
			genre := e.GenreID
			if band := ageBandLabel(e.AgeBand); band != "" {
				genre += " (" + band + ")"
			}
			fmt.Fprintf(b, "elsewhere: %s %s %s %s (%s): %d capture(s), best #%d, last %s\n", e.Storefront, e.Platform, genre, e.Chart, e.Source, e.Captures, e.BestRank, s3aTime(e.LastSeenAt))
		}
		if v.Note != "" {
			fmt.Fprintln(b, v.Note)
		}
	})
}
