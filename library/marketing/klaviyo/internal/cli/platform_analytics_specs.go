package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

// platformAnalyticsSpec is the reviewed declaration contract for one computed
// command. CRUD reads, raw SQL/query surfaces, and mutations are intentionally
// absent from this registry.
type platformAnalyticsSpec struct {
	RequiredInputs   []string
	JoinKeys         []string
	MinimumInputRows map[string]int64
	BusinessMetrics  []string
	FreshnessStatus  map[string]string
	Remediation      string
	JoinRequired     bool
}

func singleSourcePlatformAnalyticsSpec(input string, metrics ...string) platformAnalyticsSpec {
	return platformAnalyticsSpec{
		RequiredInputs:   []string{input},
		JoinKeys:         []string{},
		MinimumInputRows: map[string]int64{input: 1},
		BusinessMetrics:  metrics,
		FreshnessStatus:  map[string]string{input: "unknown"},
		Remediation:      "sync or provide a timestamped source window before relying on this result",
	}
}

func scalarPlatformAnalyticsSpec(input string, metrics ...string) platformAnalyticsSpec {
	spec := singleSourcePlatformAnalyticsSpec(input, metrics...)
	spec.FreshnessStatus[input] = "not_applicable"
	spec.Remediation = "provide complete non-secret operator inputs and retry"
	return spec
}

func multiSourcePlatformAnalyticsSpec(inputs []string, metrics ...string) platformAnalyticsSpec {
	spec := joinedPlatformAnalyticsSpec(inputs, []string{}, metrics...)
	spec.Remediation = "sync every required source for the same tenant and window, then retry"
	spec.JoinRequired = false
	return spec
}

func joinedPlatformAnalyticsSpec(inputs, joinKeys []string, metrics ...string) platformAnalyticsSpec {
	minimums := make(map[string]int64, len(inputs))
	freshness := make(map[string]string, len(inputs))
	for _, input := range inputs {
		minimums[input] = 1
		freshness[input] = "unknown"
	}
	return platformAnalyticsSpec{
		RequiredInputs:   append([]string(nil), inputs...),
		JoinKeys:         append(make([]string, 0, len(joinKeys)), joinKeys...),
		MinimumInputRows: minimums,
		BusinessMetrics:  metrics,
		FreshnessStatus:  freshness,
		Remediation:      "sync both sources for the same tenant and window, then inspect missing join keys",
		JoinRequired:     true,
	}
}

var platformAnalyticsSpecs = map[string]platformAnalyticsSpec{
	"analytics":             singleSourcePlatformAnalyticsSpec("selected_local_resource", "record_count"),
	"segment-velocity":      singleSourcePlatformAnalyticsSpec("segment_snapshots", "current_size", "change", "change_pct"),
	"flow-path-analysis":    singleSourcePlatformAnalyticsSpec("flow_message_events", "converted", "conversion_rate", "revenue"),
	"campaign-time-decay":   singleSourcePlatformAnalyticsSpec("campaign_conversion_events", "total_conversions", "total_revenue"),
	"list-quality-score":    singleSourcePlatformAnalyticsSpec("list_performance_records", "quality_score", "engagement_rate", "purchase_rate", "growth_rate"),
	"content-fatigue":       singleSourcePlatformAnalyticsSpec("profile_message_history", "total_profiles_analyzed", "fatigued_profiles", "fatigue_rate"),
	"cohort":                singleSourcePlatformAnalyticsSpec("profile_event_history", "profiles", "retained", "retention_rate"),
	"flow-cannibalization":  singleSourcePlatformAnalyticsSpec("flow_message_events", "total_collisions"),
	"optimal-send-time":     singleSourcePlatformAnalyticsSpec("message_events", "engagement_rate"),
	"segments overlap":      joinedPlatformAnalyticsSpec([]string{"left_segment_members", "right_segment_members"}, []string{"profile_id"}, "overlap_count", "overlap_percentage"),
	"reconcile":             singleSourcePlatformAnalyticsSpec("klaviyo_order_events", "orders", "revenue"),
	"report revenue":        singleSourcePlatformAnalyticsSpec("message_events", "revenue", "orders"),
	"revenue-per-email":     singleSourcePlatformAnalyticsSpec("message_events", "revenue", "emails_sent", "revenue_per_email"),
	"send-fatigue":          singleSourcePlatformAnalyticsSpec("profile_message_history", "fatigued_profiles", "fatigued_percentage"),
	"subject-line-analysis": singleSourcePlatformAnalyticsSpec("campaign_message_events", "open_rate", "click_rate"),
	"flow-decay":            singleSourcePlatformAnalyticsSpec("flow_performance_records", "open_rate", "previous_open_rate", "decay"),
	"report targeting":      joinedPlatformAnalyticsSpec([]string{"campaign_values", "campaign_audience_assignments"}, []string{"segment_id"}, "attributed_revenue", "total_sends"),
}

func platformAnalyticsSpecForCommand(cmd *cobra.Command) (platformAnalyticsSpec, bool) {
	if cmd == nil {
		return platformAnalyticsSpec{}, false
	}
	parts := []string{}
	for current := cmd; current != nil && current.Parent() != nil; current = current.Parent() {
		parts = append([]string{strings.ToLower(current.Name())}, parts...)
	}
	key := strings.Join(parts, " ")
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(cmd.Use))
	}
	spec, ok := platformAnalyticsSpecs[key]
	return spec, ok
}

func applyPlatformAnalyticsSpec(payload any, rowCount int64, observedMetrics map[string]*float64, declaration *platform.AnalyticsDeclaration, spec platformAnalyticsSpec) {
	declaration.RequiredInputs = append([]string(nil), spec.RequiredInputs...)
	declaration.JoinKeys = make([]string, len(spec.JoinKeys))
	copy(declaration.JoinKeys, spec.JoinKeys)
	declaration.MinimumInputRows = make(map[string]int64, len(spec.MinimumInputRows))
	for name, count := range spec.MinimumInputRows {
		declaration.MinimumInputRows[name] = count
	}
	if declaration.InputRows == nil {
		declaration.InputRows = map[string]int64{}
	}
	if len(spec.RequiredInputs) == 1 {
		declaration.InputRows[spec.RequiredInputs[0]] = rowCount
	}
	declaration.ValidJoinKeys = map[string]int64{}
	declaration.MissingJoinKeys = map[string]int64{}
	for _, key := range spec.JoinKeys {
		valid := countPlatformJoinKey(payload, key)
		declaration.ValidJoinKeys[key] = valid
		if valid < rowCount {
			declaration.MissingJoinKeys[key] = rowCount - valid
		} else {
			declaration.MissingJoinKeys[key] = 0
		}
	}
	declaration.NewestRowAt = map[string]time.Time{}
	declaration.FreshnessStatus = make(map[string]string, len(spec.FreshnessStatus))
	for input, status := range spec.FreshnessStatus {
		declaration.FreshnessStatus[input] = status
	}
	if len(spec.RequiredInputs) == 1 {
		input := spec.RequiredInputs[0]
		if declaration.FreshnessStatus[input] != "not_applicable" {
			if newest := newestPlatformAnalyticsTime(payload); !newest.IsZero() {
				declaration.NewestRowAt[input] = newest
				declaration.FreshnessStatus[input] = "observed"
			}
		}
	}
	declaration.Remediation = spec.Remediation
	declaredMetrics := collectPlatformDeclaredMetrics(payload, spec.BusinessMetrics)
	for _, name := range spec.BusinessMetrics {
		if declaredMetrics[name] == nil {
			declaredMetrics[name] = observedMetrics[name]
		}
	}
	declaration.BusinessMetrics = declaredMetrics
}

func collectPlatformDeclaredMetrics(payload any, metricNames []string) map[string]*float64 {
	wanted := make(map[string]string, len(metricNames))
	metrics := make(map[string]*float64, len(metricNames))
	for _, name := range metricNames {
		wanted[strings.ToLower(name)] = name
	}
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for name, child := range typed {
				if canonical, ok := wanted[strings.ToLower(name)]; ok {
					if number, ok := platformNumber(child); ok {
						if metrics[canonical] == nil {
							value := 0.0
							metrics[canonical] = &value
						}
						*metrics[canonical] += number
					}
				}
				visit(child)
			}
		case []map[string]any:
			for _, child := range typed {
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(payload)
	for _, name := range metricNames {
		if _, ok := metrics[name]; !ok {
			metrics[name] = nil
		}
	}
	return metrics
}

func declareReviewedPlatformAnalytics(cmd *cobra.Command, flags *rootFlags, inputRows, validJoinKeys, missingJoinKeys map[string]int64, matchedRows int64, newestRowAt map[string]time.Time, metrics map[string]*float64) {
	spec, ok := platformAnalyticsSpecForCommand(cmd)
	if !ok {
		return
	}
	declaration := platform.AnalyticsDeclaration{
		DataSource:      "command_inputs",
		InputRows:       map[string]int64{},
		MatchedRows:     matchedRows,
		BusinessMetrics: metrics,
	}
	applyPlatformAnalyticsSpec(nil, 0, metrics, &declaration, spec)
	for input, count := range inputRows {
		declaration.InputRows[input] = count
	}
	for key, count := range validJoinKeys {
		declaration.ValidJoinKeys[key] = count
	}
	for key, count := range missingJoinKeys {
		declaration.MissingJoinKeys[key] = count
	}
	for input, newest := range newestRowAt {
		if !newest.IsZero() {
			declaration.NewestRowAt[input] = newest
			declaration.FreshnessStatus[input] = "observed"
		}
	}
	if flags != nil && flags.receiptWriter != nil {
		for input, count := range inputRows {
			flags.receiptWriter.RecordRows(input, platform.RowCounts{Read: count})
		}
		flags.receiptWriter.RecordRows("joined", platform.RowCounts{Emitted: matchedRows})
	}
	declarePlatformAnalytics(flags, declaration)
}

func countPlatformJoinKey(value any, wanted string) int64 {
	var count int64
	var visit func(any, bool)
	visit = func(current any, row bool) {
		switch typed := current.(type) {
		case []any:
			for _, item := range typed {
				visit(item, true)
			}
		case map[string]any:
			if row {
				for key, item := range typed {
					if strings.EqualFold(strings.ReplaceAll(key, "-", "_"), strings.ReplaceAll(wanted, "-", "_")) && strings.TrimSpace(platformString(item)) != "" {
						count++
						break
					}
				}
			}
			for _, item := range typed {
				visit(item, false)
			}
		}
	}
	visit(value, false)
	return count
}

func platformString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func newestPlatformAnalyticsTime(value any) time.Time {
	var newest time.Time
	var visit func(any, string)
	visit = func(current any, key string) {
		switch typed := current.(type) {
		case map[string]any:
			for nestedKey, item := range typed {
				visit(item, strings.ToLower(nestedKey))
			}
		case []any:
			for _, item := range typed {
				visit(item, key)
			}
		case string:
			if !strings.Contains(key, "date") && !strings.Contains(key, "time") && !strings.HasSuffix(key, "_at") {
				return
			}
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
				if parsed, err := time.Parse(layout, typed); err == nil && parsed.After(newest) {
					newest = parsed
					break
				}
			}
		}
	}
	visit(value, "")
	return newest
}
