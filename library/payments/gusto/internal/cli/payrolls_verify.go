// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/gusto"
)

func newNovelPayrollsVerifyCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var flagFrom string
	var flagRate string
	var flagStdin bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check the draft payroll against your timesheet and exit non-zero on any mismatch",
		Long: "Check the draft payroll against the source timesheet CSV: regular hours per employee, each\n" +
			"expected reimbursement line and amount, lines the timesheet does not explain, and duplicate\n" +
			"descriptions. Exits 0 when everything matches and 1 on any mismatch, so an agent can gate\n" +
			"the human handoff on the exit code. Read-only.\n\n" +
			"Use this command to prove the current draft matches the source timesheet before a human\n" +
			"submits. Do NOT use it to write changes; use 'payrolls prep' instead. Do NOT use it to\n" +
			"compare against a previous period; use 'payrolls compare' instead.",
		Example:     "  # timesheet as a JSON array on stdin\n  gusto-pp-cli payrolls verify --period-end 2026-11-01 --stdin --rate 0.60 --agent\n  # timesheet as a CSV file\n  gusto-pp-cli payrolls verify --period-end 2026-11-01 --from week.csv --rate 0.60 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "mcp:read-flags": "from", "pp:typed-exit-codes": "0,1", "pp:verified-by-proof": "payrolls-write-lifecycle.md", "pp:happy-args": "--rate=0.60", "pp:happy-stdin": `[{"employee":"example-value","hours":0}]`},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls verify")
			}
			if err := timesheetSourceErr(flagFrom, flagStdin); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			rows, rate, err := loadTimesheet(flagFrom, flagRate, flagStdin)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			api, err := newGustoAPI(flags)
			if err != nil {
				return err
			}
			d, err := resolveDraft(ctx, api, t)
			if err != nil {
				return gustoErr(err)
			}
			mismatches := gusto.Compare(rows, d, rate)
			view := map[string]any{"ok": len(mismatches) == 0, "draft_id": d.ID, "period_end": d.PeriodEnd, "submit_by": d.RunBy, "employees_checked": len(rows), "mismatches": mismatches}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if len(mismatches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "OK: draft %s matches the timesheet for %d employee(s).\n", d.PeriodEnd, len(rows))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "MISMATCH: %d difference(s) on draft %s\n", len(mismatches), d.PeriodEnd)
				for _, m := range mismatches {
					if m.Detail != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s: want %s, got %s (%s)\n", m.Employee, m.Field, m.Want, m.Got, m.Detail)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s: want %s, got %s\n", m.Employee, m.Field, m.Want, m.Got)
					}
				}
			}
			if len(mismatches) > 0 {
				return &cliError{code: 1, err: fmt.Errorf("draft does not match the timesheet (%d difference(s))", len(mismatches))}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().StringVar(&flagFrom, "from", "", "Timesheet CSV with employee, hours, miles, reimbursement, description columns")
	cmd.Flags().StringVar(&flagRate, "rate", "", "Mileage rate in dollars per mile used to build the expected mileage line")
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, timesheetStdinFlagHelp)
	return cmd
}

// errOpenTimesheet marks a --from file that could not be opened, which prep
// tolerates under --dry-run.
var errOpenTimesheet = errors.New("opening --from")

// loadTimesheet parses --rate and the timesheet (the --from CSV, or JSON on
// stdin), and requires a rate when any row has miles.
func loadTimesheet(path, rateFlag string, stdin bool) ([]gusto.TimesheetRow, float64, error) {
	rate := 0.0
	if strings.TrimSpace(rateFlag) != "" {
		r, err := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(rateFlag), "$"), 64)
		if err != nil || r <= 0 {
			return nil, 0, fmt.Errorf("--rate must be a positive dollars-per-mile figure, e.g. --rate 0.60")
		}
		rate = r
	}
	var rows []gusto.TimesheetRow
	if stdin {
		r, err := gusto.ParseTimesheetJSON(os.Stdin)
		if err != nil {
			return nil, 0, err
		}
		rows = r
	} else {
		f, err := os.Open(filepath.Clean(path))
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %w", errOpenTimesheet, err)
		}
		r, err := gusto.ParseTimesheet(f)
		_ = f.Close()
		if err != nil {
			return nil, 0, err
		}
		rows = r
	}
	for _, r := range rows {
		if r.Miles > 0 && rate <= 0 {
			return nil, 0, fmt.Errorf("%s has %.2f miles; pass --rate (dollars per mile)", r.Employee, r.Miles)
		}
	}
	return rows, rate, nil
}

const timesheetStdinFlagHelp = `Read the timesheet as a JSON array from stdin instead of --from, e.g. [{"employee":"Ava","hours":20,"miles":5.2,"reimbursements":[{"amount":12,"description":"Parking"}]}]`

// timesheetSourceErr requires exactly one of --from and --stdin.
func timesheetSourceErr(from string, stdin bool) error {
	switch {
	case stdin && strings.TrimSpace(from) != "":
		return fmt.Errorf("pass --from or --stdin, not both")
	case !stdin && strings.TrimSpace(from) == "":
		return fmt.Errorf("--from is required: the timesheet CSV (or pass --stdin with a JSON array)")
	}
	return nil
}
