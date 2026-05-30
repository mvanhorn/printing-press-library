// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type secretFinding struct {
	Key            string `json:"key"`
	Type           string `json:"type"`
	Recommendation string `json:"recommendation,omitempty"`
}

type secretsAuditView struct {
	App                     string          `json:"app"`
	ResourceGroup           string          `json:"resource_group"`
	TotalSettings           int             `json:"total_settings"`
	KeyVaultReferences      int             `json:"key_vault_references"`
	PlaintextSecretSuspects int             `json:"plaintext_secret_suspects"`
	Findings                []secretFinding `json:"findings"`
}

func newNovelSecretsAuditCmd(flags *rootFlags) *cobra.Command {
	var app, sub string

	cmd := &cobra.Command{
		Use:   "secrets-audit",
		Short: "Flag app settings holding raw secrets instead of Key Vault references",
		Long: "Audit a Function App's application settings, flagging values that look like raw secrets " +
			"(keys named *SECRET*, *KEY*, *PASSWORD*, *TOKEN*, *CONNECTIONSTRING*) but are NOT stored as " +
			"@Microsoft.KeyVault references. Use this to find leaked plaintext credentials. Read-only.",
		Example:     "  azure-functions-admin-pp-cli secrets-audit --app my-function-app --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "secrets-audit", fmt.Sprintf("would audit app settings of %q for plaintext secrets", app))
			}
			if app == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--app is required"))
			}
			if out, ok := novelVerifyStub(cmd, flags, "secrets-audit", map[string]any{"app": app, "total_settings": 0, "key_vault_references": 0, "plaintext_secret_suspects": 0, "findings": []any{}}); ok {
				return out
			}
			ctx := cmd.Context()
			wac, _, err := appServiceClient(sub)
			if err != nil {
				return err
			}
			site, rg, err := findFunctionApp(ctx, wac, app)
			if err != nil {
				return novelLookupMiss(cmd, flags, "secrets-audit", map[string]any{"app": app}, err)
			}
			resp, err := wac.ListApplicationSettings(ctx, rg, azStr(site.Name), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			findings := make([]secretFinding, 0, len(resp.Properties))
			var kvRefs, plaintext int
			for k, vptr := range resp.Properties {
				v := azStr(vptr)
				switch {
				case isKeyVaultRef(v):
					kvRefs++
					findings = append(findings, secretFinding{Key: k, Type: "key_vault_reference"})
				case secretKeyHint(k):
					plaintext++
					findings = append(findings, secretFinding{
						Key:            k,
						Type:           "plaintext_secret_suspect",
						Recommendation: "store in Key Vault and reference as @Microsoft.KeyVault(SecretUri=...)",
					})
				}
			}
			sort.Slice(findings, func(i, j int) bool { return findings[i].Key < findings[j].Key })

			return emitView(cmd, flags, secretsAuditView{
				App:                     azStr(site.Name),
				ResourceGroup:           rg,
				TotalSettings:           len(resp.Properties),
				KeyVaultReferences:      kvRefs,
				PlaintextSecretSuspects: plaintext,
				Findings:                findings,
			})
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Function App name to audit")
	cmd.Flags().StringVar(&sub, "subscription", "", "Azure subscription ID (defaults to AZURE_SUBSCRIPTION_ID)")
	return cmd
}
