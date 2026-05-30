// Hand-authored App Insights query support for the KQL-backed novel commands
// (coldstart/scaling/failures/stale). No generated header: preserved across regen.
package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	armappservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice/v6"

	"github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/internal/azure"
)

// kqlDurationRe restricts user-supplied time windows to KQL duration literals
// (e.g. 24h, 7d, 30m). The window is the only user input interpolated into KQL
// text, so it must be validated to prevent query injection.
var kqlDurationRe = regexp.MustCompile(`^[0-9]+[mhd]$`)

// kqlWindow validates a duration window, falling back to def when empty.
func kqlWindow(s, def string) (string, error) {
	if s == "" {
		return def, nil
	}
	if !kqlDurationRe.MatchString(s) {
		return "", usageErr(fmt.Errorf("invalid window %q: use a KQL duration like 24h, 7d, or 30m", s))
	}
	return s, nil
}

// instrumentationKeyFromConnString extracts the InstrumentationKey from an
// APPLICATIONINSIGHTS_CONNECTION_STRING value.
func instrumentationKeyFromConnString(cs string) string {
	for _, part := range strings.Split(cs, ";") {
		if strings.HasPrefix(part, "InstrumentationKey=") {
			return strings.TrimPrefix(part, "InstrumentationKey=")
		}
	}
	return ""
}

// resolveAppInsightsID returns the App Insights component resource ID backing a
// Function App. If explicitID is set it wins. Otherwise the app's
// APPINSIGHTS_INSTRUMENTATIONKEY / APPLICATIONINSIGHTS_CONNECTION_STRING is read
// and matched against the subscription's AI components. Returns an actionable
// error when telemetry isn't configured or can't be matched.
func resolveAppInsightsID(ctx context.Context, sub string, wac *armappservice.WebAppsClient, appName, explicitID string) (string, error) {
	if explicitID != "" {
		return explicitID, nil
	}
	site, rg, err := findFunctionApp(ctx, wac, appName)
	if err != nil {
		return "", err
	}
	resp, err := wac.ListApplicationSettings(ctx, rg, azStr(site.Name), nil)
	if err != nil {
		return "", apiErr(err)
	}
	ikey := azStr(resp.Properties["APPINSIGHTS_INSTRUMENTATIONKEY"])
	if ikey == "" {
		ikey = instrumentationKeyFromConnString(azStr(resp.Properties["APPLICATIONINSIGHTS_CONNECTION_STRING"]))
	}
	if ikey == "" {
		return "", notFoundErr(fmt.Errorf("app %q has no Application Insights configured (no APPINSIGHTS_INSTRUMENTATIONKEY); pass --app-insights-id to query a specific component", appName))
	}

	comps, err := azure.AppInsightsComponentsClient(sub)
	if err != nil {
		return "", err
	}
	pager := comps.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", apiErr(err)
		}
		for _, c := range page.Value {
			if c == nil || c.Properties == nil {
				continue
			}
			if strings.EqualFold(azStr(c.Properties.InstrumentationKey), ikey) {
				return azStr(c.ID), nil
			}
		}
	}
	return "", notFoundErr(fmt.Errorf("could not match app %q to an Application Insights component in this subscription; pass --app-insights-id explicitly", appName))
}

// runKQL executes a KQL query against an App Insights resource and returns the
// first result table as a slice of column->value maps (JSON-friendly).
func runKQL(ctx context.Context, resourceID, query string) ([]map[string]any, error) {
	logs, err := azure.LogsClient()
	if err != nil {
		return nil, err
	}
	resp, err := logs.QueryResource(ctx, resourceID, azquery.Body{Query: &query}, nil)
	if err != nil {
		return nil, fmt.Errorf("querying App Insights: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("App Insights query error: %s", resp.Error.Error())
	}
	if len(resp.Tables) == 0 {
		return []map[string]any{}, nil
	}
	return tableToMaps(resp.Tables[0]), nil
}

// tableToMaps converts an azquery result table to a slice of maps keyed by
// column name, dropping nil columns defensively.
func tableToMaps(t *azquery.Table) []map[string]any {
	out := make([]map[string]any, 0, len(t.Rows))
	for _, row := range t.Rows {
		m := make(map[string]any, len(t.Columns))
		for i, col := range t.Columns {
			if col == nil || col.Name == nil || i >= len(row) {
				continue
			}
			m[*col.Name] = row[i]
		}
		out = append(out, m)
	}
	return out
}
