// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy-seller-dashboard/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy-seller-dashboard/internal/store"
	"github.com/spf13/cobra"
)

type dashboardSyncTask struct {
	resource     string
	label        string
	path         string
	responsePath string
	params       map[string]string
}

type dashboardSyncResult struct {
	Resource   string `json:"resource"`
	Count      int    `json:"count"`
	ObservedAt string `json:"observed_at"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newDashboardSyncCmd(flags))
	})
}

func newDashboardSyncCmd(flags *rootFlags) *cobra.Command {
	var resourcesCSV string
	var since string
	var dbPath string
	var shopID string
	var limit int
	var full bool
	var latestOnly bool
	var maxPages int
	var params []string
	var resourceParams []string
	var globalParams []string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Archive read-only Etsy dashboard observations in local SQLite",
		Long: "Fetch read-only Marketplace Insights, Etsy Ads, Offsite Ads, and promotion observations. " +
			"No Etsy seller setting is changed.",
		Example: "  etsy-seller-dashboard-pp-cli sync --shop-id 12345678 --since 30d",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxPages < 1 {
				return usageErr(fmt.Errorf("--max-pages must be at least 1"))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("etsy-seller-dashboard-pp-cli")
			}
			if shopID == "" {
				shopID = lookupParam(globalParams, "shop_id")
			}
			if shopID == "" {
				return usageErr(fmt.Errorf("--shop-id is required"))
			}
			startDate, endDate, err := dateWindow(since, time.Now().UTC())
			if err != nil {
				return usageErr(err)
			}

			tasks, err := dashboardSyncTasks(resourcesCSV, shopID, startDate, endDate, limit)
			if err != nil {
				return usageErr(err)
			}
			for index := range tasks {
				if tasks[index].params == nil {
					tasks[index].params = make(map[string]string)
				}
				applySyncParams(tasks[index].params, params)
				applySyncParams(tasks[index].params, globalParams)
				applyResourceSyncParams(tasks[index].params, tasks[index].resource, resourceParams)
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, dashboardSyncDryRun(dbPath, tasks, full, maxPages))
			}

			database, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer database.Close()

			apiClient, err := flags.newClient()
			if err != nil {
				return err
			}

			observedAt := time.Now().UTC()
			results := make([]dashboardSyncResult, 0, len(tasks))
			for _, task := range tasks {
				result, err := runDashboardSyncTask(
					cmd, flags, apiClient, database, task, observedAt, latestOnly, full, maxPages,
				)
				if err != nil {
					return fmt.Errorf("syncing %s %s: %w", task.resource, task.label, err)
				}
				results = append(results, result)
			}
			return flags.printJSON(cmd, map[string]any{
				"db": dbPath, "results": results, "remote_mutations": false,
			})
		},
	}
	cmd.Flags().StringVar(&resourcesCSV, "resources", "marketplace-insights,ads,offsite-ads,promotions", "Comma-separated dashboard resources")
	cmd.Flags().StringVar(&since, "since", "30d", "Observation window such as 7d, 24h, or 1w")
	cmd.Flags().BoolVar(&full, "full", false, "Fetch all available Etsy Ads pages, bounded by --max-pages")
	cmd.Flags().BoolVar(&latestOnly, "latest-only", false, "Store only this sync's newest observations")
	cmd.Flags().IntVar(&maxPages, "max-pages", 1, "Maximum pages per paginated resource")
	cmd.Flags().StringSliceVar(&params, "param", nil, "Apply query parameter key=value to every resource")
	cmd.Flags().StringSliceVar(&resourceParams, "resource-param", nil, "Apply resource parameter resource:key=value")
	cmd.Flags().StringSliceVar(&globalParams, "global-param", nil, "Apply global parameter key=value")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&shopID, "shop-id", "", "Etsy shop identifier")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum Etsy Ads listings to fetch")
	return cmd
}

func dashboardSyncTasks(resourcesCSV, shopID, startDate, endDate string, limit int) ([]dashboardSyncTask, error) {
	requested := make(map[string]struct{})
	for _, resource := range strings.Split(resourcesCSV, ",") {
		resource = strings.TrimSpace(resource)
		if resource != "" {
			requested[resource] = struct{}{}
		}
	}
	known := map[string]bool{
		"marketplace-insights": true, "ads": true, "offsite-ads": true, "promotions": true,
	}
	for resource := range requested {
		if !known[resource] {
			return nil, fmt.Errorf("unknown resource %q", resource)
		}
	}

	shopPath := func(path string) string { return replacePathParam(path, "shop_id", shopID) }
	dateParams := func() map[string]string {
		return map[string]string{"start_date": startDate, "end_date": endDate}
	}
	tasks := make([]dashboardSyncTask, 0, 10)
	if _, found := requested["marketplace-insights"]; found {
		tasks = append(tasks,
			dashboardSyncTask{"marketplace-insights", "data", shopPath("/api/v3/ajax/bespoke/shop/{shop_id}/marketplace-insights/data"), "", nil},
			dashboardSyncTask{"marketplace-insights", "trending-terms", shopPath("/api/v3/ajax/bespoke/shop/{shop_id}/marketplace-insights/trending-search-terms-v2"), "searchTerms", nil},
			dashboardSyncTask{"marketplace-insights", "trending-categories", shopPath("/api/v3/ajax/shop/{shop_id}/marketplace-insights/trending-categories"), "", nil},
			dashboardSyncTask{"marketplace-insights", "saved-searches", shopPath("/api/v3/ajax/shop/{shop_id}/marketplace-insights/saved-search-terms"), "", nil},
		)
	}
	if _, found := requested["ads"]; found {
		tasks = append(tasks, dashboardSyncTask{
			"ads", "listings", shopPath("/api/v3/ajax/shop/{shop_id}/prolist/stats/listings"), "listings",
			map[string]string{"limit": strconv.Itoa(limit), "offset": "0"},
		})
	}
	if _, found := requested["offsite-ads"]; found {
		tasks = append(tasks,
			dashboardSyncTask{"offsite-ads", "summary", shopPath("/api/v3/ajax/shop/{shop_id}/offsite-ads-stats"), "", dateParams()},
			dashboardSyncTask{"offsite-ads", "listings", shopPath("/api/v3/ajax/shop/{shop_id}/offsite-ads-data/listing-performance"), "", dateParams()},
			dashboardSyncTask{"offsite-ads", "channels", shopPath("/api/v3/ajax/shop/{shop_id}/offsite-ads-data/channel-performance"), "list", dateParams()},
			dashboardSyncTask{"offsite-ads", "traffic", shopPath("/api/v3/ajax/shop/{shop_id}/offsite-ads-data/ad-traffic"), "", dateParams()},
		)
	}
	if _, found := requested["promotions"]; found {
		path := shopPath("/api/v3/ajax/bespoke/shop/{shop_id}/sales-coupons/combined")
		tasks = append(tasks, dashboardSyncTask{"promotions", "combined", path, "", nil})
	}
	return tasks, nil
}

func dashboardSyncDryRun(dbPath string, tasks []dashboardSyncTask, full bool, maxPages int) map[string]any {
	requests := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		pages := 1
		if full && task.resource == "ads" {
			pages = maxPages
		}
		for page := 0; page < pages; page++ {
			params := cloneSyncParams(task.params)
			if page > 0 {
				limit, _ := strconv.Atoi(params["limit"])
				offset, _ := strconv.Atoi(params["offset"])
				params["offset"] = strconv.Itoa(offset + page*limit)
			}
			requests = append(requests, map[string]any{
				"method": "GET", "resource": task.resource, "label": task.label,
				"path": task.path, "params": params,
			})
		}
	}
	return map[string]any{
		"dry_run": true, "db": dbPath, "requests": requests, "remote_mutations": false,
	}
}

func runDashboardSyncTask(
	cmd *cobra.Command,
	flags *rootFlags,
	apiClient *client.Client,
	database *store.Store,
	task dashboardSyncTask,
	observedAt time.Time,
	latestOnly bool,
	full bool,
	maxPages int,
) (dashboardSyncResult, error) {
	pages := 1
	if full && task.resource == "ads" {
		pages = maxPages
	}
	count := 0
	prepared := make([]store.DashboardObservation, 0)
	replacementLabels := make(map[string]struct{})
	for page := 0; page < pages; page++ {
		params := cloneSyncParams(task.params)
		if page > 0 {
			limit, _ := strconv.Atoi(params["limit"])
			offset, _ := strconv.Atoi(params["offset"])
			params["offset"] = strconv.Itoa(offset + page*limit)
		}
		raw, err := apiClient.GetNoCache(cmd.Context(), task.path, params)
		if err != nil {
			return dashboardSyncResult{}, classifyAPIError(err, flags)
		}
		selections := []dashboardSyncSelection{{label: task.label, responsePath: task.responsePath}}
		if task.resource == "promotions" && task.label == "combined" {
			selections = []dashboardSyncSelection{
				{label: "promotions", responsePath: "promotions"},
				{label: "revenue-stats", responsePath: "revenue_stats"},
			}
		}
		pageCount := 0
		for _, selection := range selections {
			items, err := syncItems(raw, selection.responsePath)
			if err != nil {
				return dashboardSyncResult{}, err
			}
			replacementLabels[selection.label] = struct{}{}
			for index, item := range items {
				identifier := fmt.Sprintf(
					"%s:%s:%06d",
					selection.label,
					observedAt.Format(time.RFC3339Nano),
					count+pageCount+index,
				)
				if latestOnly {
					identifier = fmt.Sprintf("%s:%06d", selection.label, count+pageCount+index)
				}
				enriched, err := enrichObservation(item, selection.label, observedAt, identifier, params)
				if err != nil {
					return dashboardSyncResult{}, err
				}
				prepared = append(prepared, store.DashboardObservation{
					ID: identifier, Label: selection.label, Data: enriched,
				})
			}
			pageCount += len(items)
		}
		count += pageCount
		if task.resource == "ads" {
			limit, _ := strconv.Atoi(params["limit"])
			if pageCount < limit {
				break
			}
		}
	}
	if latestOnly {
		labels := make([]string, 0, len(replacementLabels))
		for label := range replacementLabels {
			labels = append(labels, label)
		}
		if err := database.ReplaceDashboardObservations(
			cmd.Context(), task.resource, labels, prepared,
		); err != nil {
			return dashboardSyncResult{}, err
		}
	} else {
		for _, observation := range prepared {
			if err := upsertDashboardObservation(
				database, task.resource, observation.ID, observation.Data,
			); err != nil {
				return dashboardSyncResult{}, err
			}
		}
	}
	if err := database.SaveSyncState(task.resource, "", count); err != nil {
		return dashboardSyncResult{}, err
	}
	return dashboardSyncResult{
		Resource: task.resource + ":" + task.label,
		Count:    count, ObservedAt: observedAt.Format(time.RFC3339Nano),
	}, nil
}

type dashboardSyncSelection struct {
	label        string
	responsePath string
}

func upsertDashboardObservation(database *store.Store, resource, identifier string, data json.RawMessage) error {
	switch resource {
	case "marketplace-insights":
		return database.UpsertMarketplaceInsights(data)
	case "offsite-ads":
		return database.UpsertOffsiteAds(data)
	case "promotions":
		return database.UpsertPromotions(data)
	default:
		return database.Upsert(resource, identifier, data)
	}
}

func syncItems(raw json.RawMessage, responsePath string) ([]json.RawMessage, error) {
	selected := raw
	if responsePath != "" {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			return nil, fmt.Errorf("decoding response wrapper: %w", err)
		}
		var found bool
		selected, found = wrapper[responsePath]
		if !found {
			return nil, fmt.Errorf("response path %q not found", responsePath)
		}
	}
	var items []json.RawMessage
	if json.Unmarshal(selected, &items) == nil {
		return items, nil
	}
	var object map[string]any
	if err := json.Unmarshal(selected, &object); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return []json.RawMessage{selected}, nil
}

func enrichObservation(
	raw json.RawMessage,
	label string,
	observedAt time.Time,
	identifier string,
	params map[string]string,
) (json.RawMessage, error) {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	value["id"] = identifier
	value["_observation_type"] = label
	value["_observed_at"] = observedAt.Format(time.RFC3339Nano)
	if startDate := params["start_date"]; startDate != "" {
		value["_request_start_date"] = startDate
	}
	if endDate := params["end_date"]; endDate != "" {
		value["_request_end_date"] = endDate
	}
	if limit := params["limit"]; limit != "" {
		value["_request_limit"] = limit
	}
	if offset := params["offset"]; offset != "" {
		value["_request_offset"] = offset
	}
	return json.Marshal(value)
}

func cloneSyncParams(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func dateWindow(since string, now time.Time) (string, string, error) {
	duration, err := parseLookback(since)
	if err != nil {
		return "", "", fmt.Errorf("invalid --since %q: %w", since, err)
	}
	return now.Add(-duration).Format("2006-01-02"), now.Format("2006-01-02"), nil
}

func parseLookback(value string) (time.Duration, error) {
	if len(value) < 2 {
		return 0, fmt.Errorf("expected duration such as 7d, 24h, or 1w")
	}
	unit := value[len(value)-1]
	number, err := strconv.Atoi(value[:len(value)-1])
	if err == nil && number >= 0 {
		switch unit {
		case 'd':
			return time.Duration(number) * 24 * time.Hour, nil
		case 'w':
			return time.Duration(number) * 7 * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(value)
}

func lookupParam(values []string, name string) string {
	for _, value := range values {
		key, result, found := strings.Cut(value, "=")
		if found && key == name {
			return result
		}
	}
	return ""
}

func applySyncParams(target map[string]string, values []string) {
	for _, value := range values {
		key, result, found := strings.Cut(value, "=")
		if found && key != "shop_id" {
			target[key] = result
		}
	}
}

func applyResourceSyncParams(target map[string]string, resource string, values []string) {
	for _, value := range values {
		prefix, assignment, found := strings.Cut(value, ":")
		if !found || prefix != resource {
			continue
		}
		applySyncParams(target, []string{assignment})
	}
}
