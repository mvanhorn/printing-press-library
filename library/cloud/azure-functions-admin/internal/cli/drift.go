// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type driftKey struct {
	Key         string   `json:"key"`
	PresentIn   []string `json:"present_in"`
	MissingFrom []string `json:"missing_from"`
}

type driftPlaintext struct {
	App string `json:"app"`
	Key string `json:"key"`
}

type driftView struct {
	ResourceGroup    string           `json:"resource_group"`
	AppsCompared     []string         `json:"apps_compared"`
	DriftedKeys      []driftKey       `json:"drifted_keys"`
	PlaintextSecrets []driftPlaintext `json:"plaintext_secrets"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var rg, sub string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Diff app settings across Function Apps in a resource group",
		Long: "Compare application settings across every Function App in a resource group. Reports keys " +
			"that exist in some apps but not others (config drift) and flags plaintext secret-looking values " +
			"that should be Key Vault references. Read-only.",
		Example:     "  azure-functions-admin-pp-cli drift --resource-group my-resource-group --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "drift", fmt.Sprintf("would diff app settings across Function Apps in %q", rg))
			}
			if rg == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--resource-group is required"))
			}
			if out, ok := novelVerifyStub(cmd, flags, "drift", map[string]any{"resource_group": rg, "apps_compared": []any{}, "drifted_keys": []any{}, "plaintext_secrets": []any{}}); ok {
				return out
			}
			ctx := cmd.Context()
			wac, _, err := appServiceClient(sub)
			if err != nil {
				return err
			}

			// Gather Function Apps in the resource group.
			var appNames []string
			pager := wac.NewListByResourceGroupPager(rg, nil)
			settingsByApp := map[string]map[string]string{}
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return novelLookupMiss(cmd, flags, "drift", map[string]any{"resource_group": rg}, classifyAPIError(err, flags))
				}
				for _, site := range page.Value {
					if site == nil || !isFunctionApp(azStr(site.Kind)) {
						continue
					}
					name := azStr(site.Name)
					resp, err := wac.ListApplicationSettings(ctx, rg, name, nil)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					m := make(map[string]string, len(resp.Properties))
					for k, v := range resp.Properties {
						m[k] = azStr(v)
					}
					settingsByApp[name] = m
					appNames = append(appNames, name)
				}
			}
			sort.Strings(appNames)

			// Union of keys → which apps have each.
			keyPresence := map[string][]string{}
			plaintext := make([]driftPlaintext, 0)
			for _, name := range appNames {
				for k, v := range settingsByApp[name] {
					keyPresence[k] = append(keyPresence[k], name)
					if !isKeyVaultRef(v) && secretKeyHint(k) {
						plaintext = append(plaintext, driftPlaintext{App: name, Key: k})
					}
				}
			}

			drifted := make([]driftKey, 0)
			for k, present := range keyPresence {
				if len(present) == len(appNames) {
					continue // present everywhere: no drift
				}
				sort.Strings(present)
				missing := make([]string, 0)
				presentSet := map[string]bool{}
				for _, p := range present {
					presentSet[p] = true
				}
				for _, name := range appNames {
					if !presentSet[name] {
						missing = append(missing, name)
					}
				}
				drifted = append(drifted, driftKey{Key: k, PresentIn: present, MissingFrom: missing})
			}
			sort.Slice(drifted, func(i, j int) bool { return drifted[i].Key < drifted[j].Key })
			sort.Slice(plaintext, func(i, j int) bool {
				if plaintext[i].App != plaintext[j].App {
					return plaintext[i].App < plaintext[j].App
				}
				return plaintext[i].Key < plaintext[j].Key
			})

			return emitView(cmd, flags, driftView{
				ResourceGroup:    rg,
				AppsCompared:     appNames,
				DriftedKeys:      drifted,
				PlaintextSecrets: plaintext,
			})
		},
	}
	cmd.Flags().StringVar(&rg, "resource-group", "", "Resource group whose Function Apps to compare")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	return cmd
}
