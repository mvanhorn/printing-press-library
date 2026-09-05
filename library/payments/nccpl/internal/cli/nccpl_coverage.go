package cli

// pp:data-source local

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

type nccplCoverageRow struct {
	Resource     string   `json:"resource"`
	StoredDates  int      `json:"stored_dates"`
	FirstDate    string   `json:"first_date,omitempty"`
	LastDate     string   `json:"last_date,omitempty"`
	MissingCount int      `json:"missing_dates"`
	Missing      []string `json:"missing,omitempty"`
	EmptyDates   int      `json:"empty_dates"`
	TotalRows    int      `json:"total_rows"`
	MinRows      int      `json:"min_rows_on_a_date"`
	MaxRows      int      `json:"max_rows_on_a_date"`
}

type nccplCoverageView struct {
	Resources []nccplCoverageRow `json:"resources"`
	HasGaps   bool               `json:"has_gaps"`
	Note      string             `json:"note,omitempty"`
	DBPath    string             `json:"db_path"`
}

func newNCCPLCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		resourcesCSV string
		dbPath       string
		exitCode     bool
		listMissing  int
	)

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report which settlement dates are stored, missing or empty per resource",
		Long: strings.Trim(`
Diff each resource's stored date set against the weekday session calendar spanning
what it holds, and report interior gaps, empty dates and per-date row width.

This is the check that sees what is MISSING. The latest-date endpoints report only a
resource's newest published date and are structurally blind to a hole in the middle of
the archive.

A date recorded with zero rows is reported as empty, not missing: it was fetched and
NCCPL published nothing. A date never fetched is reported as missing. Keeping those
apart is the whole point of the coverage ledger.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli coverage
  nccpl-pp-cli coverage --resources fipi,lipi --exit-code
  nccpl-pp-cli coverage --resources var-margins --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--resources=fipi",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: nccpl-pp-cli sync --resources fipi --latest-only --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), nccplCoverageView{Resources: make([]nccplCoverageRow, 0), DBPath: dbPath}, flags)
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
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\nrun: nccpl-pp-cli sync --resources fipi --latest-only --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), nccplCoverageView{Resources: make([]nccplCoverageRow, 0), DBPath: dbPath}, flags)
				}
				return nil
			}

			selected, err := nccplSelectResources(resourcesCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			view := nccplCoverageView{Resources: make([]nccplCoverageRow, 0, len(selected)), DBPath: dbPath}
			for _, r := range selected {
				covered, err := store.NCCPLCoveredDates(ctx, db, r.Name)
				if err != nil {
					return err
				}
				row := nccplCoverageRow{Resource: r.Name, Missing: make([]string, 0)}
				if len(covered) == 0 {
					view.Resources = append(view.Resources, row)
					continue
				}
				have := make(map[string]bool, len(covered))
				row.MinRows, row.MaxRows = covered[0].RowCount, covered[0].RowCount
				for _, c := range covered {
					have[c.Date] = true
					row.TotalRows += c.RowCount
					if c.RowCount == 0 {
						row.EmptyDates++
					}
					if c.RowCount < row.MinRows {
						row.MinRows = c.RowCount
					}
					if c.RowCount > row.MaxRows {
						row.MaxRows = c.RowCount
					}
				}
				row.StoredDates = len(covered)
				row.FirstDate = covered[0].Date
				row.LastDate = covered[len(covered)-1].Date

				sessions, err := nccplSessionDates(row.FirstDate, row.LastDate)
				if err != nil {
					return err
				}
				for _, d := range sessions {
					if !have[d] {
						row.MissingCount++
						if listMissing < 0 || len(row.Missing) < listMissing {
							row.Missing = append(row.Missing, d)
						}
					}
				}
				if row.MissingCount > 0 {
					view.HasGaps = true
				}
				view.Resources = append(view.Resources, row)
			}
			if view.HasGaps {
				view.Note = "interior sessions are absent from the local store; re-run sync for the affected ranges before using this data"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-7s %-12s %-12s %-8s %-7s %-9s %s\n",
					"RESOURCE", "DATES", "FIRST", "LAST", "MISSING", "EMPTY", "ROWS", "ROWS/DATE")
				for _, r := range view.Resources {
					if r.StoredDates == 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-7s %s\n", r.Resource, "0", "(nothing synced)")
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-7d %-12s %-12s %-8d %-7d %-9d %d-%d\n",
						r.Resource, r.StoredDates, r.FirstDate, r.LastDate,
						r.MissingCount, r.EmptyDates, r.TotalRows, r.MinRows, r.MaxRows)
				}
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
				}
			}

			if exitCode && view.HasGaps {
				return notFoundErr(fmt.Errorf("coverage gaps found in %d resource(s)", nccplGapCount(view)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to audit; empty means all")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "Exit 3 when any audited resource has a gap, for pipeline gating")
	cmd.Flags().IntVar(&listMissing, "list-missing", 20, "Maximum missing dates to list per resource (-1 for all)")
	return cmd
}

// nccplGapCount counts audited resources that have at least one missing session.
func nccplGapCount(v nccplCoverageView) int {
	n := 0
	for _, r := range v.Resources {
		if r.MissingCount > 0 {
			n++
		}
	}
	return n
}
