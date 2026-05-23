package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCloseCmd(flags *rootFlags) *cobra.Command {
	var month string
	var noCache bool
	var autoSubmit bool
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Bundle a month's untagged + unreported expenses into a single Zoho expense report",
		Example: strings.Trim(`
  zoho-expense-pp-cli close --month 2026-04
  zoho-expense-pp-cli close --month 2026-04 --auto-submit
  zoho-expense-pp-cli close --month 2026-04 --dry-run
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if month == "" {
				return cmd.Help()
			}
			start, end, err := parseMonth(month)
			if err != nil {
				return usageErr(err)
			}

			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			// Inventory pass from the local store (cheap + fast). --no-cache
			// is reserved for future "fetch from API" but for now drives
			// only the dry-run summary.
			_ = noCache

			rows, err := s.DB().Query(
				`SELECT COALESCE(expense_id,id), COALESCE(merchant_name,''), COALESCE(amount,total,0),
				        COALESCE(category_id,''), COALESCE(autoscan_status,''), COALESCE(report_id,'')
				 FROM expenses
				 WHERE expense_date >= ? AND expense_date <= ?`,
				start, end,
			)
			if err != nil {
				return fmt.Errorf("query expenses: %w", err)
			}
			defer rows.Close()
			var unreported []string
			var untagged []string
			var processing []string
			for rows.Next() {
				var id, merchant, catID, autoStatus, reportID string
				var amount float64
				if err := rows.Scan(&id, &merchant, &amount, &catID, &autoStatus, &reportID); err != nil {
					return err
				}
				if reportID == "" {
					unreported = append(unreported, id)
				}
				if catID == "" {
					untagged = append(untagged, id)
				}
				if strings.EqualFold(autoStatus, "Processing") || strings.EqualFold(autoStatus, "InProgress") {
					processing = append(processing, id)
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}

			summary := map[string]any{
				"month":            month,
				"unreported_count": len(unreported),
				"untagged_count":   len(untagged),
				"processing_count": len(processing),
				"will_bundle":      len(unreported),
			}

			if dryRunOK(flags) {
				if flags.asJSON {
					summary["dry_run"] = true
					return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] month=%s unreported=%d untagged=%d processing=%d would_bundle=%d\n",
					month, len(unreported), len(untagged), len(processing), len(unreported))
				return nil
			}

			if len(unreported) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "no unreported expenses in %s\n", month)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			reportBody := map[string]any{
				"report_name":      monthReportName(start),
				"report_date":      end,
				"business_purpose": "Monthly close " + month,
				"expense_ids":      unreported,
			}
			raw, _, err := c.Post(cmd.Context(), "/expensereports", reportBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				ExpenseReport struct {
					ReportID   string `json:"report_id"`
					ReportName string `json:"report_name"`
				} `json:"expensereport"`
			}
			reportID := ""
			if err := json.Unmarshal(raw, &env); err == nil && env.ExpenseReport.ReportID != "" {
				reportID = env.ExpenseReport.ReportID
			} else {
				var bare map[string]any
				if err := json.Unmarshal(raw, &bare); err == nil {
					if id, ok := bare["report_id"].(string); ok {
						reportID = id
					}
				}
			}
			summary["report_id"] = reportID

			if autoSubmit && reportID != "" {
				if _, _, perr := c.Post(cmd.Context(), "/expensereports/"+reportID+"/submit", nil); perr != nil {
					summary["submit_error"] = perr.Error()
				} else {
					summary["submitted"] = true
				}
			}

			if flags.asJSON {
				summary["bundled_expense_ids"] = unreported
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bundled %d expenses for %s into report_id=%s\n", len(unreported), month, reportID)
			if v, ok := summary["submitted"].(bool); ok && v {
				fmt.Fprintln(cmd.OutOrStdout(), "report submitted")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "Target month, YYYY-MM (e.g. 2026-04)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Fetch expenses from API instead of local store")
	cmd.Flags().BoolVar(&autoSubmit, "auto-submit", false, "After creating the report, POST /expensereports/{id}/submit")
	return cmd
}

func parseMonth(s string) (string, string, error) {
	t, err := time.Parse("2006-01", strings.TrimSpace(s))
	if err != nil {
		return "", "", fmt.Errorf("--month must be YYYY-MM: %w", err)
	}
	start := t.Format("2006-01-02")
	endT := t.AddDate(0, 1, -1)
	return start, endT.Format("2006-01-02"), nil
}

func monthReportName(start string) string {
	t, err := time.Parse("2006-01-02", start)
	if err != nil {
		return "Monthly Close " + start
	}
	return t.Format("January 2006")
}
