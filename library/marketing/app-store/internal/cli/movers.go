// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// moversRow is one app that moved between the two compared chart captures.
// Delta is previousRank - rank, so a positive delta is a climb toward #1.
type moversRow struct {
	Movement           string    `json:"movement"` // climbed | dropped | entered | exited
	AppID              string    `json:"appId"`
	Title              string    `json:"title"`
	DeveloperName      string    `json:"developerName,omitempty"`
	Rank               *int      `json:"rank"`         // null when the app exited
	PreviousRank       *int      `json:"previousRank"` // null when the app entered
	Delta              *int      `json:"delta"`        // null when the app entered or exited
	CapturedAt         time.Time `json:"capturedAt"`
	PreviousCapturedAt time.Time `json:"previousCapturedAt"`
}

// moversCounts tallies the movement kinds of one diff.
type moversCounts struct {
	Climbed, Dropped, Entered, Exited int
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newMoversCmd(flags))
	})
}

func newMoversCmd(flags *rootFlags) *cobra.Command {
	var chart, genre, platform, ageBand, source string
	var limit, minChange int
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Diff the two latest stored captures of a chart: climbed, dropped, entered, exited",
		Long: strings.TrimSpace(`
Diff the two most recent comparable captures of one chart in the local store
and list every app that climbed, dropped, entered or exited, with rank deltas.
Captures are comparable when they share storefront, platform, genre, chart,
source and row limit (the chart page records 200 ranks, the top-grossing RSS
feed 100), so "entered" means entered the captured top 200 (or 100). A Kids
chart is its own series: --age-band reads it (as captured by 'top
--age-band'); without --age-band movers reads the overall chart.

delta = previousRank - rank: positive is a climb toward #1. Rows are ordered
climbed (biggest climb first), dropped (biggest fall first), entered (best
rank first), exited (best previous rank first); --limit caps each kind.

Reads snapshots written by 'top'; run 'top' for the same chart at two
different times first. With fewer than two comparable captures, or no
movement, the result is [] and a note on stderr says why.

Use this for the whole-chart diff between two captures. For one app's rank
over time use 'rank-history'; for apps climbing in several storefronts at
once use 'breakouts'.`),
		Example: strings.Trim(`
  app-store-pp-cli movers --chart top-free --genre games --country us --agent
  app-store-pp-cli movers --chart top-paid --genre 36 --platform ipad --limit 10
  app-store-pp-cli movers --chart top-grossing --country jp --min-change 5 --json
  app-store-pp-cli movers --chart top-free --age-band 6-8 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "movers")
			}
			if err := s3aNoArgs(cmd, args); err != nil {
				return err
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (no cap) or more, got %d", limit))
			}
			if minChange < 1 {
				return usageErr(fmt.Errorf("--min-change must be at least 1, got %d", minChange))
			}
			spec, err := s3aResolveChart(flags, chart, genre, platform, ageBand, source)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]moversRow, 0)
			hint := spec.topHint() + "   (twice, at different times)"
			if s3aMissingMirror(cmd, hint) {
				return emitMovers(cmd, flags, rows, "no local chart snapshots yet", "")
			}
			st, err := openAppStoreStoreRead(cmd, flags)
			if err != nil {
				return err
			}
			defer s3aCloseStore(st)

			prev, latest, err := store.LatestTwoCharts(ctx, st, spec.Key)
			if err != nil {
				return apiErr(fmt.Errorf("reading chart snapshots: %w", err))
			}
			if latest == nil {
				return emitMovers(cmd, flags, rows, fmt.Sprintf("no stored captures of %s; run: %s", spec.label(), hint), "")
			}
			warnStale(cmd, flags, latest.CapturedAt, time.Now(), "capture of "+spec.label(), spec.topHint())
			if prev == nil {
				note := fmt.Sprintf("only one comparable capture of %s (%s, %s, %d ranks); movers needs two; run: %s",
					spec.label(), latest.CapturedAt.UTC().Format(time.RFC3339), latest.Source, latest.RowLimit, spec.topHint())
				if caps, cerr := store.ChartCaptures(ctx, st, spec.Key); cerr == nil && len(caps) > 1 {
					note += fmt.Sprintf(" (%d other stored capture(s) use a different source or row limit and are not comparable)", len(caps)-1)
				}
				return emitMovers(cmd, flags, rows, note, "")
			}
			rows, counts := s3aComputeMovers(prev, latest, minChange, limit)
			compared := spec
			compared.Key.Source = latest.Source
			summary := fmt.Sprintf("%s, %d ranks: %s -> %s: %d climbed, %d dropped, %d entered, %d exited",
				compared.label(), latest.RowLimit, prev.CapturedAt.UTC().Format(time.RFC3339), latest.CapturedAt.UTC().Format(time.RFC3339),
				counts.Climbed, counts.Dropped, counts.Entered, counts.Exited)
			note := ""
			if len(rows) == 0 {
				note = fmt.Sprintf("no app moved by %d or more ranks between the two captures", minChange)
			}
			return emitMovers(cmd, flags, rows, note, summary)
		},
	}
	cmd.Flags().StringVar(&chart, "chart", "top-free", "Chart: top-free, top-paid or top-grossing")
	cmd.Flags().StringVar(&genre, "genre", "36", "Genre id or name (36 All Apps, 6014 or games, puzzle, ...)")
	cmd.Flags().StringVar(&platform, "platform", "iphone", "Chart platform: iphone, ipad or mac")
	cmd.Flags().StringVar(&ageBand, "age-band", "", "Kids chart age band (with --genre 36): all (All Kids), 5-under, 6-8 or 9-11; unset = the overall chart")
	cmd.Flags().StringVar(&source, "source", "", "Pin the capture source: web (chart page, 200 ranks) or rss (feed, 100 ranks); default: the latest capture's")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows per movement kind (0 = no cap)")
	cmd.Flags().IntVar(&minChange, "min-change", 1, "Minimum rank change for an app to count as climbed or dropped")
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI data directory)")
	return cmd
}

// s3aComputeMovers diffs two comparable captures. limit (0 = no cap) caps
// each movement kind after sorting; counts are taken before the cap.
func s3aComputeMovers(prev, latest *store.ChartSnapshot, minChange, limit int) ([]moversRow, moversCounts) {
	best := func(rows []store.ChartRow) map[string]store.ChartRow {
		out := make(map[string]store.ChartRow, len(rows))
		for _, r := range rows {
			if r.AppID == "" || r.Rank <= 0 {
				continue
			}
			if cur, ok := out[r.AppID]; !ok || r.Rank < cur.Rank {
				out[r.AppID] = r
			}
		}
		return out
	}
	before, after := best(prev.Rows), best(latest.Rows)
	var climbed, dropped, entered, exited []moversRow
	base := func(movement string, r store.ChartRow) moversRow {
		return moversRow{
			Movement: movement, AppID: r.AppID, Title: r.Title, DeveloperName: r.DeveloperName,
			CapturedAt: latest.CapturedAt.UTC(), PreviousCapturedAt: prev.CapturedAt.UTC(),
		}
	}
	for id, now := range after {
		was, ok := before[id]
		if !ok {
			row := base("entered", now)
			row.Rank = s3aIntPtr(now.Rank)
			entered = append(entered, row)
			continue
		}
		delta := was.Rank - now.Rank
		if delta >= minChange || -delta >= minChange {
			kind := "climbed"
			if delta < 0 {
				kind = "dropped"
			}
			row := base(kind, now)
			if row.Title == "" {
				row.Title = was.Title
			}
			if row.DeveloperName == "" {
				row.DeveloperName = was.DeveloperName
			}
			row.Rank, row.PreviousRank, row.Delta = s3aIntPtr(now.Rank), s3aIntPtr(was.Rank), s3aIntPtr(delta)
			if delta > 0 {
				climbed = append(climbed, row)
			} else {
				dropped = append(dropped, row)
			}
		}
	}
	for id, was := range before {
		if _, ok := after[id]; ok {
			continue
		}
		row := base("exited", was)
		row.PreviousRank = s3aIntPtr(was.Rank)
		exited = append(exited, row)
	}
	sort.Slice(climbed, func(i, j int) bool {
		if *climbed[i].Delta != *climbed[j].Delta {
			return *climbed[i].Delta > *climbed[j].Delta
		}
		return *climbed[i].Rank < *climbed[j].Rank
	})
	sort.Slice(dropped, func(i, j int) bool {
		if *dropped[i].Delta != *dropped[j].Delta {
			return *dropped[i].Delta < *dropped[j].Delta
		}
		return *dropped[i].Rank < *dropped[j].Rank
	})
	sort.Slice(entered, func(i, j int) bool { return *entered[i].Rank < *entered[j].Rank })
	sort.Slice(exited, func(i, j int) bool { return *exited[i].PreviousRank < *exited[j].PreviousRank })
	counts := moversCounts{Climbed: len(climbed), Dropped: len(dropped), Entered: len(entered), Exited: len(exited)}
	capKind := func(rows []moversRow) []moversRow {
		if limit > 0 && len(rows) > limit {
			return rows[:limit]
		}
		return rows
	}
	out := make([]moversRow, 0, len(climbed)+len(dropped)+len(entered)+len(exited))
	out = append(out, capKind(climbed)...)
	out = append(out, capKind(dropped)...)
	out = append(out, capKind(entered)...)
	out = append(out, capKind(exited)...)
	return out, counts
}

// emitMovers prints the rows; summary (when set) is the comparison line,
// note explains an empty result.
func emitMovers(cmd *cobra.Command, flags *rootFlags, rows []moversRow, note, summary string) error {
	if summary != "" && !wantsHumanTable(cmd.OutOrStdout(), flags) {
		s3aStderr(cmd, "movers: %s\n", summary)
	}
	return s3aEmitList(cmd, flags, rows, note, func(b *bytes.Buffer) {
		if summary != "" {
			fmt.Fprintln(b, summary)
		}
		if len(rows) == 0 {
			if note != "" {
				fmt.Fprintln(b, note)
			}
			return
		}
		table := make([][]string, 0, len(rows))
		for _, r := range rows {
			delta := "-"
			if r.Delta != nil {
				delta = fmt.Sprintf("%+d", *r.Delta)
			}
			table = append(table, []string{r.Movement, s3aRankText(r.Rank), s3aRankText(r.PreviousRank), delta, r.AppID, s3aCell(r.Title, 50)})
		}
		s3aTable(b, []string{"MOVEMENT", "RANK", "PREV", "DELTA", "APP ID", "TITLE"}, table)
	})
}
