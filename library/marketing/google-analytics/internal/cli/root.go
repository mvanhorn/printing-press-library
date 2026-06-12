// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var version = "1.0.0"

const analyticsScope = "https://www.googleapis.com/auth/analytics.readonly"

type rootFlags struct {
	asJSON      bool
	compact     bool
	noInput     bool
	yes         bool
	agent       bool
	propertyID  string
	credentials string
	timeout     time.Duration
}

type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
	ProjectID   string `json:"project_id"`
}

type apiClient struct {
	httpClient *http.Client
	token      string
	baseData   string
	baseAdmin  string
	baseAlpha  string
}

type apiError struct {
	Status int
	Body   string
}

func (e apiError) Error() string { return fmt.Sprintf("google api status %d: %s", e.Status, e.Body) }

// RootCmd returns the Cobra command tree without executing it.
func RootCmd() *cobra.Command { var f rootFlags; return newRootCmd(&f) }

// Execute runs the CLI.
func Execute() error { var f rootFlags; return newRootCmd(&f).Execute() }

func newRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google-analytics-pp-cli",
		Short: "Agent-first Google Analytics 4 CLI for BestSelf, LittleMight, and any GA4 property",
		Long: `Google Analytics 4 Printing Press CLI.

Raw wrappers: report, pivot, batch, realtime, metadata, compatibility, properties, property, streams.
Novel commands: health/doctor, channels, sources, top-pages, events, conversions, funnel, compare, whats-changed, revenue, audience, cohort.

Auth: uses a Google service-account JSON key. Set GOOGLE_APPLICATION_CREDENTIALS, or pass --credentials. Scope: analytics.readonly.
Property resolution for data commands: --property, then GA4_PROPERTY_ID. The CLI never hard-codes a brand property for reads.`,
		SilenceUsage: true,
		Version:      version,
	}
	cmd.SetVersionTemplate("google-analytics-pp-cli {{ .Version }}\n")
	cmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output JSON")
	cmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Prefer token-compact fields where supported")
	cmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Disable prompts (agent/CI safe)")
	cmd.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Assume yes for safe non-mutating confirmations")
	cmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent mode: --json --compact --no-input --yes")
	cmd.PersistentFlags().StringVar(&flags.propertyID, "property", "", "GA4 numeric property ID (defaults to GA4_PROPERTY_ID)")
	cmd.PersistentFlags().StringVar(&flags.credentials, "credentials", "", "Service-account JSON key path (defaults to GOOGLE_APPLICATION_CREDENTIALS)")
	cmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "HTTP request timeout")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.agent {
			flags.asJSON = true
			flags.compact = true
			flags.noInput = true
			flags.yes = true
		}
		return nil
	}
	cmd.AddCommand(newAgentContextCmd())
	cmd.AddCommand(newHealthCmd(flags), newDoctorCmd(flags))
	cmd.AddCommand(newReportCmd(flags), newPivotCmd(flags), newBatchCmd(flags), newRealtimeCmd(flags), newMetadataCmd(flags), newCompatibilityCmd(flags))
	cmd.AddCommand(newPropertiesCmd(flags), newPropertyCmd(flags), newStreamsCmd(flags))
	cmd.AddCommand(newChannelsCmd(flags), newSourcesCmd(flags), newTopPagesCmd(flags), newEventsCmd(flags, false), newEventsCmd(flags, true))
	cmd.AddCommand(newFunnelCmd(flags), newCompareCmd(flags), newWhatsChangedCmd(flags), newRevenueCmd(flags), newAudienceCmd(flags), newCohortCmd(flags))
	return cmd
}

func newAgentContextCmd() *cobra.Command {
	return &cobra.Command{Use: "agent-context", Short: "Emit structured tool description for agents", RunE: func(cmd *cobra.Command, args []string) error {
		return printJSON(cmd.OutOrStdout(), map[string]any{"name": "google-analytics-pp-cli", "binary": "google-analytics-pp-cli", "purpose": "GA4-only analytics CLI with raw API wrappers and one-call novel reports", "auth": "Google service account JSON via --credentials or GOOGLE_APPLICATION_CREDENTIALS; analytics.readonly scope", "property_resolution": "--property, else GA4_PROPERTY_ID; health can accept --properties for fleet checks", "global_flags": []string{"--agent", "--json", "--compact", "--no-input", "--yes", "--property", "--credentials", "--timeout"}, "raw_commands": []string{"report", "pivot", "batch", "realtime", "metadata", "compatibility", "properties", "property", "streams"}, "novel_commands": []string{"channels", "sources", "top-pages", "events", "conversions", "funnel", "compare", "whats-changed", "revenue", "audience", "cohort", "health", "doctor"}, "examples": []string{"google-analytics-pp-cli health --properties 280199692,540652239 --agent", "google-analytics-pp-cli channels --property 280199692 --start 28daysAgo --end yesterday --agent", "google-analytics-pp-cli compare --property 280199692 --metrics sessions,totalRevenue --period wow --agent", "google-analytics-pp-cli funnel --property 280199692 --steps view_item,add_to_cart,begin_checkout,purchase --agent"}})
	}}
}

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var props string
	c := &cobra.Command{Use: "health", Short: "Verify credentials, Admin API, and per-property GA4 access grants", RunE: func(cmd *cobra.Command, args []string) error {
		return runHealth(cmd, flags, props)
	}}
	c.Flags().StringVar(&props, "properties", "", "Comma-separated GA4 property IDs to check (or GA4_PROPERTY_IDS)")
	return c
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var props string
	c := &cobra.Command{Use: "doctor", Short: "Verify credentials, Admin API, and per-property GA4 access grants", RunE: func(cmd *cobra.Command, args []string) error {
		return runHealth(cmd, flags, props)
	}}
	c.Flags().StringVar(&props, "properties", "", "Comma-separated GA4 property IDs to check (or GA4_PROPERTY_IDS)")
	return c
}

func runHealth(cmd *cobra.Command, flags *rootFlags, props string) error {
	client, key, err := newAPIClientWithKey(flags)
	res := map[string]any{"credential_path": credentialPath(flags), "scope": analyticsScope}
	if key.ClientEmail != "" {
		res["service_account"] = key.ClientEmail
		res["project_id"] = key.ProjectID
	}
	if err != nil {
		res["ok"] = false
		res["status"] = "creds_invalid"
		res["error"] = err.Error()
		return output(cmd, flags, res, "")
	}
	summaries, status, err := client.getJSON("https://analyticsadmin.googleapis.com/v1beta/accountSummaries?pageSize=200")
	visible := visibleProperties(summaries)
	res["visible_properties"] = visible
	res["admin_api_status"] = status
	if err != nil {
		res["ok"] = false
		res["status"] = "api_or_token_error"
		res["error"] = err.Error()
		return output(cmd, flags, res, "")
	}
	targets := splitCSV(props)
	if len(targets) == 0 {
		targets = append(targets, configuredProperty(flags))
		if env := os.Getenv("GA4_PROPERTY_IDS"); env != "" {
			targets = append(targets, splitCSV(env)...)
		}
	}
	targets = uniqNonEmpty(targets)
	checks := []map[string]any{}
	allOK := true
	for _, p := range targets {
		body := map[string]any{"dateRanges": []map[string]string{{"startDate": "7daysAgo", "endDate": "yesterday"}}, "metrics": []map[string]string{{"name": "sessions"}}, "limit": "1"}
		_, st, e := client.postJSON(dataURL(p, "runReport"), body)
		chk := map[string]any{"property": p, "status_code": st, "ok": e == nil}
		if e != nil {
			allOK = false
			chk["error"] = classifyAccessError(e)
			chk["detail"] = e.Error()
		}
		checks = append(checks, chk)
	}
	res["property_checks"] = checks
	if len(targets) == 0 {
		res["ok"] = true
		res["status"] = "token_valid_no_property_requested"
	} else if allOK {
		res["ok"] = true
		res["status"] = "valid"
	} else {
		res["ok"] = false
		res["status"] = "property_not_shared_or_invalid"
	}
	return output(cmd, flags, res, renderHealth(res))
}

func newReportCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims, start, end, filter, order string
	var limit int
	c := &cobra.Command{Use: "report", Short: "Raw GA4 runReport wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest(metrics, dims, start, end, limit)
		addRawJSON(req, "dimensionFilter", filter)
		addOrder(req, order)
		return runData(cmd, flags, "runReport", req, "")
	}}
	reportFlags(c, &metrics, &dims, &start, &end, &limit)
	c.Flags().StringVar(&filter, "filter", "", "Raw JSON dimensionFilter")
	c.Flags().StringVar(&order, "order", "", "Order by metric/dimension name, prefix - for desc")
	return c
}
func newPivotCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims, start, end string
	var limit int
	c := &cobra.Command{Use: "pivot", Short: "Raw GA4 runPivotReport wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest(metrics, dims, start, end, limit)
		ds := splitCSV(dims)
		piv := []map[string]any{}
		for _, d := range ds {
			piv = append(piv, map[string]any{"fieldNames": []string{d}, "limit": strconv.Itoa(limit)})
		}
		req["pivots"] = piv
		return runData(cmd, flags, "runPivotReport", req, "")
	}}
	reportFlags(c, &metrics, &dims, &start, &end, &limit)
	return c
}
func newBatchCmd(flags *rootFlags) *cobra.Command {
	var reportsJSON string
	c := &cobra.Command{Use: "batch", Short: "Raw GA4 batchRunReports wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		var reports []any
		if strings.TrimSpace(reportsJSON) == "" {
			reports = []any{reportRequest("sessions,totalUsers", "date", "7daysAgo", "yesterday", 10)}
		} else if err := json.Unmarshal([]byte(reportsJSON), &reports); err != nil {
			return fmt.Errorf("--reports must be a JSON array of RunReportRequest objects: %w", err)
		}
		return runData(cmd, flags, "batchRunReports", map[string]any{"requests": reports}, "")
	}}
	c.Flags().StringVar(&reportsJSON, "reports", "", "JSON array of RunReportRequest bodies (property field omitted; property comes from --property)")
	return c
}

func newRealtimeCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims string
	var limit int
	c := &cobra.Command{Use: "realtime", Short: "Raw GA4 runRealtimeReport wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		req := map[string]any{"metrics": names(splitDefault(metrics, "activeUsers")), "dimensions": names(splitCSV(dims)), "limit": strconv.Itoa(limit)}
		return runData(cmd, flags, "runRealtimeReport", req, "")
	}}
	c.Flags().StringVar(&metrics, "metrics", "activeUsers", "Comma-separated realtime metrics")
	c.Flags().StringVar(&dims, "dimensions", "unifiedScreenName", "Comma-separated realtime dimensions")
	c.Flags().IntVar(&limit, "limit", 10, "Rows")
	return c
}
func newMetadataCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "metadata", Short: "List GA4 dimensions and metrics for a property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := newAPIClientWithKey(flags)
		if err != nil {
			return err
		}
		raw, _, err := cl.getJSON(fmt.Sprintf("https://analyticsdata.googleapis.com/v1beta/properties/%s/metadata", url.PathEscape(p)))
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
}
func newCompatibilityCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims string
	c := &cobra.Command{Use: "compatibility", Short: "Check GA4 metric/dimension compatibility", RunE: func(cmd *cobra.Command, args []string) error {
		req := map[string]any{"metrics": names(splitCSV(metrics)), "dimensions": names(splitCSV(dims)), "compatibilityFilter": "COMPATIBLE"}
		return runData(cmd, flags, "checkCompatibility", req, "")
	}}
	c.Flags().StringVar(&metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Metrics")
	c.Flags().StringVar(&dims, "dimensions", "sessionDefaultChannelGroup", "Dimensions")
	return c
}
func newPropertiesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "properties", Short: "List accessible GA4 account summaries/properties", RunE: func(cmd *cobra.Command, args []string) error {
		cl, _, err := newAPIClientWithKey(flags)
		if err != nil {
			return err
		}
		raw, _, err := cl.getJSON("https://analyticsadmin.googleapis.com/v1beta/accountSummaries?pageSize=200")
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, renderProperties(raw))
	}}
}
func newPropertyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "property", Short: "Get GA4 Admin property metadata", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := newAPIClientWithKey(flags)
		if err != nil {
			return err
		}
		raw, _, err := cl.getJSON(fmt.Sprintf("https://analyticsadmin.googleapis.com/v1beta/properties/%s", url.PathEscape(p)))
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
}
func newStreamsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "streams", Short: "List data streams for a GA4 property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := newAPIClientWithKey(flags)
		if err != nil {
			return err
		}
		raw, _, err := cl.getJSON(fmt.Sprintf("https://analyticsadmin.googleapis.com/v1beta/properties/%s/dataStreams", url.PathEscape(p)))
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
}

func newChannelsCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "channels", Short: "Sessions/users/conversions/revenue by default channel group", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("sessions,totalUsers,conversions,totalRevenue", "sessionDefaultChannelGroup", start, end, limit)
		addOrder(req, "-sessions")
		return novelReport(cmd, flags, req, "channels")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newSourcesCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "sources", Short: "Source/medium acquisition breakdown with conversion rate", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("sessions,totalUsers,conversions,totalRevenue", "sessionSourceMedium", start, end, limit)
		addOrder(req, "-sessions")
		return novelReport(cmd, flags, req, "sources")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newTopPagesCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "top-pages", Short: "Top landing pages by sessions, engagement, and conversions", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("sessions,engagementRate,conversions,totalRevenue", "landingPagePlusQueryString", start, end, limit)
		addOrder(req, "-sessions")
		return novelReport(cmd, flags, req, "top_pages")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newEventsCmd(flags *rootFlags, conversions bool) *cobra.Command {
	var start, end string
	var limit int
	name := "events"
	metric := "eventCount"
	if conversions {
		name = "conversions"
		metric = "conversions"
	}
	c := &cobra.Command{Use: name, Short: "Key events / conversions over time with trend", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest(metric, "date,eventName", start, end, limit)
		if conversions {
			req["dimensionFilter"] = map[string]any{"filter": map[string]any{"fieldName": "isConversionEvent", "stringFilter": map[string]string{"matchType": "EXACT", "value": "true"}}}
		}
		return trendReport(cmd, flags, req, name, metric)
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newRevenueCmd(flags *rootFlags) *cobra.Command {
	var start, end, by string
	var limit int
	c := &cobra.Command{Use: "revenue", Short: "Ecommerce revenue, AOV, and transactions by channel/source", RunE: func(cmd *cobra.Command, args []string) error {
		dim := "sessionDefaultChannelGroup"
		if by == "source" || by == "source-medium" {
			dim = "sessionSourceMedium"
		}
		req := reportRequest("purchaseRevenue,transactions,averagePurchaseRevenue,sessions", dim, start, end, limit)
		addOrder(req, "-purchaseRevenue")
		return novelReport(cmd, flags, req, "revenue")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	c.Flags().StringVar(&by, "by", "channel", "Breakdown: channel or source")
	return c
}
func newAudienceCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "audience", Short: "Audience snapshot by country/device/new-vs-returning", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("totalUsers,newUsers,sessions,engagementRate,conversions", "country,deviceCategory,newVsReturning", start, end, limit)
		addOrder(req, "-totalUsers")
		return novelReport(cmd, flags, req, "audience")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newCohortCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "cohort", Short: "Cheap retention proxy: users by first-user date and returning status", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("totalUsers,sessions,engagementRate", "firstSessionDate,newVsReturning", start, end, limit)
		addDimensionOrder(req, "firstSessionDate")
		return novelReport(cmd, flags, req, "cohort")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var start, end, prevStart, prevEnd, period, metrics, dims string
	var limit int
	c := &cobra.Command{Use: "compare", Short: "Period-over-period deltas and percent change without doing two calls manually", RunE: func(cmd *cobra.Command, args []string) error {
		if start == "" || end == "" {
			start = "7daysAgo"
			end = "yesterday"
		}
		if prevStart == "" || prevEnd == "" {
			prevStart, prevEnd = inferPrevious(start, end, period)
		}
		dimList := splitCSV(dims)
		reqA := reportRequest(metrics, dims, start, end, limit)
		reqB := reportRequest(metrics, dims, prevStart, prevEnd, limit)
		rowsA, rawA, err := runReportRows(flags, reqA)
		if err != nil {
			return err
		}
		rowsB, _, err := runReportRows(flags, reqB)
		if err != nil {
			return err
		}
		out := compareRows(rowsA, rowsB, dimList, splitCSV(metrics))
		out["period_a"] = map[string]string{"start": start, "end": end}
		out["period_b"] = map[string]string{"start": prevStart, "end": prevEnd}
		out["property"] = configuredProperty(flags)
		out["raw_row_count"] = rowCount(rawA)
		return output(cmd, flags, out, renderCompare(out))
	}}
	c.Flags().StringVar(&start, "start", "", "Current period start")
	c.Flags().StringVar(&end, "end", "", "Current period end")
	c.Flags().StringVar(&prevStart, "previous-start", "", "Previous period start")
	c.Flags().StringVar(&prevEnd, "previous-end", "", "Previous period end")
	c.Flags().StringVar(&period, "period", "wow", "If previous dates absent: wow, mom, or trailing")
	c.Flags().StringVar(&metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Metrics")
	c.Flags().StringVar(&metrics, "metric", "sessions,totalUsers,conversions,totalRevenue", "Alias for --metrics")
	c.Flags().StringVar(&dims, "dimensions", "", "Optional dimensions to compare by")
	c.Flags().IntVar(&limit, "limit", 25, "Rows per report")
	return c
}
func newWhatsChangedCmd(flags *rootFlags) *cobra.Command {
	var start, end, prevStart, prevEnd, period, metrics, dims string
	var limit int
	c := &cobra.Command{Use: "whats-changed", Short: "Scan key metrics for notable spikes/drops vs trailing period", RunE: func(cmd *cobra.Command, args []string) error {
		if dims == "" {
			dims = "sessionDefaultChannelGroup,sessionSourceMedium,landingPagePlusQueryString"
		}
		if prevStart == "" || prevEnd == "" {
			prevStart, prevEnd = inferPrevious(start, end, period)
		}
		movers := []map[string]any{}
		for _, dim := range splitCSV(dims) {
			reqA := reportRequest(metrics, dim, start, end, limit)
			reqB := reportRequest(metrics, dim, prevStart, prevEnd, limit)
			a, _, err := runReportRows(flags, reqA)
			if err != nil {
				return err
			}
			b, _, err := runReportRows(flags, reqB)
			if err != nil {
				return err
			}
			cmp := compareRows(a, b, []string{dim}, splitCSV(metrics))
			for _, r := range cmp["rows"].([]map[string]any) {
				r["dimension"] = dim
				movers = append(movers, r)
			}
		}
		sort.Slice(movers, func(i, j int) bool {
			return abs(toFloat(movers[i]["largest_pct_change"])) > abs(toFloat(movers[j]["largest_pct_change"]))
		})
		if len(movers) > limit {
			movers = movers[:limit]
		}
		out := map[string]any{"property": configuredProperty(flags), "period": map[string]string{"start": start, "end": end}, "previous": map[string]string{"start": prevStart, "end": prevEnd}, "movers": movers}
		return output(cmd, flags, out, renderMovers(out))
	}}
	c.Flags().StringVar(&start, "start", "7daysAgo", "Current start")
	c.Flags().StringVar(&end, "end", "yesterday", "Current end")
	c.Flags().StringVar(&prevStart, "previous-start", "", "Previous start")
	c.Flags().StringVar(&prevEnd, "previous-end", "", "Previous end")
	c.Flags().StringVar(&period, "period", "trailing", "wow/mom/trailing")
	c.Flags().StringVar(&metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Metrics")
	c.Flags().StringVar(&dims, "dimensions", "", "Dimensions to scan")
	c.Flags().IntVar(&limit, "limit", 20, "Movers")
	return c
}
func newFunnelCmd(flags *rootFlags) *cobra.Command {
	var steps, start, end string
	c := &cobra.Command{Use: "funnel", Short: "Run GA4 v1alpha runFunnelReport for a named event step sequence", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := newAPIClientWithKey(flags)
		if err != nil {
			return err
		}
		st := splitCSV(steps)
		fs := []map[string]any{}
		for _, s := range st {
			fs = append(fs, map[string]any{"name": s, "filterExpression": map[string]any{"funnelEventFilter": map[string]any{"eventName": s}}})
		}
		req := map[string]any{"dateRanges": []map[string]string{{"startDate": start, "endDate": end}}, "funnel": map[string]any{"steps": fs}}
		raw, _, err := cl.postJSON(fmt.Sprintf("https://analyticsdata.googleapis.com/v1alpha/properties/%s:runFunnelReport", url.PathEscape(p)), req)
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
	c.Flags().StringVar(&steps, "steps", "view_item,add_to_cart,begin_checkout,purchase", "Comma-separated GA4 event names")
	c.Flags().StringVar(&start, "start", "30daysAgo", "Start date")
	c.Flags().StringVar(&end, "end", "yesterday", "End date")
	return c
}

func reportFlags(c *cobra.Command, metrics, dims, start, end *string, limit *int) {
	c.Flags().StringVar(metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Comma-separated metrics")
	c.Flags().StringVar(dims, "dimensions", "date", "Comma-separated dimensions")
	dateLimitFlags(c, start, end, limit)
}
func dateLimitFlags(c *cobra.Command, start, end *string, limit *int) {
	c.Flags().StringVar(start, "start", "30daysAgo", "Start date (YYYY-MM-DD or NdaysAgo)")
	c.Flags().StringVar(end, "end", "yesterday", "End date")
	c.Flags().IntVar(limit, "limit", 25, "Max rows")
}

func configuredProperty(f *rootFlags) string {
	if f.propertyID != "" {
		return strings.TrimSpace(strings.TrimPrefix(f.propertyID, "properties/"))
	}
	return strings.TrimSpace(strings.TrimPrefix(os.Getenv("GA4_PROPERTY_ID"), "properties/"))
}
func requireProperty(f *rootFlags) (string, error) {
	p := configuredProperty(f)
	if p == "" {
		return "", fmt.Errorf("missing GA4 property: pass --property or set GA4_PROPERTY_ID")
	}
	return p, nil
}
func credentialPath(f *rootFlags) string {
	if f.credentials != "" {
		return f.credentials
	}
	if p := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); p != "" {
		return p
	}
	return ""
}
func newAPIClientWithKey(f *rootFlags) (*apiClient, serviceAccountKey, error) {
	var key serviceAccountKey
	path := credentialPath(f)
	if path == "" {
		return nil, key, fmt.Errorf("missing credentials: set GOOGLE_APPLICATION_CREDENTIALS or pass --credentials")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, key, err
	}
	if err := json.Unmarshal(b, &key); err != nil {
		return nil, key, err
	}
	tok, err := mintToken(key)
	if err != nil {
		return nil, key, err
	}
	return &apiClient{httpClient: &http.Client{Timeout: f.timeout}, token: tok}, key, nil
}
func mintToken(key serviceAccountKey) (string, error) {
	if key.TokenURI == "" {
		key.TokenURI = "https://oauth2.googleapis.com/token"
	}
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("invalid service-account private_key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	pk, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service-account private key is not RSA")
	}
	now := time.Now().Unix()
	header := b64json(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims := b64json(map[string]any{"iss": key.ClientEmail, "scope": analyticsScope, "aud": key.TokenURI, "iat": now, "exp": now + 3600})
	unsigned := header + "." + claims
	h := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, pk, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	assertion := unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	resp, err := http.PostForm(key.TokenURI, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", apiError{resp.StatusCode, string(body)}
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}
	return out.AccessToken, nil
}
func b64json(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
func (c *apiClient) getJSON(u string) (map[string]any, int, error) {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, apiError{resp.StatusCode, string(body)}
	}
	var out map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, resp.StatusCode, err
		}
	}
	return out, resp.StatusCode, nil
}
func (c *apiClient) postJSON(u string, body any) (map[string]any, int, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", u, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, apiError{resp.StatusCode, string(rb)}
	}
	var out map[string]any
	if len(rb) > 0 {
		if err := json.Unmarshal(rb, &out); err != nil {
			return nil, resp.StatusCode, err
		}
	}
	return out, resp.StatusCode, nil
}
func dataURL(p, method string) string {
	return fmt.Sprintf("https://analyticsdata.googleapis.com/v1beta/properties/%s:%s", url.PathEscape(p), method)
}
func runData(cmd *cobra.Command, f *rootFlags, method string, req map[string]any, human string) error {
	p, err := requireProperty(f)
	if err != nil {
		return err
	}
	cl, _, err := newAPIClientWithKey(f)
	if err != nil {
		return err
	}
	raw, _, err := cl.postJSON(dataURL(p, method), req)
	if err != nil {
		return err
	}
	return output(cmd, f, raw, human)
}
func runReportRows(f *rootFlags, req map[string]any) ([]map[string]any, map[string]any, error) {
	p, err := requireProperty(f)
	if err != nil {
		return nil, nil, err
	}
	cl, _, err := newAPIClientWithKey(f)
	if err != nil {
		return nil, nil, err
	}
	raw, _, err := cl.postJSON(dataURL(p, "runReport"), req)
	if err != nil {
		return nil, nil, err
	}
	return flattenRows(raw), raw, nil
}
func novelReport(cmd *cobra.Command, f *rootFlags, req map[string]any, name string) error {
	rows, raw, err := runReportRows(f, req)
	if err != nil {
		return err
	}
	out := map[string]any{"report": name, "property": configuredProperty(f), "rows": enrich(rows), "totals": flattenTotals(raw), "row_count": len(rows)}
	return output(cmd, f, out, renderRows(out))
}
func trendReport(cmd *cobra.Command, f *rootFlags, req map[string]any, name, metric string) error {
	rows, _, err := runReportRows(f, req)
	if err != nil {
		return err
	}
	out := map[string]any{"report": name, "property": configuredProperty(f), "metric": metric, "rows": enrich(rows), "trend": trend(rows, metric)}
	return output(cmd, f, out, renderRows(out))
}

func reportRequest(metrics, dims, start, end string, limit int) map[string]any {
	if start == "" {
		start = "30daysAgo"
	}
	if end == "" {
		end = "yesterday"
	}
	if limit <= 0 {
		limit = 25
	}
	return map[string]any{"dateRanges": []map[string]string{{"startDate": start, "endDate": end}}, "metrics": names(splitCSV(metrics)), "dimensions": names(splitCSV(dims)), "limit": strconv.Itoa(limit)}
}
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func splitDefault(s, d string) []string {
	if strings.TrimSpace(s) == "" {
		s = d
	}
	return splitCSV(s)
}
func names(xs []string) []map[string]string {
	out := []map[string]string{}
	for _, x := range xs {
		out = append(out, map[string]string{"name": x})
	}
	return out
}
func addRawJSON(req map[string]any, k, s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		req[k] = v
	}
}
func addOrder(req map[string]any, order string) {
	if order == "" {
		return
	}
	desc := strings.HasPrefix(order, "-")
	name := strings.TrimPrefix(order, "-")
	req["orderBys"] = []map[string]any{{"desc": desc, "metric": map[string]string{"metricName": name}}}
}
func addDimensionOrder(req map[string]any, order string) {
	if order == "" {
		return
	}
	desc := strings.HasPrefix(order, "-")
	name := strings.TrimPrefix(order, "-")
	req["orderBys"] = []map[string]any{{"desc": desc, "dimension": map[string]string{"dimensionName": name}}}
}
func flattenRows(raw map[string]any) []map[string]any {
	var out []map[string]any
	rows, _ := raw["rows"].([]any)
	headersD := headerNames(raw["dimensionHeaders"])
	headersM := headerNames(raw["metricHeaders"])
	for _, rv := range rows {
		rmap, _ := rv.(map[string]any)
		row := map[string]any{}
		valsD := valueList(rmap["dimensionValues"])
		valsM := valueList(rmap["metricValues"])
		for i, h := range headersD {
			if i < len(valsD) {
				row[h] = valsD[i]
			}
		}
		for i, h := range headersM {
			if i < len(valsM) {
				row[h] = parseNum(valsM[i])
			}
		}
		out = append(out, row)
	}
	return out
}
func headerNames(v any) []string {
	arr, _ := v.([]any)
	out := []string{}
	for _, x := range arr {
		m, _ := x.(map[string]any)
		if s, _ := m["name"].(string); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func valueList(v any) []string {
	arr, _ := v.([]any)
	out := []string{}
	for _, x := range arr {
		m, _ := x.(map[string]any)
		s, _ := m["value"].(string)
		out = append(out, s)
	}
	return out
}
func parseNum(s string) any {
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return s
}
func flattenTotals(raw map[string]any) []map[string]any {
	rows, _ := raw["totals"].([]any)
	raw2 := map[string]any{"rows": rows, "metricHeaders": raw["metricHeaders"]}
	return flattenRows(raw2)
}
func enrich(rows []map[string]any) []map[string]any {
	for _, r := range rows {
		conv := toFloat(r["conversions"])
		sess := toFloat(r["sessions"])
		rev := toFloat(r["totalRevenue"]) + toFloat(r["purchaseRevenue"])
		trans := toFloat(r["transactions"])
		if sess > 0 && conv >= 0 {
			r["conversion_rate"] = conv / sess
		}
		if trans > 0 && rev > 0 {
			r["aov"] = rev / trans
		}
	}
	return rows
}
func rowKey(r map[string]any, dims []string) string {
	parts := []string{}
	for _, d := range dims {
		parts = append(parts, fmt.Sprint(r[d]))
	}
	return strings.Join(parts, "|")
}
func compareRows(a, b []map[string]any, dims, metrics []string) map[string]any {
	bm := map[string]map[string]any{}
	for _, r := range b {
		bm[rowKey(r, dims)] = r
	}
	rows := []map[string]any{}
	for _, ar := range a {
		key := rowKey(ar, dims)
		br := bm[key]
		out := map[string]any{"key": key}
		for _, d := range dims {
			out[d] = ar[d]
		}
		maxPct := 0.0
		for _, m := range metrics {
			av := toFloat(ar[m])
			bv := toFloat(br[m])
			delta := av - bv
			pct := 0.0
			if bv != 0 {
				pct = delta / bv
			}
			out[m] = map[string]float64{"current": av, "previous": bv, "delta": delta, "pct_change": pct}
			if abs(pct) > abs(maxPct) {
				maxPct = pct
			}
		}
		out["largest_pct_change"] = maxPct
		rows = append(rows, out)
	}
	return map[string]any{"rows": rows, "row_count": len(rows)}
}
func trend(rows []map[string]any, metric string) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	first := toFloat(rows[0][metric])
	last := toFloat(rows[len(rows)-1][metric])
	delta := last - first
	pct := 0.0
	if first != 0 {
		pct = delta / first
	}
	return map[string]any{"first": first, "last": last, "delta": delta, "pct_change": pct}
}
func inferPrevious(start, end, period string) (string, string) {
	if previousStart, previousEnd, ok := inferPreviousRelativeRange(start, end); ok {
		return previousStart, previousEnd
	}
	switch period {
	case "wow":
		return "14daysAgo", "8daysAgo"
	case "mom":
		return "60daysAgo", "31daysAgo"
	default:
		return "14daysAgo", "8daysAgo"
	}
}
func inferPreviousRelativeRange(start, end string) (string, string, bool) {
	startDays, ok := relativeDaysAgo(start)
	if !ok {
		return "", "", false
	}
	endDays, ok := relativeDaysAgo(end)
	if !ok || startDays < endDays {
		return "", "", false
	}
	windowDays := startDays - endDays + 1
	return formatDaysAgo(startDays + windowDays), formatDaysAgo(endDays + windowDays), true
}
func relativeDaysAgo(value string) (int, bool) {
	value = strings.TrimSpace(value)
	switch value {
	case "today":
		return 0, true
	case "yesterday":
		return 1, true
	}
	if !strings.HasSuffix(value, "daysAgo") {
		return 0, false
	}
	days, err := strconv.Atoi(strings.TrimSuffix(value, "daysAgo"))
	if err != nil || days < 0 {
		return 0, false
	}
	return days, true
}
func formatDaysAgo(days int) string {
	switch days {
	case 0:
		return "today"
	case 1:
		return "yesterday"
	default:
		return strconv.Itoa(days) + "daysAgo"
	}
}
func toFloat(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case map[string]any:
		return toFloat(x["pct_change"])
	}
	return 0
}
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
func rowCount(raw map[string]any) int { r, _ := raw["rows"].([]any); return len(r) }
func uniqNonEmpty(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(strings.TrimPrefix(x, "properties/"))
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func output(cmd *cobra.Command, f *rootFlags, v any, human string) error {
	if f.asJSON || f.agent {
		return printJSON(cmd.OutOrStdout(), v)
	}
	if human != "" {
		fmt.Fprint(cmd.OutOrStdout(), human)
		return nil
	}
	return printJSON(cmd.OutOrStdout(), v)
}
func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func renderRows(v map[string]any) string { rows, _ := v["rows"].([]map[string]any); return table(rows) }
func table(rows []map[string]any) string {
	if len(rows) == 0 {
		return "No rows.\n"
	}
	keys := []string{}
	for k := range rows[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(keys, "\t"))
	for _, r := range rows {
		vals := []string{}
		for _, k := range keys {
			vals = append(vals, fmt.Sprint(r[k]))
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	tw.Flush()
	return b.String()
}
func renderProperties(raw map[string]any) string {
	props := visibleProperties(raw)
	rows := []map[string]any{}
	for _, p := range props {
		rows = append(rows, map[string]any{"property": p})
	}
	return table(rows)
}
func renderHealth(v map[string]any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}
func renderCompare(v map[string]any) string {
	rows, _ := v["rows"].([]map[string]any)
	return table(rows)
}
func renderMovers(v map[string]any) string {
	rows, _ := v["movers"].([]map[string]any)
	return table(rows)
}
func visibleProperties(raw map[string]any) []string {
	var props []string
	sums, _ := raw["accountSummaries"].([]any)
	for _, s := range sums {
		sm, _ := s.(map[string]any)
		ps, _ := sm["propertySummaries"].([]any)
		for _, p := range ps {
			pm, _ := p.(map[string]any)
			if n, _ := pm["property"].(string); n != "" {
				props = append(props, strings.TrimPrefix(n, "properties/"))
			}
		}
	}
	sort.Strings(props)
	return props
}
func classifyAccessError(err error) string {
	var ae apiError
	if errors.As(err, &ae) {
		if ae.Status == 401 {
			return "creds_invalid_or_token_rejected"
		}
		if ae.Status == 403 {
			return "api_enabled_but_property_not_shared_or_permission_denied"
		}
		if ae.Status == 404 {
			return "property_not_found_or_not_shared"
		}
	}
	return "request_failed"
}
