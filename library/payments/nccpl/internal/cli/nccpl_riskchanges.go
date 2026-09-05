package cli

// pp:data-source local

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

var nccplRiskFields = []string{"free_float", "var_value", "hair_cut", "half_hour_avg_rate"}

type nccplRiskChange struct {
	Symbol   string  `json:"symbol"`
	Field    string  `json:"field"`
	FromDate string  `json:"from_date"`
	ToDate   string  `json:"to_date"`
	From     float64 `json:"from_value"`
	To       float64 `json:"to_value"`
	Delta    float64 `json:"delta"`
	PctDelta float64 `json:"pct_delta,omitempty"`
}

type nccplRiskChangesView struct {
	Changes        []nccplRiskChange `json:"changes"`
	SymbolsScanned int               `json:"symbols_scanned"`
	SymbolsChanged int               `json:"symbols_changed"`
	DatesSpanned   int               `json:"dates_spanned"`
	FirstDate      string            `json:"first_date,omitempty"`
	LastDate       string            `json:"last_date,omitempty"`
	Note           string            `json:"note,omitempty"`
}

func newNCCPLRiskChangesCmd(flags *rootFlags) *cobra.Command {
	var (
		since    string
		fromDate string
		toDate   string
		field    string
		minPct   float64
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "risk-changes",
		Short: "Date every step change in a symbol's free float, VAR margin or haircut",
		Long: strings.Trim(`
Diff consecutive stored var-margins rows per symbol and report each field that moved,
with the dates that bracket the move.

A single-date endpoint cannot express a change, so this only exists because the values
are held locally across dates. free float in particular is the input a cap-weighted
cross-section needs and is not published daily by any other public Pakistani source.

Symbols scanned is always reported alongside symbols changed, so an empty result is
distinguishable from an unscanned universe.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli risk-changes --since 30d
  nccpl-pp-cli risk-changes --since 90d --field free_float --json
  nccpl-pp-cli risk-changes --from 2026-08-01 --to 2026-09-04 --min-pct 5
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--since=30d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "risk-changes")
			}
			if field != "" {
				known := false
				for _, f := range nccplRiskFields {
					if f == field {
						known = true
						break
					}
				}
				if !known {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("unknown --field %q; valid: %s", field, strings.Join(nccplRiskFields, ", ")))
				}
			}
			if since != "" && fromDate == "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				fromDate = time.Now().Add(-d).Format("2006-01-02")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			view := nccplRiskChangesView{Changes: make([]nccplRiskChange, 0)}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: nccpl-pp-cli sync --resources var-margins --latest-only --db %s\n", dbPath, dbPath)
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
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\nrun: nccpl-pp-cli sync --resources var-margins --latest-only --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			res, _ := nccplResourceByName("var-margins")
			obs, err := store.NCCPLObservations(ctx, db, "var-margins", fromDate, toDate)
			if err != nil {
				return err
			}

			type point struct {
				date string
				vals map[string]float64
			}
			bySymbol := map[string][]point{}
			dates := map[string]bool{}
			for _, o := range obs {
				row, err := nccplDecodePayload(o.Payload)
				if err != nil {
					continue
				}
				sym := nccplSymbolOf(res, row)
				if sym == "" {
					continue
				}
				dates[o.Date] = true
				vals := map[string]float64{}
				for _, f := range nccplRiskFields {
					if field != "" && f != field {
						continue
					}
					if v, ok := nccplNum(row, f); ok {
						vals[f] = v
					}
				}
				bySymbol[sym] = append(bySymbol[sym], point{date: o.Date, vals: vals})
			}

			syms := make([]string, 0, len(bySymbol))
			for s := range bySymbol {
				syms = append(syms, s)
			}
			sort.Strings(syms)

			changed := map[string]bool{}
			for _, s := range syms {
				pts := bySymbol[s]
				sort.Slice(pts, func(i, j int) bool { return pts[i].date < pts[j].date })
				for i := 1; i < len(pts); i++ {
					prev, cur := pts[i-1], pts[i]
					fields := make([]string, 0, len(cur.vals))
					for f := range cur.vals {
						fields = append(fields, f)
					}
					sort.Strings(fields)
					for _, f := range fields {
						pv, ok := prev.vals[f]
						if !ok {
							continue
						}
						cv := cur.vals[f]
						if pv == cv {
							continue
						}
						delta := cv - pv
						pct := 0.0
						if pv != 0 {
							pct = delta / math.Abs(pv) * 100
						}
						if minPct > 0 && math.Abs(pct) < minPct {
							continue
						}
						view.Changes = append(view.Changes, nccplRiskChange{
							Symbol: s, Field: f, FromDate: prev.date, ToDate: cur.date,
							From: pv, To: cv, Delta: delta, PctDelta: pct,
						})
						changed[s] = true
					}
				}
			}

			view.SymbolsScanned = len(bySymbol)
			view.SymbolsChanged = len(changed)
			view.DatesSpanned = len(dates)
			ds := make([]string, 0, len(dates))
			for d := range dates {
				ds = append(ds, d)
			}
			sort.Strings(ds)
			if len(ds) > 0 {
				view.FirstDate, view.LastDate = ds[0], ds[len(ds)-1]
			}
			switch {
			case view.SymbolsScanned == 0:
				view.Note = "no var-margins rows stored in this range; run 'sync --resources var-margins'"
			case view.DatesSpanned < 2:
				view.Note = fmt.Sprintf("only %d date stored; a change needs at least two dates to compare", view.DatesSpanned)
			case len(view.Changes) == 0:
				view.Note = fmt.Sprintf("scanned %d symbol(s) across %d date(s); nothing moved", view.SymbolsScanned, view.DatesSpanned)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, c := range view.Changes {
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-18s %s -> %s  %14.4f -> %14.4f  (%+.2f%%)\n",
					c.Symbol, c.Field, c.FromDate, c.ToDate, c.From, c.To, c.PctDelta)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nscanned %d symbol(s) across %d date(s); %d changed\n",
				view.SymbolsScanned, view.DatesSpanned, view.SymbolsChanged)
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Look back this far from today (e.g. 30d, 4w, 24h)")
	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date (YYYY-MM-DD); overrides --since")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&field, "field", "", "Restrict to one field ("+strings.Join(nccplRiskFields, ", ")+")")
	cmd.Flags().Float64Var(&minPct, "min-pct", 0, "Only report changes of at least this percent magnitude")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
