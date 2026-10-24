// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

type breakoutMarket struct {
	Storefront string `json:"storefront"`
	FromRank   *int   `json:"fromRank"`
	ToRank     int    `json:"toRank"`
	Gain       int    `json:"gain"`
	Entered    bool   `json:"entered"`
	From       string `json:"from"`
	To         string `json:"to"`
}

type breakoutRow struct {
	AppID         string           `json:"appId"`
	Title         string           `json:"title"`
	DeveloperName string           `json:"developerName"`
	Markets       int              `json:"markets"`
	MedianGain    float64          `json:"medianGain"`
	BestRank      int              `json:"bestRank"`
	Storefronts   []breakoutMarket `json:"storefronts"`
}

type breakoutCompared struct {
	Storefront string `json:"storefront"`
	From       string `json:"from"`
	To         string `json:"to"`
	Source     string `json:"source"`
	RowLimit   int    `json:"rowLimit"`
	Captures   int    `json:"capturesInWindow"`
}

type breakoutSkipped struct {
	Storefront string `json:"storefront"`
	Reason     string `json:"reason"`
	Captures   int    `json:"capturesInWindow"`
}

type breakoutsMeta struct {
	Platform            string             `json:"platform"`
	GenreID             string             `json:"genre_id"`
	GenreName           string             `json:"genre_name"`
	AgeBand             string             `json:"age_band,omitempty"`
	Chart               string             `json:"chart"`
	Since               *string            `json:"since"`
	Window              string             `json:"window"`
	MinMarkets          int                `json:"min_markets"`
	MinGain             int                `json:"min_gain"`
	StorefrontsCompared []breakoutCompared `json:"storefronts_compared"`
	StorefrontsSkipped  []breakoutSkipped  `json:"storefronts_skipped"`
	BelowMinMarkets     int                `json:"below_min_markets"`
	TotalBreakouts      int                `json:"total_breakouts"`
	Limit               int                `json:"limit"`
	Truncated           bool               `json:"truncated"`
	EnteredGainRule     string             `json:"entered_gain_rule"`
	Note                string             `json:"note"`
}

type breakoutsView struct {
	Meta    breakoutsMeta `json:"meta"`
	Results []breakoutRow `json:"results"`
}

type breakoutsOpts struct {
	platform   string
	genre      appstore.Genre
	ageBand    string // Kids chart age band id; "" = the overall chart
	chart      string
	since      time.Time
	window     string
	minMarkets int
	minGain    int
	limit      int
	scope      s3bScope
}

func newNovelBreakoutsCmd(flags *rootFlags) *cobra.Command {
	var flagGenre, flagChart, flagPlatform, flagAgeBand, flagSince, flagCountries, flagDB string
	var flagMinMarkets, flagMinGain, flagLimit int

	cmd := &cobra.Command{
		Use:   "breakouts",
		Short: "Find apps climbing or entering the same chart in several countries at once",
		Long:  "Use this to find apps climbing or entering one chart in several storefronts at once over a window. Do NOT use this for one storefront's chart diff; use 'movers' instead, and for one app's rank trajectory use 'rank-history'. A Kids chart is its own series: --age-band reads it (as captured by 'top --age-band'); without it the overall chart is read.",
		Example: strings.Trim(`
  app-store-pp-cli breakouts --genre 6014 --chart top-free --since 7d --min-markets 3 --agent
  app-store-pp-cli breakouts --genre puzzle --countries us,gb,jp,de --min-gain 20 --json
  app-store-pp-cli breakouts --chart top-grossing --platform ipad --since 14d
  app-store-pp-cli breakouts --chart top-free --age-band 6-8 --since 14d --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--genre=6014;--chart=top-free;--since=7d;--min-markets=2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "breakouts")
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("breakouts takes no positional arguments (got %q); pick the chart with --genre, --chart and --platform", strings.Join(args, " ")))
			}
			opts := breakoutsOpts{minMarkets: flagMinMarkets, minGain: flagMinGain, limit: flagLimit, window: strings.TrimSpace(flagSince)}
			var err error
			if opts.genre, err = resolveGenre(flagGenre); err != nil {
				return err
			}
			if opts.chart, err = appstore.NormalizeChart(flagChart); err != nil {
				return usageErr(fmt.Errorf("--chart: %w", err))
			}
			if opts.platform, err = parseChartPlatform(flagPlatform); err != nil {
				return err
			}
			band, err := resolveAgeBand(flagAgeBand)
			if err != nil {
				return err
			}
			if err := checkChartAgeBand(band, opts.genre, opts.platform, opts.chart); err != nil {
				return err
			}
			opts.ageBand = band.ID
			if opts.since, err = s3bParseSince("since", flagSince, s3bNow()); err != nil {
				return err
			}
			if flagMinMarkets < 1 {
				return usageErr(fmt.Errorf("--min-markets must be at least 1, got %d", flagMinMarkets))
			}
			if flagMinGain < 1 {
				return usageErr(fmt.Errorf("--min-gain must be at least 1 rank, got %d", flagMinGain))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (no limit) or more, got %d", flagLimit))
			}
			if opts.scope, err = s3bStorefrontScope(cmd, flags, flagCountries); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			hint := fmt.Sprintf("capture the chart in several storefronts on two or more days first, e.g. '%s --country %s'", breakoutsTopCommand(opts), breakoutsHintCountry(opts))
			st, ok, err := s3bOpenStore(cmd, flags, hint, func(note string) any {
				meta := newBreakoutsMeta(opts)
				meta.Note = note
				return breakoutsView{Meta: meta, Results: make([]breakoutRow, 0)}
			})
			if err != nil || !ok {
				return err
			}
			defer func() { _ = st.Close() }()

			view, err := buildBreakouts(ctx, st, opts)
			if err != nil {
				return s3bLocalReadErr(err)
			}
			stamps := make([]string, 0, len(view.Meta.StorefrontsCompared))
			for _, c := range view.Meta.StorefrontsCompared {
				stamps = append(stamps, c.To)
			}
			warnStale(cmd, flags, s3bNewestOf(stamps...), s3bNow(), "chart capture", breakoutsTopCommand(opts)+" --country <cc>")
			return renderBreakouts(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagGenre, "genre", "", "Chart genre: an id (6014) or name (games, puzzle); default 36 (all apps)")
	cmd.Flags().StringVar(&flagChart, "chart", "top-free", "Chart: top-free, top-paid or top-grossing")
	cmd.Flags().StringVar(&flagPlatform, "platform", "iphone", "Chart platform: iphone, ipad or mac")
	cmd.Flags().StringVar(&flagAgeBand, "age-band", "", "Kids chart age band (with --genre 36): all (All Kids), 5-under, 6-8 or 9-11; unset = the overall chart")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window to compare: the first and last comparable capture inside it (7d, 2w, a date such as 2026-10-01, or all)")
	cmd.Flags().IntVar(&flagMinMarkets, "min-markets", 2, "Minimum storefronts where an app must enter or gain")
	cmd.Flags().IntVar(&flagMinGain, "min-gain", 10, "Minimum rank gain that counts as climbing in a storefront (entering always counts)")
	cmd.Flags().StringVar(&flagCountries, "countries", "", "Storefronts to compare (us,gb,jp or all); default every storefront with stored captures, or --country when set")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum apps to return (0 = no limit)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local snapshot store path (default: the CLI data directory)")
	return cmd
}

// breakoutsTopCommand is the live command that captures the compared chart
// in one storefront (append --country).
func breakoutsTopCommand(opts breakoutsOpts) string {
	cmd := fmt.Sprintf("app-store-pp-cli top --chart %s --genre %s --platform %s", opts.chart, opts.genre.ID, opts.platform)
	if opts.ageBand != "" {
		cmd += " --age-band " + opts.ageBand
	}
	return cmd
}

// breakoutsHintCountry is the storefront a capture hint names: the first one
// requested with --countries or --country, else gb (a second market beside
// the default us).
func breakoutsHintCountry(opts breakoutsOpts) string {
	if list := opts.scope.list(); len(list) > 0 {
		return list[0]
	}
	return "gb"
}

func newBreakoutsMeta(opts breakoutsOpts) breakoutsMeta {
	return breakoutsMeta{
		Platform:            opts.platform,
		GenreID:             opts.genre.ID,
		GenreName:           opts.genre.Name,
		AgeBand:             opts.ageBand,
		Chart:               opts.chart,
		Since:               s3bTimePtr(opts.since),
		Window:              opts.window,
		MinMarkets:          opts.minMarkets,
		MinGain:             opts.minGain,
		StorefrontsCompared: make([]breakoutCompared, 0),
		StorefrontsSkipped:  make([]breakoutSkipped, 0),
		Limit:               opts.limit,
		EnteredGainRule:     "an app that entered the chart counts as a gain of rowLimit+1-rank (from just outside the chart)",
	}
}

func buildBreakouts(ctx context.Context, st *store.Store, opts breakoutsOpts) (breakoutsView, error) {
	view := breakoutsView{Meta: newBreakoutsMeta(opts), Results: make([]breakoutRow, 0)}
	universe := opts.scope.list()
	explicit := universe != nil
	if !explicit {
		for _, sf := range appstore.Storefronts() {
			universe = append(universe, sf.Code)
		}
		sort.Strings(universe)
	}

	type market struct {
		sf        string
		base, end *store.ChartSnapshot
	}
	markets := make([]market, 0)
	for _, sf := range universe {
		key := store.ChartKey{Storefront: sf, Platform: opts.platform, GenreID: opts.genre.ID, AgeBand: opts.ageBand, Chart: opts.chart}
		caps, err := store.ChartCaptures(ctx, st, key)
		if err != nil {
			return view, err
		}
		if len(caps) == 0 {
			if explicit {
				view.Meta.StorefrontsSkipped = append(view.Meta.StorefrontsSkipped, breakoutSkipped{Storefront: sf, Reason: "no stored capture of this chart"})
			}
			continue
		}
		inWindow := make([]store.ChartCapture, 0, len(caps))
		for _, c := range caps {
			if opts.since.IsZero() || !c.CapturedAt.Before(opts.since) {
				inWindow = append(inWindow, c)
			}
		}
		if len(inWindow) < 2 {
			view.Meta.StorefrontsSkipped = append(view.Meta.StorefrontsSkipped, breakoutSkipped{Storefront: sf, Captures: len(inWindow),
				Reason: fmt.Sprintf("%d capture(s) in the window; needs 2", len(inWindow))})
			continue
		}
		last := inWindow[len(inWindow)-1]
		var base *store.ChartCapture
		for i := range inWindow {
			c := inWindow[i]
			if c.Source == last.Source && c.RowLimit == last.RowLimit && c.CapturedAt.Before(last.CapturedAt) {
				base = &c
				break
			}
		}
		if base == nil {
			view.Meta.StorefrontsSkipped = append(view.Meta.StorefrontsSkipped, breakoutSkipped{Storefront: sf, Captures: len(inWindow),
				Reason: "no earlier capture in the window with the same source and row limit as the latest"})
			continue
		}
		lk := key
		lk.Source = last.Source
		baseSnap, err := store.ChartSnapshotAt(ctx, st, lk, base.CapturedAt)
		if err != nil {
			return view, err
		}
		endSnap, err := store.ChartSnapshotAt(ctx, st, lk, last.CapturedAt)
		if err != nil {
			return view, err
		}
		if baseSnap == nil || endSnap == nil {
			view.Meta.StorefrontsSkipped = append(view.Meta.StorefrontsSkipped, breakoutSkipped{Storefront: sf, Captures: len(inWindow), Reason: "stored capture has no rows"})
			continue
		}
		view.Meta.StorefrontsCompared = append(view.Meta.StorefrontsCompared, breakoutCompared{Storefront: sf, From: s3bFormat(base.CapturedAt),
			To: s3bFormat(last.CapturedAt), Source: last.Source, RowLimit: last.RowLimit, Captures: len(inWindow)})
		markets = append(markets, market{sf: sf, base: baseSnap, end: endSnap})
	}

	type appAgg struct {
		title, developer string
		markets          []breakoutMarket
	}
	apps := map[string]*appAgg{}
	for _, m := range markets {
		baseRank := map[string]int{}
		for _, r := range m.base.Rows {
			if _, dup := baseRank[r.AppID]; !dup {
				baseRank[r.AppID] = r.Rank
			}
		}
		rowLimit := m.end.RowLimit
		if rowLimit <= 0 {
			rowLimit = len(m.end.Rows)
		}
		seen := map[string]bool{}
		for _, r := range m.end.Rows {
			if seen[r.AppID] {
				continue
			}
			seen[r.AppID] = true
			from, was := baseRank[r.AppID]
			bm := breakoutMarket{Storefront: m.sf, ToRank: r.Rank, Entered: !was, From: s3bFormat(m.base.CapturedAt), To: s3bFormat(m.end.CapturedAt)}
			if was {
				f := from
				bm.FromRank = &f
				bm.Gain = from - r.Rank
			} else {
				bm.Gain = rowLimit + 1 - r.Rank
			}
			if !bm.Entered && bm.Gain < opts.minGain {
				continue
			}
			a := apps[r.AppID]
			if a == nil {
				a = &appAgg{}
				apps[r.AppID] = a
			}
			if a.title == "" {
				a.title = r.Title
			}
			if a.developer == "" {
				a.developer = r.DeveloperName
			}
			a.markets = append(a.markets, bm)
		}
	}

	rows := make([]breakoutRow, 0)
	for id, a := range apps {
		if len(a.markets) < opts.minMarkets {
			view.Meta.BelowMinMarkets++
			continue
		}
		gains := make([]float64, 0, len(a.markets))
		best := 0
		for _, m := range a.markets {
			gains = append(gains, float64(m.Gain))
			if best == 0 || m.ToRank < best {
				best = m.ToRank
			}
		}
		sort.SliceStable(a.markets, func(i, j int) bool { return a.markets[i].Storefront < a.markets[j].Storefront })
		rows = append(rows, breakoutRow{AppID: id, Title: a.title, DeveloperName: a.developer, Markets: len(a.markets),
			MedianGain: s3bRound2(s3bMedian(gains)), BestRank: best, Storefronts: a.markets})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Markets != b.Markets {
			return a.Markets > b.Markets
		}
		if a.MedianGain != b.MedianGain {
			return a.MedianGain > b.MedianGain
		}
		if a.BestRank != b.BestRank {
			return a.BestRank < b.BestRank
		}
		return a.AppID < b.AppID
	})
	view.Meta.TotalBreakouts = len(rows)
	if opts.limit > 0 && len(rows) > opts.limit {
		rows = rows[:opts.limit]
		view.Meta.Truncated = true
	}
	view.Results = rows

	chart := s3bChartSeriesName(opts.platform, opts.genre.ID, opts.ageBand, opts.chart)
	window := "across all stored captures"
	if !opts.since.IsZero() {
		window = "since " + s3bFormat(opts.since)
	}
	compared := len(view.Meta.StorefrontsCompared)
	switch {
	case compared == 0:
		view.Meta.Note = fmt.Sprintf("No storefront has two comparable captures of %s %s; capture it with '%s --country %s' on two or more days in each storefront (one request per storefront).",
			chart, window, breakoutsTopCommand(opts), breakoutsHintCountry(opts))
	case compared < opts.minMarkets:
		view.Meta.Note = fmt.Sprintf("Only %d storefront(s) have two comparable captures of %s %s, fewer than --min-markets %d, so no app can qualify; capture more storefronts or lower --min-markets.",
			compared, chart, window, opts.minMarkets)
	case len(rows) == 0:
		view.Meta.Note = fmt.Sprintf("Compared %d storefront(s) of %s %s; no app entered or gained %d+ ranks in %d or more of them (%d app(s) did so in fewer storefronts).",
			compared, chart, window, opts.minGain, opts.minMarkets, view.Meta.BelowMinMarkets)
	default:
		view.Meta.Note = fmt.Sprintf("Apps that entered or gained %d+ ranks in at least %d of %d compared storefront(s) of %s %s, ranked by storefront count then median gain; %d app(s) did so in fewer storefronts and are excluded.",
			opts.minGain, opts.minMarkets, compared, chart, window, view.Meta.BelowMinMarkets)
		if view.Meta.Truncated {
			view.Meta.Note += fmt.Sprintf(" Showing the first %d; raise --limit to see all.", opts.limit)
		}
	}
	return view, nil
}

func renderBreakouts(cmd *cobra.Command, flags *rootFlags, view breakoutsView) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if len(view.Results) == 0 {
		_, _ = fmt.Fprintln(w, view.Meta.Note)
		return nil
	}
	rows := make([][]string, 0, len(view.Results))
	for _, r := range view.Results {
		moves := make([]string, 0, len(r.Storefronts))
		for _, m := range r.Storefronts {
			if m.Entered {
				moves = append(moves, fmt.Sprintf("%s new->%d", m.Storefront, m.ToRank))
			} else {
				moves = append(moves, fmt.Sprintf("%s %d->%d", m.Storefront, *m.FromRank, m.ToRank))
			}
		}
		rows = append(rows, []string{r.AppID, s3bTruncate(r.Title, 32), fmt.Sprint(r.Markets), fmt.Sprintf("%g", r.MedianGain), fmt.Sprint(r.BestRank), s3bTruncate(strings.Join(moves, ", "), 70)})
	}
	if err := s3bPrintTable(w, []string{"app", "title", "markets", "median gain", "best rank", "moves"}, rows); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(w, "\n"+view.Meta.Note)
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelBreakoutsCmd(flags))
	})
}
