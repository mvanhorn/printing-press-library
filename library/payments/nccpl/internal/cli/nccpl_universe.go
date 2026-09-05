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

type nccplUniverseSymbol struct {
	Symbol    string   `json:"symbol"`
	FirstSeen string   `json:"first_seen"`
	LastSeen  string   `json:"last_seen"`
	Dates     int      `json:"dates_present"`
	Sources   []string `json:"sources"`
}

type nccplUniverseView struct {
	On           string                `json:"on,omitempty"`
	From         string                `json:"from,omitempty"`
	To           string                `json:"to,omitempty"`
	Symbols      []nccplUniverseSymbol `json:"symbols"`
	Width        int                   `json:"universe_width"`
	DatesSpanned int                   `json:"dates_spanned"`
	Entered      []string              `json:"entered,omitempty"`
	Exited       []string              `json:"exited,omitempty"`
	Note         string                `json:"note,omitempty"`
}

func newNCCPLUniverseCmd(flags *rootFlags) *cobra.Command {
	var (
		on       string
		fromDate string
		toDate   string
		changes  bool
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "universe",
		Short: "Reconstruct which symbols were listed and clearing-eligible on a past date",
		Long: strings.Trim(`
Derive the point-in-time instrument universe from the symbols NCCPL actually published
risk parameters and settlement records for on each date.

This is a clearing-house liveness signal, not a price-staleness one. A symbol appears
here because the clearing house margined or settled it that day, so it can disagree
with a filter based on how recently a price printed -- which is what makes it usable
as a control rather than a duplicate of one.

Scope limit: this yields the listed-and-eligible universe, not historical index
membership. It cannot tell you what was in the KSE-100 on a given day.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli universe --on 2026-09-04
  nccpl-pp-cli universe --from 2026-08-01 --to 2026-09-04 --changes --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--on=2026-09-04",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "universe")
			}
			if on == "" && (fromDate == "" || toDate == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give --on, or both --from and --to"))
			}
			if on != "" {
				fromDate, toDate = on, on
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			view := nccplUniverseView{On: on, From: fromDate, To: toDate, Symbols: make([]nccplUniverseSymbol, 0)}
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

			type agg struct {
				first, last string
				dates       map[string]bool
				sources     map[string]bool
			}
			symbols := map[string]*agg{}
			allDates := map[string]bool{}

			// Drain each resource fully before querying the next one.
			for _, r := range nccplPerSymbolResources() {
				obs, err := store.NCCPLObservations(ctx, db, r.Name, fromDate, toDate)
				if err != nil {
					return err
				}
				for _, o := range obs {
					row, err := nccplDecodePayload(o.Payload)
					if err != nil {
						continue
					}
					sym := nccplSymbolOf(r, row)
					if sym == "" {
						continue
					}
					allDates[o.Date] = true
					a, ok := symbols[sym]
					if !ok {
						a = &agg{first: o.Date, last: o.Date, dates: map[string]bool{}, sources: map[string]bool{}}
						symbols[sym] = a
					}
					if o.Date < a.first {
						a.first = o.Date
					}
					if o.Date > a.last {
						a.last = o.Date
					}
					a.dates[o.Date] = true
					a.sources[r.Name] = true
				}
			}

			names := make([]string, 0, len(symbols))
			for s := range symbols {
				names = append(names, s)
			}
			sort.Strings(names)
			for _, s := range names {
				a := symbols[s]
				src := make([]string, 0, len(a.sources))
				for k := range a.sources {
					src = append(src, k)
				}
				sort.Strings(src)
				view.Symbols = append(view.Symbols, nccplUniverseSymbol{
					Symbol: s, FirstSeen: a.first, LastSeen: a.last, Dates: len(a.dates), Sources: src,
				})
			}
			view.Width = len(view.Symbols)
			view.DatesSpanned = len(allDates)

			if changes && view.DatesSpanned > 1 {
				view.Entered = make([]string, 0)
				view.Exited = make([]string, 0)
				for _, s := range view.Symbols {
					if s.FirstSeen > fromDate {
						view.Entered = append(view.Entered, s.Symbol)
					}
					if s.LastSeen < toDate {
						view.Exited = append(view.Exited, s.Symbol)
					}
				}
			}
			if view.Width == 0 {
				view.Note = "no per-symbol rows stored in this range; run 'sync --resources var-margins,settlement-uin'"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, s := range view.Symbols {
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-12s %-12s %-5d %s\n", s.Symbol, s.FirstSeen, s.LastSeen, s.Dates, strings.Join(s.Sources, ","))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nuniverse width: %d symbol(s) across %d date(s)\n", view.Width, view.DatesSpanned)
			if len(view.Entered) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "entered: %s\n", strings.Join(view.Entered, ", "))
			}
			if len(view.Exited) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "exited:  %s\n", strings.Join(view.Exited, ", "))
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&on, "on", "", "Single settlement date to reconstruct (YYYY-MM-DD)")
	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&changes, "changes", false, "Also report symbols that entered or left the universe within the range")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
