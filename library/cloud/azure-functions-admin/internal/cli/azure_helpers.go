// Hand-authored helpers shared by the SDK-backed Azure commands. No generated
// header: this file is preserved across `generate --force`.
package cli

import (
	"context"
	"fmt"
	"strings"

	armappservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice/v6"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/internal/azure"
)

// azStr safely dereferences an Azure SDK *string.
func azStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// rgFromID extracts the resourceGroups segment from an ARM resource ID.
// IDs look like /subscriptions/{s}/resourceGroups/{rg}/providers/...
func rgFromID(id string) string {
	parts := strings.Split(id, "/")
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "resourceGroups") {
			return parts[i+1]
		}
	}
	return ""
}

// isFunctionApp reports whether an App Service site's kind marks it a Function
// App (kind contains "functionapp", e.g. "functionapp", "functionapp,linux").
func isFunctionApp(kind string) bool {
	return strings.Contains(strings.ToLower(kind), "functionapp")
}

// appServiceClient resolves the subscription and builds a WebAppsClient.
func appServiceClient(subFlag string) (*armappservice.WebAppsClient, string, error) {
	sub, err := azure.SubscriptionID(subFlag)
	if err != nil {
		return nil, "", configErr(err)
	}
	factory, err := azure.AppServiceFactory(sub)
	if err != nil {
		return nil, "", authErr(err)
	}
	return factory.NewWebAppsClient(), sub, nil
}

// listFunctionApps pages every site in the subscription and keeps Function Apps.
func listFunctionApps(ctx context.Context, wac *armappservice.WebAppsClient) ([]*armappservice.Site, error) {
	var apps []*armappservice.Site
	pager := wac.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, site := range page.Value {
			if site != nil && isFunctionApp(azStr(site.Kind)) {
				apps = append(apps, site)
			}
		}
	}
	return apps, nil
}

// findFunctionApp locates a single Function App by name and returns it with its
// resource group, so callers don't have to know the resource group up front.
func findFunctionApp(ctx context.Context, wac *armappservice.WebAppsClient, name string) (*armappservice.Site, string, error) {
	apps, err := listFunctionApps(ctx, wac)
	if err != nil {
		return nil, "", err
	}
	for _, app := range apps {
		if strings.EqualFold(azStr(app.Name), name) {
			return app, rgFromID(azStr(app.ID)), nil
		}
	}
	return nil, "", notFoundErr(fmt.Errorf("function app %q not found in subscription (run `apps list` to see available apps)", name))
}

// knownNonSecretKeys are app-setting keys whose names match the credential
// markers below but are not sensitive: Application Insights instrumentation
// keys and connection strings are designed to be embeddable (e.g. in
// client-side JS) and flagging them trains users to ignore audit output.
var knownNonSecretKeys = map[string]bool{
	"APPINSIGHTS_INSTRUMENTATIONKEY":        true,
	"APPLICATIONINSIGHTS_CONNECTION_STRING": true,
}

// secretKeyHint reports whether an app-setting key name looks like it holds a
// credential, so the audit can flag plaintext values that should be Key Vault
// references. Known non-secret telemetry keys are excluded.
func secretKeyHint(key string) bool {
	k := strings.ToUpper(key)
	if knownNonSecretKeys[k] {
		return false
	}
	for _, marker := range []string{"SECRET", "KEY", "PASSWORD", "PWD", "TOKEN", "CONNECTIONSTRING", "CONNECTION_STRING", "SAS", "CREDENTIAL"} {
		if strings.Contains(k, marker) {
			return true
		}
	}
	return false
}

// isKeyVaultRef reports whether an app-setting value is a Key Vault reference
// rather than a raw secret.
func isKeyVaultRef(val string) bool {
	return strings.HasPrefix(strings.TrimSpace(val), "@Microsoft.KeyVault(")
}

// emitView prints v as JSON, honoring --select/--compact/--quiet. Azure
// analysis output is structured enough that JSON is the honest view for both
// machine consumers and humans reading the result.
func emitView(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// emitDryRun reports a dry-run intent, emitting JSON when --json is set (so the
// output stays parseable for agents and verifiers) and plain text otherwise.
func emitDryRun(cmd *cobra.Command, flags *rootFlags, command, msg string) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"dry_run": true,
			"command": command,
			"message": msg,
		}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}
