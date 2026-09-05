package cli

// pp:data-source local

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// The four leverage markets, their store keys and the field holding net open value.
// Each uses a different response envelope upstream (positions / positions / positions
// / rows), which is why they can only be compared once normalised locally.
var nccplLeverageMarkets = []struct {
	Resource string
	Label    string
	VolField []string
	ValField []string
}{
	{"mts", "MTS", []string{"net_open_mts_volume"}, []string{"net_open_mts_amount"}},
	{"mfs", "MFS", []string{"net_open_position_volume"}, []string{"net_open_position_value"}},
	{"msf", "MSF", []string{"msf_currentday_vol"}, []string{"msf_currentday_val"}},
	{"slb", "SLB", []string{"net_open_position_volume"}, []string{"net_open_position_value"}},
}

type nccplLeverageRow struct {
	Date         string             `json:"date"`
	Symbol       string             `json:"symbol"`
	Volumes      map[string]float64 `json:"open_volume"`
	Values       map[string]float64 `json:"open_value"`
	TotalValue   float64            `json:"total_open_value"`
	MarketsSeen  []string           `json:"markets"`
	ForceRelease bool               `json:"force_release_that_day"`
}

type nccplLeverageView struct {
	Rows          []nccplLeverageRow `json:"rows"`
	SymbolsSeen   int                `json:"symbols"`
	DatesSpanned  int                `json:"dates_spanned"`
	MarketsLoaded []string           `json:"markets_loaded"`
	MarketsEmpty  []string           `json:"markets_absent_from_store,omitempty"`
	Note          string             `json:"note,omitempty"`
}

func newNCCPLLeverageCmd(flags *rootFlags) *cobra.Command {
	var (
		fromDate string
		toDate   string
		symbol   string
		limit    int
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "leverage",
		Short: "Join MTS, MFS, MSF and SLB open positions into one per-symbol cross-section",
		Long: strings.Trim(`
Combine the four leverage markets NCCPL publishes separately into a single panel keyed
(date, symbol), with force-release events flagged against the day they landed.

No API call performs this join: the four endpoints return three different response
envelopes and are only comparable once normalised locally. SLB net open position is
the closest thing this market has to published short interest, so pairing it with MTS
and MFS open interest shows leverage building in a name before a forced release.

Markets absent from the local store are named explicitly rather than silently treated
as zero, so a partial sync cannot masquerade as an unlevered symbol.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli leverage --from 2026-08-01 --to 2026-09-04
  nccpl-pp-cli leverage --symbol OGDC --json
  nccpl-pp-cli leverage --from 2026-09-01 --to 2026-09-04 --agent --select rows.symbol,rows.total_open_value
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--from=2026-08-01;--to=2026-09-04",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "leverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			view := nccplLeverageView{
				Rows:          make([]nccplLeverageRow, 0),
				MarketsLoaded: make([]string, 0),
				MarketsEmpty:  make([]string, 0),
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: nccpl-pp-cli sync --resources mts,mfs,msf,slb --latest-only --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			ready, err := store.NCCPLSchemaReady(ctx, db)
			if err != nil {
				return err
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\nrun: nccpl-pp-cli sync --resources mts,mfs,msf,slb --latest-only --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			type cell struct {
				vol map[string]float64
				val map[string]float64
			}
			grid := map[string]map[string]*cell{} // date -> symbol -> cell
			dates := map[string]bool{}

			for _, m := range nccplLeverageMarkets {
				res, ok := nccplResourceByName(m.Resource)
				if !ok {
					continue
				}
				obs, err := store.NCCPLObservations(ctx, db, m.Resource, fromDate, toDate)
				if err != nil {
					return err
				}
				if len(obs) == 0 {
					view.MarketsEmpty = append(view.MarketsEmpty, m.Label)
					continue
				}
				view.MarketsLoaded = append(view.MarketsLoaded, m.Label)
				for _, o := range obs {
					row, err := nccplDecodePayload(o.Payload)
					if err != nil {
						continue
					}
					sym := nccplSymbolOf(res, row)
					if sym == "" || (symbol != "" && !strings.EqualFold(sym, symbol)) {
						continue
					}
					dates[o.Date] = true
					if grid[o.Date] == nil {
						grid[o.Date] = map[string]*cell{}
					}
					c := grid[o.Date][sym]
					if c == nil {
						c = &cell{vol: map[string]float64{}, val: map[string]float64{}}
						grid[o.Date][sym] = c
					}
					if v, ok := nccplNum(row, m.VolField...); ok {
						c.vol[m.Label] = v
					}
					if v, ok := nccplNum(row, m.ValField...); ok {
						c.val[m.Label] = v
					}
				}
			}

			// Force-release days, drained separately after the market loop.
			forceDays := map[string]bool{}
			if fr, err := store.NCCPLObservations(ctx, db, "mts-force-release", fromDate, toDate); err == nil {
				for _, o := range fr {
					forceDays[o.Date] = true
				}
			}

			dateList := make([]string, 0, len(dates))
			for d := range dates {
				dateList = append(dateList, d)
			}
			sort.Strings(dateList)

			symbolsSeen := map[string]bool{}
			for _, d := range dateList {
				syms := make([]string, 0, len(grid[d]))
				for s := range grid[d] {
					syms = append(syms, s)
				}
				sort.Strings(syms)
				for _, s := range syms {
					c := grid[d][s]
					total := 0.0
					markets := make([]string, 0, len(c.val))
					for label, v := range c.val {
						total += v
						markets = append(markets, label)
					}
					sort.Strings(markets)
					symbolsSeen[s] = true
					view.Rows = append(view.Rows, nccplLeverageRow{
						Date: d, Symbol: s, Volumes: c.vol, Values: c.val,
						TotalValue: total, MarketsSeen: markets, ForceRelease: forceDays[d],
					})
				}
			}
			sort.Slice(view.Rows, func(i, j int) bool {
				if view.Rows[i].Date != view.Rows[j].Date {
					return view.Rows[i].Date > view.Rows[j].Date
				}
				return view.Rows[i].TotalValue > view.Rows[j].TotalValue
			})
			if limit > 0 && len(view.Rows) > limit {
				view.Rows = view.Rows[:limit]
			}
			view.SymbolsSeen = len(symbolsSeen)
			view.DatesSpanned = len(dateList)

			switch {
			case len(view.MarketsLoaded) == 0:
				view.Note = "no leverage rows stored in this range; run 'sync --resources mts,mfs,msf,slb'"
			case len(view.MarketsEmpty) > 0:
				view.Note = fmt.Sprintf("markets absent from the local store for this range: %s -- their columns are missing, not zero",
					strings.Join(view.MarketsEmpty, ", "))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-14s %18s  %-18s %s\n", "DATE", "SYMBOL", "TOTAL OPEN VALUE", "MARKETS", "FORCE-RELEASE DAY")
			for _, r := range view.Rows {
				fr := ""
				if r.ForceRelease {
					fr = "yes"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-14s %18.2f  %-18s %s\n",
					r.Date, r.Symbol, r.TotalValue, strings.Join(r.MarketsSeen, ","), fr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d row(s); %d symbol(s) across %d date(s); markets loaded: %s\n",
				len(view.Rows), view.SymbolsSeen, view.DatesSpanned, strings.Join(view.MarketsLoaded, ","))
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Restrict to one instrument code")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return, largest total open value first (0 = no cap)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
