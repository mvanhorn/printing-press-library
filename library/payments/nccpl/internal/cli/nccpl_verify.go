package cli

// pp:data-source local

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// Arithmetic invariants NCCPL publishes for free.
//
// 1. FIPI net == -LIPI net on every date. Every foreign buy is a local sell, so the
//    two sides must cancel exactly.
// 2. Each sector's rows net to zero across the investor classes, for the same reason
//    applied within a sector.
//
// Neither is checkable from a single endpoint: both need the two sides held together
// locally. A date that fails one is corrupt input and must not reach a regression.

type nccplInvariantFailure struct {
	Date      string  `json:"date"`
	Check     string  `json:"check"`
	Scope     string  `json:"scope,omitempty"`
	FIPINet   float64 `json:"fipi_net,omitempty"`
	LIPINet   float64 `json:"lipi_net,omitempty"`
	Residual  float64 `json:"residual"`
	Tolerance float64 `json:"tolerance"`
}

type nccplVerifyView struct {
	DatesChecked int                     `json:"dates_checked"`
	DatesPassed  int                     `json:"dates_passed"`
	DatesFailed  int                     `json:"dates_failed"`
	DatesSkipped int                     `json:"dates_skipped_missing_side"`
	Failures     []nccplInvariantFailure `json:"failures"`
	Quarantine   []string                `json:"quarantine"`
	Tolerance    float64                 `json:"tolerance"`
	Note         string                  `json:"note,omitempty"`
	DBPath       string                  `json:"db_path"`
}

func nccplNum(row map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := row[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			return t, true
		case string:
			s := strings.ReplaceAll(strings.TrimSpace(t), ",", "")
			if s == "" {
				continue
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func newNCCPLVerifyCmd(flags *rootFlags) *cobra.Command {
	var (
		fromDate  string
		toDate    string
		tolerance float64
		dbPath    string
		exitCode  bool
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check stored dates against NCCPL's own arithmetic identities",
		Long: strings.Trim(`
Assert the two identities NCCPL's data must satisfy and quarantine the dates that fail.

  FIPI net == -LIPI net      every foreign buy is a local sell
  sector rows net to zero    the same identity applied within each sector

A failing date is corrupt input, not a weak signal: it should be re-synced or excluded
before it reaches a model. Dates where only one side is stored are reported as skipped
rather than failed, because a missing counterpart is a coverage problem, not an
arithmetic one -- run 'coverage' for those.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli verify
  nccpl-pp-cli verify --from 2026-08-01 --to 2026-09-04 --json
  nccpl-pp-cli verify --tolerance 1.0 --exit-code
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--from=2026-08-01;--to=2026-09-04",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			view := nccplVerifyView{
				Failures:   make([]nccplInvariantFailure, 0),
				Quarantine: make([]string, 0),
				Tolerance:  tolerance,
				DBPath:     dbPath,
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: nccpl-pp-cli sync --resources fipi,lipi --latest-only --db %s\n", dbPath, dbPath)
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
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\nrun: nccpl-pp-cli sync --resources fipi,lipi --latest-only --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			// Drain each resource fully before the next query: SQLite runs on a
			// single connection, so a follow-up query while rows are open deadlocks.
			fipi, err := store.NCCPLObservations(ctx, db, "fipi", fromDate, toDate)
			if err != nil {
				return err
			}
			lipi, err := store.NCCPLObservations(ctx, db, "lipi", fromDate, toDate)
			if err != nil {
				return err
			}
			fipiSector, err := store.NCCPLObservations(ctx, db, "fipi-sector", fromDate, toDate)
			if err != nil {
				return err
			}
			lipiSector, err := store.NCCPLObservations(ctx, db, "lipi-sector", fromDate, toDate)
			if err != nil {
				return err
			}

			flows, err := store.NCCPLObservations(ctx, db, "flows", fromDate, toDate)
			if err != nil {
				return err
			}

			fipiNet := nccplSumNetByDate(fipi)
			lipiNet := nccplSumNetByDate(lipi)
			bad := map[string]bool{}
			sectorNet := nccplSumNetByDateSector(append(append([]store.NCCPLObs{}, fipiSector...), lipiSector...))

			// The scstrade-sourced `flows` resource carries both sides in one row set,
			// so it can be checked on its own. Provenance drift shows up here as a
			// failing identity rather than a plausible-looking wrong number.
			flowsSector, flowsFIPI, flowsLIPI := nccplSumFlowsResource(flows)
			for d := range flowsSector {
				for s, v := range flowsSector[d] {
					if math.Abs(v) > tolerance {
						view.Failures = append(view.Failures, nccplInvariantFailure{
							Date: d, Check: "flows sector nets to zero", Scope: s,
							Residual: v, Tolerance: tolerance,
						})
						bad[d] = true
					}
				}
			}
			for d, f := range flowsFIPI {
				l := flowsLIPI[d]
				if math.Abs(f+l) > tolerance {
					view.Failures = append(view.Failures, nccplInvariantFailure{
						Date: d, Check: "flows FIPI + LIPI == 0",
						FIPINet: f, LIPINet: l, Residual: f + l, Tolerance: tolerance,
					})
					bad[d] = true
				}
				view.DatesChecked++
			}

			dates := make([]string, 0)
			seen := map[string]bool{}
			for d := range fipiNet {
				if !seen[d] {
					seen[d] = true
					dates = append(dates, d)
				}
			}
			for d := range lipiNet {
				if !seen[d] {
					seen[d] = true
					dates = append(dates, d)
				}
			}
			sort.Strings(dates)

			for _, d := range dates {
				view.DatesChecked++
				f, haveF := fipiNet[d]
				l, haveL := lipiNet[d]
				if !haveF || !haveL {
					view.DatesSkipped++
					continue
				}
				residual := f + l
				if math.Abs(residual) > tolerance {
					view.Failures = append(view.Failures, nccplInvariantFailure{
						Date: d, Check: "fipi_net + lipi_net == 0",
						FIPINet: f, LIPINet: l, Residual: residual, Tolerance: tolerance,
					})
					bad[d] = true
				}
			}

			sectorDates := make([]string, 0, len(sectorNet))
			for d := range sectorNet {
				sectorDates = append(sectorDates, d)
			}
			sort.Strings(sectorDates)
			for _, d := range sectorDates {
				sectors := sectorNet[d]
				names := make([]string, 0, len(sectors))
				for s := range sectors {
					names = append(names, s)
				}
				sort.Strings(names)
				for _, s := range names {
					if math.Abs(sectors[s]) > tolerance {
						view.Failures = append(view.Failures, nccplInvariantFailure{
							Date: d, Check: "sector rows net to zero", Scope: s,
							Residual: sectors[s], Tolerance: tolerance,
						})
						bad[d] = true
					}
				}
			}

			for d := range bad {
				view.Quarantine = append(view.Quarantine, d)
			}
			sort.Strings(view.Quarantine)
			view.DatesFailed = len(view.Quarantine)
			view.DatesPassed = view.DatesChecked - view.DatesFailed - view.DatesSkipped
			if view.DatesChecked == 0 {
				view.Note = "no fipi/lipi dates stored in range; run sync first"
			} else if view.DatesSkipped > 0 {
				view.Note = fmt.Sprintf("%d date(s) had only one of fipi/lipi stored and could not be checked; run coverage", view.DatesSkipped)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "checked %d date(s): %d passed, %d failed, %d skipped (tolerance %g)\n",
					view.DatesChecked, view.DatesPassed, view.DatesFailed, view.DatesSkipped, tolerance)
				for _, f := range view.Failures {
					if f.Scope != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-26s %-28s residual=%.4f\n", f.Date, f.Check, f.Scope, f.Residual)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-26s fipi=%.2f lipi=%.2f residual=%.4f\n", f.Date, f.Check, f.FIPINet, f.LIPINet, f.Residual)
					}
				}
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
				}
			}

			if exitCode && view.DatesFailed > 0 {
				return notFoundErr(fmt.Errorf("%d date(s) failed an invariant", view.DatesFailed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date to check (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date to check (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&tolerance, "tolerance", 1.0, "Absolute residual tolerated before a date is quarantined")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "Exit 3 when any date fails an invariant, for pipeline gating")
	return cmd
}

func nccplSumNetByDate(obs []store.NCCPLObs) map[string]float64 {
	out := map[string]float64{}
	for _, o := range obs {
		row, err := nccplDecodePayload(o.Payload)
		if err != nil {
			continue
		}
		// The "net" client_type rows are pre-summed totals; including them
		// alongside the per-class rows would double-count the whole side.
		if ct, ok := row["client_type"]; ok {
			s := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", ct)))
			if s == "FN" || s == "LN" || strings.Contains(s, "NET") {
				continue
			}
		}
		if v, ok := nccplNum(row, "net_value", "NET_VALUE"); ok {
			out[o.Date] += v
		}
	}
	return out
}

func nccplSumNetByDateSector(obs []store.NCCPLObs) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for _, o := range obs {
		row, err := nccplDecodePayload(o.Payload)
		if err != nil {
			continue
		}
		if ct, ok := row["CLIENT_TYPE"]; ok {
			s := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", ct)))
			if s == "FN" || s == "LN" || strings.Contains(s, "NET") {
				continue
			}
		}
		sector := ""
		if v, ok := row["SECTOR_NAME"]; ok {
			sector = strings.TrimSpace(fmt.Sprintf("%v", v))
		}
		if sector == "" {
			continue
		}
		if v, ok := nccplNum(row, "NET_VALUE", "net_value"); ok {
			if out[o.Date] == nil {
				out[o.Date] = map[string]float64{}
			}
			out[o.Date][sector] += v
		}
	}
	return out
}

// nccplSumFlowsResource folds the scstrade-sourced `flows` rows into the two identities.
//
// Rows carry FLSectorName, FLTypeNew (either "FIPI" or a local investor class) and a
// signed FLNetValueUSD. Each sector must net to zero across all classes, and the FIPI
// side must be the exact negative of the summed local side.
func nccplSumFlowsResource(obs []store.NCCPLObs) (map[string]map[string]float64, map[string]float64, map[string]float64) {
	sector := map[string]map[string]float64{}
	fipi := map[string]float64{}
	lipi := map[string]float64{}
	for _, o := range obs {
		row, err := nccplDecodePayload(o.Payload)
		if err != nil {
			continue
		}
		v, ok := nccplNum(row, "FLNetValueUSD", "NET_VALUE", "net_value")
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", row["FLSectorName"]))
		if name != "" && name != "<nil>" {
			if sector[o.Date] == nil {
				sector[o.Date] = map[string]float64{}
			}
			sector[o.Date][name] += v
		}
		if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", row["FLTypeNew"])), "FIPI") {
			fipi[o.Date] += v
		} else {
			lipi[o.Date] += v
		}
	}
	return sector, fipi, lipi
}
