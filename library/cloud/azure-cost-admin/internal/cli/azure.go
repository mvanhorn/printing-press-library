package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	costManagementAPIVersion = "2025-03-01"
	resourceGraphAPIVersion  = "2022-10-01"
	retailPricesURL          = "https://prices.azure.com/api/retail/prices"
)

var (
	uuidPattern       = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	emailPattern      = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	bearerPattern     = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._~+/=-]+`)
	subscriptionPath  = regexp.MustCompile(`(?i)/subscriptions/[^[:space:]"']+`)
	resourceIDPattern = regexp.MustCompile(`(?i)/resourceGroups/[^[:space:]"']+`)
)

type subscriptionInfo struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	State     string `json:"state,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

type safeSubscriptionInfo struct {
	Name      string `json:"name"`
	State     string `json:"state,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
	ID        string `json:"id,omitempty"`
}

type costQueryOptions struct {
	Timeframe    string
	From         string
	To           string
	GroupBy      string
	TagKey       string
	Granularity  string
	Subscription string
}

type costQueryResponse struct {
	Properties struct {
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Rows [][]any `json:"rows"`
	} `json:"properties"`
}

type costRow struct {
	Group    string  `json:"group,omitempty"`
	Date     string  `json:"date,omitempty"`
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency,omitempty"`
}

type costSummary struct {
	Timeframe string    `json:"timeframe"`
	Rows      []costRow `json:"rows"`
	Total     float64   `json:"total"`
	Currency  string    `json:"currency,omitempty"`
}

type retailPriceResponse struct {
	Items        []retailPriceRow `json:"Items"`
	NextPageLink string           `json:"NextPageLink"`
	NextLink     string           `json:"nextLink"`
}

type retailPriceRow struct {
	ServiceName   string  `json:"serviceName"`
	SKUName       string  `json:"skuName"`
	ProductName   string  `json:"productName,omitempty"`
	Region        string  `json:"armRegionName"`
	RetailPrice   float64 `json:"retailPrice"`
	UnitOfMeasure string  `json:"unitOfMeasure"`
	CurrencyCode  string  `json:"currencyCode"`
}

type resourceGraphResponse struct {
	Data []resourceRow `json:"data"`
}

type resourceRow struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	ResourceGroup string `json:"resourceGroup"`
	Location      string `json:"location"`
	ID            string `json:"id,omitempty"`
}

type anomalyRow struct {
	Group          string  `json:"group"`
	CurrentCost    float64 `json:"currentCost"`
	PreviousCost   float64 `json:"previousCost"`
	Change         float64 `json:"change"`
	PercentChange  float64 `json:"percentChange"`
	Currency       string  `json:"currency,omitempty"`
	CurrentWindow  string  `json:"currentWindow"`
	PreviousWindow string  `json:"previousWindow"`
}

func redactAzureText(input string) string {
	redacted := bearerPattern.ReplaceAllString(input, "Bearer <redacted>")
	redacted = subscriptionPath.ReplaceAllString(redacted, "/subscriptions/<redacted>")
	redacted = resourceIDPattern.ReplaceAllString(redacted, "/resourceGroups/<redacted>")
	redacted = emailPattern.ReplaceAllString(redacted, "<redacted-email>")
	redacted = uuidPattern.ReplaceAllString(redacted, "<redacted-id>")
	return redacted
}

func safeSubscription(sub subscriptionInfo, includeID bool) safeSubscriptionInfo {
	out := safeSubscriptionInfo{
		Name:      sub.Name,
		State:     sub.State,
		IsDefault: sub.IsDefault,
	}
	if includeID && sub.ID != "" {
		out.ID = sub.ID
	}
	return out
}

func maskID(id string) string {
	if len(id) <= 8 {
		return "<redacted>"
	}
	return id[:4] + "..." + id[len(id)-4:]
}

func (a *app) activeSubscription(ctx context.Context, requested string) (subscriptionInfo, error) {
	args := []string{"account", "show", "--output", "json"}
	if requested != "" {
		args = []string{"account", "show", "--subscription", requested, "--output", "json"}
	}
	out, err := a.runner.Run(ctx, "az", args...)
	if err != nil {
		return subscriptionInfo{}, err
	}
	var sub subscriptionInfo
	if err := json.Unmarshal(out, &sub); err != nil {
		return subscriptionInfo{}, fmt.Errorf("parse active Azure subscription: %w", err)
	}
	if sub.ID == "" {
		return subscriptionInfo{}, fmt.Errorf("active Azure subscription is missing; run az login and az account set")
	}
	if sub.Name == "" && requested != "" {
		sub.Name = requested
	}
	return sub, nil
}

func (a *app) listSubscriptions(ctx context.Context) ([]subscriptionInfo, error) {
	out, err := a.runner.Run(ctx, "az", "account", "list", "--output", "json")
	if err != nil {
		return nil, err
	}
	var subs []subscriptionInfo
	if err := json.Unmarshal(out, &subs); err != nil {
		return nil, fmt.Errorf("parse Azure subscriptions: %w", err)
	}
	return subs, nil
}

func buildCostQuery(opts costQueryOptions) ([]byte, error) {
	timeframe := opts.Timeframe
	if timeframe == "" {
		timeframe = "MonthToDate"
	}
	granularity := opts.Granularity
	if granularity == "" {
		granularity = "None"
	}

	dataset := map[string]any{
		"granularity": granularity,
		"aggregation": map[string]any{
			"totalCost": map[string]string{
				"name":     "Cost",
				"function": "Sum",
			},
		},
	}
	if opts.GroupBy != "" {
		groupingType := "Dimension"
		groupingName := opts.GroupBy
		if opts.TagKey != "" {
			groupingType = "TagKey"
			groupingName = opts.TagKey
		}
		dataset["grouping"] = []map[string]string{{
			"type": groupingType,
			"name": groupingName,
		}}
	}

	body := map[string]any{
		"type":      "ActualCost",
		"timeframe": timeframe,
		"dataset":   dataset,
	}

	if timeframe == "Custom" {
		if opts.From == "" || opts.To == "" {
			return nil, fmt.Errorf("--from and --to are required when --timeframe Custom is used")
		}
		body["timePeriod"] = map[string]string{
			"from": opts.From,
			"to":   opts.To,
		}
	}

	return json.Marshal(body)
}

func addCostFlags(cmd *cobra.Command, opts *costQueryOptions) {
	cmd.Flags().StringVar(&opts.Timeframe, "timeframe", "MonthToDate", "Cost Management timeframe: MonthToDate, BillingMonthToDate, TheLastMonth, or Custom")
	cmd.Flags().StringVar(&opts.From, "from", "", "Start date for Custom timeframe, formatted YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.To, "to", "", "End date for Custom timeframe, formatted YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.Subscription, "subscription", "", "Azure subscription ID or name to inspect")
}

func (a *app) queryCost(ctx context.Context, opts costQueryOptions) (costSummary, error) {
	sub, err := a.activeSubscription(ctx, opts.Subscription)
	if err != nil {
		return costSummary{}, err
	}
	body, err := buildCostQuery(opts)
	if err != nil {
		return costSummary{}, err
	}
	endpoint := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.CostManagement/query?api-version=%s",
		url.PathEscape(sub.ID),
		costManagementAPIVersion,
	)
	out, err := a.runner.Run(ctx, "az", "rest", "--method", "post", "--url", endpoint, "--body", string(body), "--output", "json")
	if err != nil {
		return costSummary{}, err
	}
	var response costQueryResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return costSummary{}, fmt.Errorf("parse Cost Management response: %w", err)
	}
	rows := parseCostRows(response)
	return summarizeCost(opts.Timeframe, rows), nil
}

func parseCostRows(response costQueryResponse) []costRow {
	var rows []costRow
	columnNames := make([]string, len(response.Properties.Columns))
	for i, col := range response.Properties.Columns {
		columnNames[i] = col.Name
	}

	for _, values := range response.Properties.Rows {
		row := costRow{}
		for i, value := range values {
			if i >= len(columnNames) {
				continue
			}
			switch columnNames[i] {
			case "Cost", "PreTaxCost":
				row.Cost = asFloat(value)
			case "Currency":
				row.Currency = asString(value)
			case "UsageDate":
				row.Date = asString(value)
			default:
				if row.Group == "" {
					row.Group = asString(value)
				}
			}
		}
		rows = append(rows, row)
	}

	return rows
}

func summarizeCost(timeframe string, rows []costRow) costSummary {
	summary := costSummary{
		Timeframe: timeframe,
		Rows:      rows,
	}
	for _, row := range rows {
		summary.Total += row.Cost
		if summary.Currency == "" {
			summary.Currency = row.Currency
		}
	}
	return summary
}

func buildMissingTagQuery(tag string, resourceGroup string, limit int) string {
	if limit <= 0 {
		limit = 20
	}
	tag = escapeKustoString(tag)
	parts := []string{
		"Resources",
		fmt.Sprintf("| where isnull(tags['%s']) or tags['%s'] == ''", tag, tag),
	}
	if resourceGroup != "" {
		parts = append(parts, fmt.Sprintf("| where resourceGroup == '%s'", escapeKustoString(resourceGroup)))
	}
	parts = append(parts, "| project name, type, resourceGroup, location, id")
	parts = append(parts, fmt.Sprintf("| take %d", limit))
	return strings.Join(parts, " ")
}

func (a *app) queryMissingTags(ctx context.Context, subscription string, tag string, resourceGroup string, limit int) ([]resourceRow, error) {
	query := buildMissingTagQuery(tag, resourceGroup, limit)
	args := []string{"graph", "query", "--graph-query", query, "--output", "json", "--only-show-errors"}
	if subscription != "" {
		args = append(args, "--subscriptions", subscription)
	}
	out, err := a.runner.Run(ctx, "az", args...)
	if err != nil {
		return nil, err
	}
	var response resourceGraphResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, fmt.Errorf("parse Resource Graph response: %w", err)
	}
	for i := range response.Data {
		response.Data[i].ID = redactAzureText(response.Data[i].ID)
	}
	return response.Data, nil
}

func buildRetailPriceURL(service string, region string, currency string) string {
	filters := []string{}
	if service != "" {
		filters = append(filters, fmt.Sprintf("serviceName eq '%s'", escapeODataString(service)))
	}
	if region != "" {
		filters = append(filters, fmt.Sprintf("armRegionName eq '%s'", escapeODataString(region)))
	}
	if currency != "" {
		filters = append(filters, fmt.Sprintf("currencyCode eq '%s'", escapeODataString(currency)))
	}

	params := url.Values{}
	params.Set("$top", "1000")
	if len(filters) > 0 {
		params.Set("$filter", strings.Join(filters, " and "))
	}
	return retailPricesURL + "?" + params.Encode()
}

func (a *app) searchRetailPrices(ctx context.Context, service string, region string, sku string, currency string) ([]retailPriceRow, error) {
	endpoint := buildRetailPriceURL(service, region, currency)
	seen := map[string]bool{}
	var rows []retailPriceRow
	for endpoint != "" {
		if seen[endpoint] {
			return nil, fmt.Errorf("Azure Retail Prices pagination repeated endpoint: %s", endpoint)
		}
		seen[endpoint] = true

		page, err := a.fetchRetailPricePage(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page.Items...)
		endpoint = strings.TrimSpace(page.nextPageLink())
	}
	if sku == "" {
		return rows, nil
	}
	sku = strings.ToLower(sku)
	filtered := make([]retailPriceRow, 0, len(rows))
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row.SKUName), sku) || strings.Contains(strings.ToLower(row.ProductName), sku) {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func (a *app) fetchRetailPricePage(ctx context.Context, endpoint string) (retailPriceResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return retailPriceResponse{}, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return retailPriceResponse{}, fmt.Errorf("query Azure Retail Prices: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retailPriceResponse{}, fmt.Errorf("read Azure Retail Prices response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return retailPriceResponse{}, fmt.Errorf("Azure Retail Prices returned %s: %s", resp.Status, redactAzureText(string(body)))
	}
	return parseRetailPriceResponse(body)
}

func parseRetailPriceResponse(body []byte) (retailPriceResponse, error) {
	var response retailPriceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return retailPriceResponse{}, fmt.Errorf("parse Azure Retail Prices response: %w", err)
	}
	return response, nil
}

func (r retailPriceResponse) nextPageLink() string {
	if r.NextPageLink != "" {
		return r.NextPageLink
	}
	return r.NextLink
}

func (a *app) findAnomalies(ctx context.Context, subscription string, days int, thresholdPercent float64) ([]anomalyRow, error) {
	if days <= 0 {
		days = 7
	}
	today := a.now().UTC().Truncate(24 * time.Hour)
	currentTo := today.AddDate(0, 0, -1)
	currentFrom := currentTo.AddDate(0, 0, -days+1)
	previousTo := currentFrom.AddDate(0, 0, -1)
	previousFrom := previousTo.AddDate(0, 0, -days+1)

	current, err := a.queryCost(ctx, costQueryOptions{
		Timeframe:    "Custom",
		From:         currentFrom.Format("2006-01-02"),
		To:           currentTo.Format("2006-01-02"),
		GroupBy:      "ServiceName",
		Subscription: subscription,
	})
	if err != nil {
		return nil, err
	}
	previous, err := a.queryCost(ctx, costQueryOptions{
		Timeframe:    "Custom",
		From:         previousFrom.Format("2006-01-02"),
		To:           previousTo.Format("2006-01-02"),
		GroupBy:      "ServiceName",
		Subscription: subscription,
	})
	if err != nil {
		return nil, err
	}

	currentByGroup := costByGroup(current.Rows)
	previousByGroup := costByGroup(previous.Rows)
	groups := map[string]bool{}
	for group := range currentByGroup {
		groups[group] = true
	}
	for group := range previousByGroup {
		groups[group] = true
	}

	var anomalies []anomalyRow
	for group := range groups {
		currentCost := currentByGroup[group]
		previousCost := previousByGroup[group]
		change := currentCost - previousCost
		percent := percentChange(previousCost, change)
		if math.Abs(percent) < thresholdPercent && math.Abs(change) < 1 {
			continue
		}
		anomalies = append(anomalies, anomalyRow{
			Group:          group,
			CurrentCost:    currentCost,
			PreviousCost:   previousCost,
			Change:         change,
			PercentChange:  percent,
			Currency:       current.Currency,
			CurrentWindow:  currentFrom.Format("2006-01-02") + "/" + currentTo.Format("2006-01-02"),
			PreviousWindow: previousFrom.Format("2006-01-02") + "/" + previousTo.Format("2006-01-02"),
		})
	}
	sort.Slice(anomalies, func(i, j int) bool {
		return math.Abs(anomalies[i].Change) > math.Abs(anomalies[j].Change)
	})
	return anomalies, nil
}

func costByGroup(rows []costRow) map[string]float64 {
	out := map[string]float64{}
	for _, row := range rows {
		group := row.Group
		if group == "" {
			group = "total"
		}
		out[group] += row.Cost
	}
	return out
}

func percentChange(previous float64, change float64) float64 {
	if previous == 0 {
		if change == 0 {
			return 0
		}
		return 100
	}
	return (change / previous) * 100
}

func printCostSummary(out io.Writer, summary costSummary, title string, limit int) {
	fmt.Fprintf(out, "%s\n", title)
	fmt.Fprintf(out, "Total: %.2f %s\n", summary.Total, summary.Currency)
	if len(summary.Rows) == 0 || (len(summary.Rows) == 1 && summary.Rows[0].Group == "") {
		return
	}
	fmt.Fprintln(out, "\nGroup\tCost\tCurrency")
	printed := 0
	for _, row := range summary.Rows {
		if limit > 0 && printed >= limit {
			break
		}
		group := row.Group
		if group == "" {
			group = "(none)"
		}
		fmt.Fprintf(out, "%s\t%.2f\t%s\n", group, row.Cost, row.Currency)
		printed++
	}
}

func printResources(out io.Writer, rows []resourceRow) {
	fmt.Fprintln(out, "Name\tType\tResourceGroup\tLocation")
	for _, row := range rows {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", row.Name, row.Type, row.ResourceGroup, row.Location)
	}
}

func printRetailPrices(out io.Writer, rows []retailPriceRow, limit int) {
	fmt.Fprintln(out, "Service\tSKU\tRegion\tPrice\tUnit\tCurrency")
	for i, row := range rows {
		if limit > 0 && i >= limit {
			break
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%.6f\t%s\t%s\n", row.ServiceName, row.SKUName, row.Region, row.RetailPrice, row.UnitOfMeasure, row.CurrencyCode)
	}
}

func printAnomalies(out io.Writer, rows []anomalyRow, limit int) {
	fmt.Fprintln(out, "Group\tCurrent\tPrevious\tChange\tPercent\tCurrency")
	for i, row := range rows {
		if limit > 0 && i >= limit {
			break
		}
		fmt.Fprintf(out, "%s\t%.2f\t%.2f\t%.2f\t%.1f%%\t%s\n", row.Group, row.CurrentCost, row.PreviousCost, row.Change, row.PercentChange, row.Currency)
	}
}

func dryRunCost(out io.Writer, opts costQueryOptions) error {
	body, err := buildCostQuery(opts)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.CostManagement/query?api-version=%s", "<subscription>", costManagementAPIVersion)
	return writeJSON(out, map[string]any{
		"method": "POST",
		"url":    endpoint,
		"body":   json.RawMessage(body),
	})
}

func dryRunGraph(out io.Writer, query string) error {
	return writeJSON(out, map[string]any{
		"command": "az graph query",
		"query":   query,
	})
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		out, _ := typed.Float64()
		return out
	case string:
		out, _ := strconv.ParseFloat(typed, 64)
		return out
	default:
		return 0
	}
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func escapeKustoString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func escapeODataString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
