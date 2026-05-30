// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelScalingCmd(flags *rootFlags) *cobra.Command {
	var app, sub, aiID, window string

	cmd := &cobra.Command{
		Use:   "scaling",
		Short: "Track instance scale-out and execution-time drift over a window",
		Long: "Report per-hour distinct instance count, request volume, and p95 execution duration from " +
			"Application Insights, so you can see whether an app is scaling out and whether its execution time is " +
			"drifting upward. Read-only.",
		Example:     "  azure-functions-admin-pp-cli scaling --app my-function-app --window 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "scaling", fmt.Sprintf("would report scaling/exec-drift for %q", app))
			}
			if app == "" && aiID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--app (or --app-insights-id) is required"))
			}
			if out, ok := novelVerifyStub(cmd, flags, "scaling", map[string]any{"app": app, "buckets": []any{}}); ok {
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
				return novelLookupMiss(cmd, flags, "scaling", map[string]any{"app": app}, err)
			}
			kql := fmt.Sprintf(`requests
| where timestamp > ago(%s)
| summarize instances = dcount(cloud_RoleInstance), requests = count(), p95_ms = round(percentile(duration, 95), 1) by bin(timestamp, 1h)
| sort by timestamp asc`, win)

			rows, err := runKQL(ctx, resID, kql)
			if err != nil {
				return err
			}
			return emitView(cmd, flags, map[string]any{
				"app":             app,
				"window":          win,
				"app_insights_id": resID,
				"buckets":         rows,
			})
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Function App name")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	cmd.Flags().StringVar(&aiID, "app-insights-id", "", "App Insights resource ID (skips auto-resolution from the app)")
	cmd.Flags().StringVar(&window, "window", "7d", "Look-back window as a KQL duration (e.g. 24h, 7d, 30d)")
	return cmd
}
