// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var app, sub, aiID string
	var days int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find functions with zero invocations in the last N days",
		Long: "Cross-reference a Function App's declared functions (from ARM) against the operations seen in " +
			"Application Insights over the last N days. Functions with no recent invocations are flagged as cleanup " +
			"candidates. Read-only.",
		Example:     "  azure-functions-admin-pp-cli stale --app my-function-app --days 90 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "stale", fmt.Sprintf("would find functions in %q with no invocations in %dd", app, days))
			}
			if app == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--app is required"))
			}
			if days <= 0 {
				days = 90
			}
			if out, ok := novelVerifyStub(cmd, flags, "stale", map[string]any{"app": app, "days": days, "declared_count": 0, "active_count": 0, "stale_functions": []any{}, "declared_functions": []any{}}); ok {
				return out
			}
			ctx := cmd.Context()
			wac, subID, err := appServiceClient(sub)
			if err != nil {
				return err
			}
			site, rg, err := findFunctionApp(ctx, wac, app)
			if err != nil {
				return novelLookupMiss(cmd, flags, "stale", map[string]any{"app": app}, err)
			}

			// Declared functions from ARM.
			declared := make([]string, 0)
			fnPager := wac.NewListFunctionsPager(rg, azStr(site.Name), nil)
			for fnPager.More() {
				page, err := fnPager.NextPage(ctx)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, fn := range page.Value {
					if fn == nil {
						continue
					}
					declared = append(declared, lastSegment(azStr(fn.Name)))
				}
			}

			// Active operations from App Insights.
			resID, err := resolveAppInsightsID(ctx, subID, wac, app, aiID)
			if err != nil {
				return err
			}
			kql := fmt.Sprintf(`requests
| where timestamp > ago(%dd)
| summarize last_seen = max(timestamp) by operation_Name`, days)
			rows, err := runKQL(ctx, resID, kql)
			if err != nil {
				return err
			}
			active := map[string]bool{}
			for _, r := range rows {
				if name, ok := r["operation_Name"].(string); ok {
					active[strings.ToLower(lastSegment(name))] = true
				}
			}

			stale := make([]string, 0)
			for _, fn := range declared {
				if !active[strings.ToLower(fn)] {
					stale = append(stale, fn)
				}
			}
			sort.Strings(declared)
			sort.Strings(stale)

			return emitView(cmd, flags, map[string]any{
				"app":                azStr(site.Name),
				"days":               days,
				"declared_count":     len(declared),
				"active_count":       len(declared) - len(stale),
				"stale_functions":    stale,
				"declared_functions": declared,
			})
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Function App name")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	cmd.Flags().StringVar(&aiID, "app-insights-id", "", "App Insights resource ID (skips auto-resolution from the app)")
	cmd.Flags().IntVar(&days, "days", 90, "Days of invocation history to consider")
	return cmd
}

// lastSegment returns the trailing path segment of an Azure function name,
// which is often "<appname>/<functionname>".
func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 && i+1 < len(s) {
		return s[i+1:]
	}
	return s
}
