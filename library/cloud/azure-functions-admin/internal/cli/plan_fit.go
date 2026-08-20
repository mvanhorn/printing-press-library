// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/internal/azure"
)

type planFitRow struct {
	App            string `json:"app"`
	ResourceGroup  string `json:"resource_group"`
	Plan           string `json:"plan"`
	SKU            string `json:"sku"`
	Tier           string `json:"tier"`
	HasColdStarts  bool   `json:"has_cold_starts"`
	Recommendation string `json:"recommendation"`
}

type planFitView struct {
	Subscription  string       `json:"subscription"`
	ResourceGroup string       `json:"resource_group,omitempty"`
	Apps          []planFitRow `json:"apps"`
}

func planFitRecommendation(tier azure.PlanTier) string {
	switch tier {
	case azure.TierConsumption:
		return "On Consumption (scales to zero; cold starts apply). Run `coldstart --app <name>` to measure cold-start impact before considering Premium."
	case azure.TierPremium:
		return "On Premium (pre-warmed, no cold starts). If invocation volume is low, Consumption may cut cost — verify with `scaling --app <name>`."
	case azure.TierDedicated:
		return "On a Dedicated App Service plan (always on, no elastic scale). For spiky serverless workloads, Consumption or Premium usually fit better."
	default:
		return "Hosting plan tier unrecognized; inspect the plan SKU directly."
	}
}

func newNovelPlanFitCmd(flags *rootFlags) *cobra.Command {
	var rg, sub string

	cmd := &cobra.Command{
		Use:   "plan-fit",
		Short: "Recommend Consumption vs Premium vs Dedicated per Function App",
		Long: "For each Function App, resolve its hosting plan tier (Consumption/Premium/Dedicated) and give a " +
			"hosting recommendation. Scope to a resource group with --resource-group, or omit it to scan the whole " +
			"subscription. Read-only.",
		Example:     "  azure-functions-admin-pp-cli plan-fit --resource-group my-resource-group --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "plan-fit", "would recommend a hosting plan per Function App")
			}
			ctx := cmd.Context()
			subID, err := azure.SubscriptionID(sub)
			if err != nil {
				return configErr(err)
			}
			factory, err := azure.AppServiceFactory(subID)
			if err != nil {
				return authErr(err)
			}
			wac := factory.NewWebAppsClient()
			plans := factory.NewPlansClient()

			// Map plan resource ID -> SKU name for tier resolution.
			skuByPlanID := map[string]string{}
			ppager := plans.NewListPager(nil)
			for ppager.More() {
				page, err := ppager.NextPage(ctx)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, p := range page.Value {
					if p == nil {
						continue
					}
					sku := ""
					if p.SKU != nil {
						sku = azStr(p.SKU.Name)
					}
					skuByPlanID[strings.ToLower(azStr(p.ID))] = sku
				}
			}

			apps, err := listFunctionApps(ctx, wac)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			rows := make([]planFitRow, 0, len(apps))
			for _, app := range apps {
				appRG := rgFromID(azStr(app.ID))
				if rg != "" && !strings.EqualFold(appRG, rg) {
					continue
				}
				planID, sku := "", ""
				if app.Properties != nil {
					planID = azStr(app.Properties.ServerFarmID)
				}
				if planID != "" {
					sku = skuByPlanID[strings.ToLower(planID)]
				}
				tier := azure.ClassifyPlanTier(sku)
				rows = append(rows, planFitRow{
					App:            azStr(app.Name),
					ResourceGroup:  appRG,
					Plan:           planBaseName(planID),
					SKU:            sku,
					Tier:           string(tier),
					HasColdStarts:  tier.HasColdStarts(),
					Recommendation: planFitRecommendation(tier),
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].App < rows[j].App })

			return emitView(cmd, flags, planFitView{
				Subscription:  subID,
				ResourceGroup: rg,
				Apps:          rows,
			})
		},
	}
	cmd.Flags().StringVar(&rg, "resource-group", "", "Limit to one resource group (default: whole subscription)")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	return cmd
}

// planBaseName returns the trailing resource name from a serverfarms resource ID.
func planBaseName(id string) string {
	if id == "" {
		return ""
	}
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}
