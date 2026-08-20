// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelFailuresCmd(flags *rootFlags) *cobra.Command {
	var app, sub, aiID, since string

	cmd := &cobra.Command{
		Use:   "failures",
		Short: "Cluster recent invocation failures by function and result code",
		Long: "Cluster failed requests from Application Insights by operation (function) and result code over a " +
			"look-back window, so you can triage which function is failing most and why. Read-only.",
		Example:     "  azure-functions-admin-pp-cli failures --app my-function-app --since 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "failures", fmt.Sprintf("would cluster failures for %q", app))
			}
			if app == "" && aiID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--app (or --app-insights-id) is required"))
			}
			if out, ok := novelVerifyStub(cmd, flags, "failures", map[string]any{"app": app, "failure_clusters": []any{}}); ok {
				return out
			}
			win, err := kqlWindow(since, "24h")
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
				return novelLookupMiss(cmd, flags, "failures", map[string]any{"app": app}, err)
			}
			kql := fmt.Sprintf(`requests
| where timestamp > ago(%s)
| where success == false
| summarize failures = count(), last_seen = max(timestamp) by operation_Name, resultCode
| sort by failures desc
| take 50`, win)

			rows, err := runKQL(ctx, resID, kql)
			if err != nil {
				return err
			}
			return emitView(cmd, flags, map[string]any{
				"app":              app,
				"since":            win,
				"app_insights_id":  resID,
				"failure_clusters": rows,
			})
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Function App name")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	cmd.Flags().StringVar(&aiID, "app-insights-id", "", "App Insights resource ID (skips auto-resolution from the app)")
	cmd.Flags().StringVar(&since, "since", "24h", "Look-back window as a KQL duration (e.g. 1h, 24h, 7d)")
	return cmd
}
