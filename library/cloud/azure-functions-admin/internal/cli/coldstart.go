// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelColdstartCmd(flags *rootFlags) *cobra.Command {
	var app, sub, aiID, window string

	cmd := &cobra.Command{
		Use:   "coldstart",
		Short: "Measure how often a Function App cold-starts and how slow those starts are",
		Long: "Derive cold starts from Application Insights request telemetry. Azure does not expose cold start " +
			"as a first-class metric, so this detects requests that follow an idle gap (the app having scaled to " +
			"zero) and reports their count and latency alongside overall p50/p95. Consumption-plan apps are the " +
			"ones that cold-start. Read-only.",
		Example:     "  azure-functions-admin-pp-cli coldstart --app my-function-app --window 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "coldstart", fmt.Sprintf("would measure cold starts for %q", app))
			}
			if app == "" && aiID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--app (or --app-insights-id) is required"))
			}
			if out, ok := novelVerifyStub(cmd, flags, "coldstart", map[string]any{"app": app, "cold_start_summary": []any{}}); ok {
				return out
			}
			win, err := kqlWindow(window, "7d")
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			wac, subID, err := appServiceClient(sub)
			if err != nil {
				return err
			}
			resID, err := resolveAppInsightsID(ctx, subID, wac, app, aiID)
			if err != nil {
				return novelLookupMiss(cmd, flags, "coldstart", map[string]any{"app": app}, err)
			}
			// Cold start ~ a request that follows an idle gap (>5 min) on the
			// app (the instance was reclaimed during the idle period). The very
			// first request in the window is deliberately NOT counted: with no
			// in-window predecessor its prior idle state is unknowable, so
			// counting it would over-report cold starts on busy apps.
			kql := fmt.Sprintf(`requests
| where timestamp > ago(%s)
| sort by timestamp asc
| serialize
| extend gapMin = todouble(datetime_diff('second', timestamp, prev(timestamp))) / 60.0
| extend coldStart = isnotnull(prev(timestamp)) and gapMin > 5
| summarize total_requests = count(), cold_starts = countif(coldStart), p50_ms = round(percentile(duration, 50), 1), p95_ms = round(percentile(duration, 95), 1)`, win)

			rows, err := runKQL(ctx, resID, kql)
			if err != nil {
				return err
			}
			return emitView(cmd, flags, map[string]any{
				"app":                app,
				"window":             win,
				"app_insights_id":    resID,
				"cold_start_summary": rows,
			})
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Function App name")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	cmd.Flags().StringVar(&aiID, "app-insights-id", "", "App Insights resource ID (skips auto-resolution from the app)")
	cmd.Flags().StringVar(&window, "window", "7d", "Look-back window as a KQL duration (e.g. 24h, 7d, 30d)")
	return cmd
}
