// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/spf13/cobra"
)

// buildReportDefinition assembles a GoogleAdsAdmanagerV1__ReportDefinition body
// from the comma-separated dimensions/metrics, a single relative date-range enum
// value, and a reportType enum. dimensions and metrics are GAM enum strings
// (e.g. AD_UNIT_NAME, IMPRESSIONS); dateRange is a RelativeDateRange enum
// (e.g. YESTERDAY, LAST_7_DAYS). Empty/blank entries are dropped so a trailing
// comma in the flag value does not emit an empty enum. The result is a plain
// map so it round-trips through json.Marshal without a typed struct.
func buildReportDefinition(dimensions, metrics, dateRange, reportType string) map[string]any {
	def := map[string]any{
		"dimensions": splitEnumList(dimensions),
		"metrics":    splitEnumList(metrics),
	}
	if rt := strings.TrimSpace(reportType); rt != "" {
		def["reportType"] = rt
	}
	if dr := strings.TrimSpace(dateRange); dr != "" {
		def["dateRange"] = map[string]any{"relative": dr}
	}
	return def
}

// splitEnumList comma-splits an enum flag value, trims whitespace, upper-cases
// each token (GAM dimension/metric enums are upper snake case), and drops blanks.
func splitEnumList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// reportResultFromOperation extracts the report-result resource name from a
// completed RunReport operation. RunReportResponse.reportResult lives under the
// operation's response object; this is the `name` passed to fetchRows. Returns
// "" if the operation carries no result (caller treats that as an API error).
func reportResultFromOperation(op json.RawMessage) string {
	var parsed struct {
		Response struct {
			ReportResult string `json:"reportResult"`
		} `json:"response"`
	}
	if err := json.Unmarshal(op, &parsed); err != nil {
		return ""
	}
	return parsed.Response.ReportResult
}

// fetchReportRows pages GET /v1/{resultName}:fetchRows until it has collected up
// to limit rows or the API stops returning a nextPageToken. Under dogfood the
// caller curtails limit before calling; this function additionally caps the
// page count defensively. It returns the accumulated rows as raw JSON values so
// the original ReportRow shape (dimensionValues/metricValueGroups) is preserved
// for the output envelope and for diffing.
func fetchReportRows(ctx context.Context, c *client.Client, resultName string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 1000
	}
	pageCap := 50
	if cliutil.IsDogfoodEnv() {
		pageCap = 2
	}
	var rows []json.RawMessage
	pageToken := ""
	for pages := 0; pages < pageCap; pages++ {
		params := map[string]string{"pageSize": fmt.Sprintf("%d", remainingPageSize(limit, len(rows)))}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := c.Get(ctx, "/v1/"+resultName+":fetchRows", params)
		if err != nil {
			return rows, err
		}
		var page struct {
			Rows          []json.RawMessage `json:"rows"`
			NextPageToken string            `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return rows, err
		}
		rows = append(rows, page.Rows...)
		if len(rows) >= limit {
			rows = rows[:limit]
			break
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return rows, nil
}

// runReportAndFetch :runs an already-created report by its full resource name
// (networks/{code}/reports/{id}), polls the long-running operation to
// completion, and fetches up to limit rows. It is the shared run→poll→fetch
// path behind "report rerun" and "report watch" (both skip the create step).
// Returns the rows, the completed Operation JSON, and any error already wrapped
// with the appropriate exit-code classifier.
func runReportAndFetch(ctx context.Context, c *client.Client, reportName string, limit int, reportTimeout time.Duration) ([]json.RawMessage, json.RawMessage, error) {
	runResp, _, err := c.Post(ctx, "/v1/"+reportName+":run", map[string]any{})
	if err != nil {
		return nil, nil, apiErr(fmt.Errorf("running report %q: %w", reportName, err))
	}
	var op struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(runResp, &op)

	completed, err := pollReportOperation(ctx, c, op.Name, 2*time.Second, reportTimeout)
	if err != nil {
		return nil, completed, err
	}
	resultName := reportResultFromOperation(completed)
	if resultName == "" {
		return nil, completed, apiErr(fmt.Errorf("report operation completed without a result name"))
	}
	rows, err := fetchReportRows(ctx, c, resultName, limit)
	if err != nil {
		return rows, completed, apiErr(fmt.Errorf("fetching report rows: %w", err))
	}
	return rows, completed, nil
}

// remainingPageSize clamps the per-request pageSize so the final page does not
// overshoot the user's --limit. GAM caps pageSize at 1000.
func remainingPageSize(limit, have int) int {
	want := limit - have
	if want > 1000 {
		want = 1000
	}
	if want < 1 {
		want = 1
	}
	return want
}

// pp:data-source live -- creates, runs, polls, and fetches the report through
// the GAM API on every invocation; report rows are not mirrored locally.
func newNovelReportRunCmd(flags *rootFlags) *cobra.Command {
	var flagDimensions string
	var flagMetrics string
	var flagDateRange string
	var flagReportType string
	var flagNetwork string
	var flagLimit int
	var flagWait bool
	var flagReportTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Create, run, poll, and fetch every row of a GAM report in a single blocking command instead of the four-step UI slog.",
		Long: `Create an ad-hoc report from --dimensions, --metrics, and a relative --date-range,
run it, poll the long-running operation to completion, then fetch its rows — all
in one call. Emits a JSON envelope {report_id, operation, row_count, rows}.

With --wait=false the command returns after kicking off the run, emitting
{report_id, operation_name} so you can fetch the rows later. --date-range takes a
GAM RelativeDateRange enum value (e.g. YESTERDAY, LAST_7_DAYS, THIS_MONTH).`,
		Example:     "  google-ad-manager-pp-cli report run --dimensions AD_UNIT_NAME,DATE --metrics IMPRESSIONS,CLICKS --date-range LAST_7_DAYS",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would create+run report and fetch rows (dimensions/metrics/date-range)")
				return nil
			}
			// Bound by the report-timeout (async report runs take minutes), NOT
			// the root --timeout, whose 60s default would cut off polling.
			ctx, cancel := context.WithTimeout(cmd.Context(), flagReportTimeout+30*time.Second)
			defer cancel()

			code, err := resolveNetworkCode(flagNetwork)
			if err != nil {
				return err
			}
			if strings.TrimSpace(flagDimensions) == "" || strings.TrimSpace(flagMetrics) == "" || strings.TrimSpace(flagDateRange) == "" {
				return usageErr(fmt.Errorf("--dimensions, --metrics, and --date-range are all required"))
			}

			limit := flagLimit
			if cliutil.IsDogfoodEnv() && limit > 25 {
				limit = 25
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			parent := networkParent(code)

			// 1. Create the report.
			createBody := map[string]any{
				"reportDefinition": buildReportDefinition(flagDimensions, flagMetrics, flagDateRange, flagReportType),
			}
			created, _, err := c.Post(ctx, "/v1/"+parent+"/reports", createBody)
			if err != nil {
				return apiErr(fmt.Errorf("creating report: %w", err))
			}
			var rep struct {
				Name     string `json:"name"`
				ReportID string `json:"reportId"`
			}
			_ = json.Unmarshal(created, &rep)
			reportName := rep.Name
			if reportName == "" {
				reportName = parent + "/reports/" + rep.ReportID
			}

			// 2. Run the report (returns a long-running Operation).
			runResp, _, err := c.Post(ctx, "/v1/"+reportName+":run", map[string]any{})
			if err != nil {
				return apiErr(fmt.Errorf("running report %q: %w", reportName, err))
			}
			var op struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(runResp, &op)

			if !flagWait {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"report_id":      rep.ReportID,
					"operation_name": op.Name,
				}, flags)
			}

			// 3. Poll the operation to completion.
			completed, err := pollReportOperation(ctx, c, op.Name, 2*time.Second, flagReportTimeout)
			if err != nil {
				return err
			}
			resultName := reportResultFromOperation(completed)
			if resultName == "" {
				return apiErr(fmt.Errorf("report operation completed without a result name"))
			}

			// 4. Fetch rows.
			rows, err := fetchReportRows(ctx, c, resultName, limit)
			if err != nil {
				return apiErr(fmt.Errorf("fetching report rows: %w", err))
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"report_id": rep.ReportID,
				"operation": json.RawMessage(completed),
				"row_count": len(rows),
				"rows":      rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagDimensions, "dimensions", "", "Comma-separated GAM dimension enums to group by (e.g. AD_UNIT_NAME,DATE). Required.")
	cmd.Flags().StringVar(&flagMetrics, "metrics", "", "Comma-separated GAM metric enums to report (e.g. IMPRESSIONS,CLICKS). Required.")
	cmd.Flags().StringVar(&flagDateRange, "date-range", "", "Relative date range enum (e.g. YESTERDAY, LAST_7_DAYS, THIS_MONTH). Required.")
	cmd.Flags().StringVar(&flagReportType, "report-type", "HISTORICAL", "Report type enum (default HISTORICAL).")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "GAM network code (else $GOOGLE_AD_MANAGER_NETWORK_CODE).")
	cmd.Flags().IntVar(&flagLimit, "limit", 1000, "Maximum number of rows to fetch.")
	cmd.Flags().BoolVar(&flagWait, "wait", true, "Poll the run to completion and fetch rows. With --wait=false, return the operation name to fetch later.")
	cmd.Flags().DurationVar(&flagReportTimeout, "report-timeout", 300*time.Second, "Max time to poll the async report run before giving up. Governs the whole create/run/fetch and is NOT bounded by --timeout; large reports may need a higher value.")
	return cmd
}
