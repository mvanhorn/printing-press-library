// Copyright 2026 kevin-magnan. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Beehiiv-specific read-only growth and audience intelligence",
		Long:  "Read-only compound workflows for publication health, subscriber sources, post performance, custom-field coverage, referral health, and subscriber lookup.",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}

	cmd.AddCommand(newInsightsGrowthSummaryCmd(flags))
	cmd.AddCommand(newInsightsSubscriberSourcesCmd(flags))
	cmd.AddCommand(newInsightsPostPerformanceCmd(flags))
	cmd.AddCommand(newInsightsFieldCoverageCmd(flags))
	cmd.AddCommand(newInsightsReferralHealthCmd(flags))
	cmd.AddCommand(newInsightsSubscriberLookupCmd(flags))

	return cmd
}

func newInsightsGrowthSummaryCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "growth-summary <publicationId>",
		Short: "Summarize publication, subscriber, post, referral, and field health in one read-only call",
		Example: `  beehiiv-pp-cli insights growth-summary pub_... --agent
  beehiiv-pp-cli insights growth-summary pub_... --limit 100 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH: Escape compound insight publication IDs before inserting them into API paths.
			pubID := escapePathParam(args[0])
			if limit <= 0 {
				limit = 100
			}

			// PATCH: Surface partial-failure warnings for compound Beehiiv insight calls.
			publication, pubErr := beehiivObject(c.Get("/publications/"+pubID, nil))
			subscriptions, subMeta, subErr := beehiivArray(c.Get("/publications/"+pubID+"/subscriptions", map[string]string{"limit": fmt.Sprintf("%d", limit)}))
			posts, postMeta, postErr := beehiivArray(c.Get("/publications/"+pubID+"/posts", map[string]string{"limit": fmt.Sprintf("%d", limit)}))
			fields, _, fieldErr := beehiivArray(c.Get("/publications/"+pubID+"/custom_fields", nil))
			referral, refErr := beehiivObject(c.Get("/publications/"+pubID+"/referral_program", nil))

			result := map[string]any{
				"publication": map[string]any{
					"id":                       stringValue(publication, "id"),
					"name":                     stringValue(publication, "name"),
					"organization_name":        stringValue(publication, "organization_name"),
					"referral_program_enabled": boolValue(publication, "referral_program_enabled"),
				},
				"subscribers": map[string]any{
					"sample_count": len(subscriptions),
					"total":        numberOrMeta(subMeta, "total_results", len(subscriptions)),
					"status":       countByString(subscriptions, "status"),
					"tier":         countByString(subscriptions, "subscription_tier"),
					"sources":      topCounts(sourceCounts(subscriptions), 10),
				},
				"posts": map[string]any{
					"sample_count": len(posts),
					"total":        numberOrMeta(postMeta, "total_results", len(posts)),
					"status":       countByString(posts, "status"),
					"audience":     countByString(posts, "audience"),
				},
				"custom_fields": map[string]any{
					"count": len(fields),
					"kinds": countByString(fields, "kind"),
				},
				"referral_program": referralSummary(referral),
				"warnings": compactWarnings(map[string]error{
					"publication":      pubErr,
					"subscriptions":    subErr,
					"posts":            postErr,
					"custom_fields":    fieldErr,
					"referral_program": refErr,
				}),
				"sample_limit": limit,
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum subscriptions and posts to sample for summary counts")
	return cmd
}

func newInsightsSubscriberSourcesCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "subscriber-sources <publicationId>",
		Short: "Group subscribers by UTM and referral source fields",
		Example: `  beehiiv-pp-cli insights subscriber-sources pub_... --agent
  beehiiv-pp-cli insights subscriber-sources pub_... --limit 100 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if limit <= 0 {
				limit = 100
			}
			// PATCH: Escape compound insight publication IDs before inserting them into API paths.
			pubID := escapePathParam(args[0])
			subs, meta, err := beehiivArray(c.Get("/publications/"+pubID+"/subscriptions", map[string]string{"limit": fmt.Sprintf("%d", limit)}))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{
				"sample_count":   len(subs),
				"total":          numberOrMeta(meta, "total_results", len(subs)),
				"utm_source":     topCounts(countByString(subs, "utm_source"), 15),
				"utm_medium":     topCounts(countByString(subs, "utm_medium"), 15),
				"utm_channel":    topCounts(countByString(subs, "utm_channel"), 15),
				"referring_site": topCounts(countByString(subs, "referring_site"), 15),
				"combined":       topCounts(sourceCounts(subs), 20),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum subscriptions to sample")
	return cmd
}

func newInsightsPostPerformanceCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "post-performance <publicationId>",
		Short: "List recent posts with status, audience, publish timing, and available engagement stats",
		Example: `  beehiiv-pp-cli insights post-performance pub_... --agent
  beehiiv-pp-cli insights post-performance pub_... --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if limit <= 0 {
				limit = 25
			}
			// PATCH: Escape compound insight publication IDs before inserting them into API paths.
			pubID := escapePathParam(args[0])
			posts, meta, err := beehiivArray(c.Get("/publications/"+pubID+"/posts", map[string]string{
				"limit":     fmt.Sprintf("%d", limit),
				"expand":    "stats",
				"order_by":  "created",
				"direction": "desc",
			}))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items := make([]map[string]any, 0, len(posts))
			for _, post := range posts {
				items = append(items, map[string]any{
					"id":             stringValue(post, "id"),
					"title":          firstNonEmpty(stringValue(post, "title"), stringValue(post, "subtitle"), stringValue(post, "slug")),
					"status":         stringValue(post, "status"),
					"audience":       stringValue(post, "audience"),
					"platform":       stringValue(post, "platform"),
					"created":        anyValue(post, "created"),
					"publish_date":   anyValue(post, "publish_date"),
					"displayed_date": anyValue(post, "displayed_date"),
					"stats":          anyValue(post, "stats"),
				})
			}
			result := map[string]any{
				"sample_count": len(items),
				"total":        numberOrMeta(meta, "total_results", len(items)),
				"posts":        items,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum posts to inspect")
	return cmd
}

func newInsightsFieldCoverageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "field-coverage <publicationId>",
		Short:       "Inspect custom-field definitions and subscriber sample size for coverage planning",
		Example:     `  beehiiv-pp-cli insights field-coverage pub_... --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH: Escape compound insight publication IDs before inserting them into API paths.
			pubID := escapePathParam(args[0])
			fields, fieldMeta, err := beehiivArray(c.Get("/publications/"+pubID+"/custom_fields", nil))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			subs, subMeta, subErr := beehiivArray(c.Get("/publications/"+pubID+"/subscriptions", map[string]string{"limit": "100"}))
			result := map[string]any{
				"custom_field_count":      len(fields),
				"custom_field_total":      numberOrMeta(fieldMeta, "total_results", len(fields)),
				"kinds":                   countByString(fields, "kind"),
				"fields":                  fields,
				"subscriber_sample_count": len(subs),
				"subscriber_total":        numberOrMeta(subMeta, "total_results", len(subs)),
				"coverage_note":           "Beehiiv's list endpoint may omit per-subscriber custom-field values. Use this as a schema and sample-size audit before enriching subscribers.",
			}
			if subErr != nil {
				result["subscriber_sample_warning"] = subErr.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func newInsightsReferralHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "referral-health <publicationId>",
		Short:       "Summarize referral-program configuration and subscriber referral-code coverage",
		Example:     `  beehiiv-pp-cli insights referral-health pub_... --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH: Escape compound insight publication IDs before inserting them into API paths.
			pubID := escapePathParam(args[0])
			publication, pubErr := beehiivObject(c.Get("/publications/"+pubID, nil))
			referral, refErr := beehiivObject(c.Get("/publications/"+pubID+"/referral_program", nil))
			subs, meta, subErr := beehiivArray(c.Get("/publications/"+pubID+"/subscriptions", map[string]string{"limit": "100"}))
			withCodes := 0
			referred := 0
			for _, sub := range subs {
				if stringValue(sub, "referral_code") != "" {
					withCodes++
				}
				if stringValue(sub, "referring_site") != "" || stringValue(sub, "referring_subscriber_id") != "" {
					referred++
				}
			}
			result := map[string]any{
				"publication": map[string]any{
					"id":                       stringValue(publication, "id"),
					"name":                     stringValue(publication, "name"),
					"referral_program_enabled": boolValue(publication, "referral_program_enabled"),
				},
				"referral_program": referralSummary(referral),
				"subscriber_sample": map[string]any{
					"sample_count":         len(subs),
					"total":                numberOrMeta(meta, "total_results", len(subs)),
					"with_referral_code":   withCodes,
					"with_referral_source": referred,
				},
				"warnings": compactWarnings(map[string]error{
					"publication":       pubErr,
					"referral_program":  refErr,
					"subscriber_sample": subErr,
				}),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func newInsightsSubscriberLookupCmd(flags *rootFlags) *cobra.Command {
	var email string
	var subscriptionID string

	cmd := &cobra.Command{
		Use:   "subscriber-lookup <publicationId>",
		Short: "Find one subscriber by email or subscription ID and return the compact record",
		Example: `  beehiiv-pp-cli insights subscriber-lookup pub_... --email person@example.com --agent
  beehiiv-pp-cli insights subscriber-lookup pub_... --subscription-id sub_... --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if email == "" && subscriptionID == "" {
				return usageErr(fmt.Errorf("provide --email or --subscription-id"))
			}
			if email != "" && subscriptionID != "" {
				return usageErr(fmt.Errorf("provide only one of --email or --subscription-id"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := ""
			if email != "" {
				path = "/publications/" + escapePathParam(args[0]) + "/subscriptions/by_email/" + escapePathParam(email)
			} else {
				path = "/publications/" + escapePathParam(args[0]) + "/subscriptions/" + escapePathParam(subscriptionID)
			}
			obj, err := beehiivObject(c.Get(path, nil))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), obj, flags)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Subscriber email address")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Beehiiv subscription ID")
	return cmd
}

func beehiivArray(raw json.RawMessage, err error) ([]map[string]any, map[string]any, error) {
	if err != nil {
		return nil, nil, err
	}
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) == nil {
		meta := map[string]any{}
		for _, key := range []string{"limit", "page", "total_pages", "total_results", "has_more", "next_cursor"} {
			if v, ok := envelope[key]; ok {
				meta[key] = v
			}
		}
		if data, ok := envelope["data"]; ok {
			if arr, ok := data.([]any); ok {
				return mapSlice(arr), meta, nil
			}
		}
	}
	var arr []any
	if json.Unmarshal(raw, &arr) == nil {
		return mapSlice(arr), nil, nil
	}
	return nil, nil, fmt.Errorf("response did not contain an array")
}

func beehiivObject(raw json.RawMessage, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) == nil {
		if data, ok := envelope["data"]; ok {
			if obj, ok := data.(map[string]any); ok {
				return obj, nil
			}
		}
		return envelope, nil
	}
	return nil, fmt.Errorf("response did not contain an object")
}

func mapSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func stringValue(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if v, ok := obj[key]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

func boolValue(obj map[string]any, key string) bool {
	if obj == nil {
		return false
	}
	if v, ok := obj[key].(bool); ok {
		return v
	}
	return false
}

func anyValue(obj map[string]any, key string) any {
	if obj == nil {
		return nil
	}
	return obj[key]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func numberOrMeta(meta map[string]any, key string, fallback int) any {
	if meta == nil {
		return fallback
	}
	if v, ok := meta[key]; ok {
		return v
	}
	return fallback
}

func countByString(items []map[string]any, key string) map[string]int {
	counts := map[string]int{}
	for _, item := range items {
		value := stringValue(item, key)
		if value == "" {
			value = "(blank)"
		}
		counts[value]++
	}
	return counts
}

func sourceCounts(items []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, item := range items {
		source := firstNonEmpty(
			stringValue(item, "utm_source"),
			stringValue(item, "referring_site"),
			stringValue(item, "utm_channel"),
			stringValue(item, "utm_medium"),
		)
		if source == "" {
			source = "(direct/unknown)"
		}
		counts[source]++
	}
	return counts
}

type countRow struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

func topCounts(counts map[string]int, limit int) []countRow {
	rows := make([]countRow, 0, len(counts))
	for value, count := range counts {
		rows = append(rows, countRow{Value: value, Count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Value < rows[j].Value
	})
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func referralSummary(referral map[string]any) map[string]any {
	if referral == nil {
		return map[string]any{"available": false}
	}
	return map[string]any{
		"available": true,
		"id":        stringValue(referral, "id"),
		"name":      firstNonEmpty(stringValue(referral, "name"), stringValue(referral, "title")),
		"status":    stringValue(referral, "status"),
		"rewards":   anyValue(referral, "rewards"),
	}
}

func compactWarnings(warnings map[string]error) []map[string]string {
	out := []map[string]string{}
	keys := make([]string, 0, len(warnings))
	for key := range warnings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if warnings[key] != nil {
			out = append(out, map[string]string{"source": key, "error": warnings[key].Error()})
		}
	}
	return out
}
