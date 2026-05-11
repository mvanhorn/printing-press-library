package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// runallExportSlugs are the documented CSV-export endpoint slugs visible on the
// Reports page (Data Exports section). Each one corresponds to a real
// /api/v1/admin/<facility>/reports/<slug> endpoint.
// Names are taken verbatim from the Reports page in app.goose.pet.
var runallExportSlugs = []string{
	"collection-methods-export",
	"package-credit-transactions-export",
	"admin-managed-credits",
	"wallet-credit-transactions-export",
	"receivable-balances-export",
	"deposit-balances-export",
	"wallet-credit-balances-export",
	"admin-added-wallet-value-export",
	"sales-export",
	"bookings-export",
	"groomer-sales-export",
	"all-pets-export",
	"all-customers-export",
	"customer-activity-export",
	"customer-agreements-export",
	"expiring-or-missing-vaccinations-export",
	"all-members-export",
	"non-billable-members-export",
	"member-renewal-collections-export",
	"feeding-medication-export",
}

// newReportsRunAllCmd implements `reports run-all` — parallel fan-out
// over every documented CSV-export endpoint. Writes timestamped files into
// ./reports/<label>/<slug>.json.
//
// Each underlying endpoint accepts a date parameter or a date-range
// (startDate/endDate). For weekly fan-out we pass the week's date range.
func newReportsRunAllCmd(flags *rootFlags) *cobra.Command {
	var week string
	var date string
	var outDir string
	var concurrency int

	cmd := &cobra.Command{
		Use:         "run-all",
		Short:       "Parallel fan-out: pull all documented CSV-export reports for a date or week",
		Example:     "  goose-pp-cli reports run-all --week 2026-W19\n  goose-pp-cli reports run-all --date 2026-05-10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var label, startDate, endDate string
			switch {
			case week != "":
				start, end, err := weekToDates(week)
				if err != nil {
					return err
				}
				label = week
				startDate = start
				endDate = end
			case date != "":
				label = date
				startDate = date
				endDate = date
			default:
				today := time.Now().Format("2006-01-02")
				label = today
				startDate = today
				endDate = today
			}

			if outDir == "" {
				outDir = filepath.Join(".", "reports", label)
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"label":   label,
						"out_dir": outDir,
						"slugs":   runallExportSlugs,
						"action":  "would-fan-out",
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would fan out %d exports to %s for %s..%s\n", len(runallExportSlugs), outDir, startDate, endDate)
				return nil
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", outDir, err)
			}

			if concurrency <= 0 {
				concurrency = 6
			}
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			var mu sync.Mutex
			results := []map[string]any{}

			for _, slug := range runallExportSlugs {
				slug := slug
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					params := map[string]string{
						"date":      startDate,
						"startDate": startDate,
						"endDate":   endDate,
					}
					data, err := c.Get("/reports/"+slug, params)
					row := map[string]any{
						"slug":      slug,
						"startDate": startDate,
						"endDate":   endDate,
					}
					if err != nil {
						row["status"] = "error"
						row["error"] = err.Error()
					} else {
						path := filepath.Join(outDir, slug+".json")
						if werr := os.WriteFile(path, data, 0o600); werr != nil {
							row["status"] = "write-error"
							row["error"] = werr.Error()
						} else {
							row["status"] = "ok"
							row["path"] = path
							row["bytes"] = len(data)
						}
					}
					mu.Lock()
					results = append(results, row)
					mu.Unlock()
				}()
			}
			wg.Wait()

			ok := 0
			for _, r := range results {
				if r["status"] == "ok" {
					ok++
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"label":   label,
					"out_dir": outDir,
					"ok":      ok,
					"total":   len(results),
					"results": results,
				}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Fan-out complete: %d/%d ok for %s..%s\n", ok, len(results), startDate, endDate)
			fmt.Fprintf(w, "Output: %s\n", outDir)
			for _, r := range results {
				if r["status"] != "ok" {
					fmt.Fprintf(w, "  ✗ %s: %v\n", r["slug"], r["error"])
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&week, "week", "", "ISO week (YYYY-Www, e.g. 2026-W19); mutually exclusive with --date")
	cmd.Flags().StringVar(&date, "date", "", "Single date (YYYY-MM-DD); mutually exclusive with --week")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory (default ./reports/<label>)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 6, "Max parallel requests")
	return cmd
}

// weekToDates parses an ISO week token like "2026-W19" and returns the
// Monday/Sunday dates as YYYY-MM-DD.
func weekToDates(token string) (string, string, error) {
	// Parse "YYYY-Www"
	parts := strings.SplitN(token, "-W", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid week %q (want YYYY-Www e.g. 2026-W19)", token)
	}
	var year, week int
	_, err := fmt.Sscanf(parts[0]+" "+parts[1], "%d %d", &year, &week)
	if err != nil {
		return "", "", fmt.Errorf("parsing week %q: %w", token, err)
	}
	// ISO week 1 is the week containing the first Thursday of the year.
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	dayOffset := int(jan4.Weekday())
	if dayOffset == 0 {
		dayOffset = 7
	}
	monday := jan4.AddDate(0, 0, -(dayOffset-1)+(week-1)*7)
	sunday := monday.AddDate(0, 0, 6)
	return monday.Format("2006-01-02"), sunday.Format("2006-01-02"), nil
}

// Ensure json import is used.
var _ = json.RawMessage(nil)
