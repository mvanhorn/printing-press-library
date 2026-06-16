package intelcli

import (
	"fmt"
	"strings"
	"time"
)

type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "High"
	ConfidenceMedium ConfidenceLevel = "Medium"
	ConfidenceLow    ConfidenceLevel = "Low"
	ConfidenceBroken ConfidenceLevel = "Broken"
)

type ConfidenceSignals struct {
	Profile                    string             `json:"profile"`
	SyncedAt                   time.Time          `json:"synced_at"`
	Entities                   int                `json:"entities"`
	SourceCoverage             map[string]float64 `json:"source_coverage"`
	HasRevenue                 bool               `json:"has_revenue"`
	HasConversions             bool               `json:"has_conversions"`
	HasSearchImpressions       bool               `json:"has_search_impressions"`
	DuplicateTrackingSuspected bool               `json:"duplicate_tracking_suspected"`
	MicroConversionPollution   bool               `json:"micro_conversion_pollution"`
	ConversionsDivergence      float64            `json:"conversions_divergence,omitempty"`
	BrokenSchemaChecks         []string           `json:"broken_schema_checks,omitempty"`
}

type ConfidenceReport struct {
	Profile        string             `json:"profile"`
	Level          ConfidenceLevel    `json:"level"`
	Score          int                `json:"score"`
	Summary        string             `json:"summary"`
	Freshness      string             `json:"freshness"`
	SourceCoverage map[string]float64 `json:"source_coverage"`
	Checks         []ConfidenceCheck  `json:"checks"`
	FailingChecks  []string           `json:"failing_checks"`
	FixFirst       []string           `json:"fix_tracking_first"`
}

type ConfidenceCheck struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func EvaluateConfidence(sig ConfidenceSignals, now time.Time) ConfidenceReport {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	report := ConfidenceReport{
		Profile:        sig.Profile,
		Score:          100,
		Freshness:      freshness(sig.SyncedAt, now),
		SourceCoverage: sig.SourceCoverage,
		Checks:         []ConfidenceCheck{},
		FailingChecks:  []string{},
		FixFirst:       []string{},
	}
	add := func(id, status, severity, msg, fix string, penalty int) {
		report.Checks = append(report.Checks, ConfidenceCheck{ID: id, Status: status, Severity: severity, Message: msg})
		if status == "fail" {
			report.FailingChecks = append(report.FailingChecks, id)
			if strings.TrimSpace(fix) != "" {
				report.FixFirst = append(report.FixFirst, fix)
			}
			report.Score -= penalty
		}
	}
	if len(sig.BrokenSchemaChecks) > 0 {
		for _, id := range sig.BrokenSchemaChecks {
			add("schema_"+id, "fail", "critical", "Unsupported or missing child-CLI schema contract for "+id, "upgrade or re-run the child CLI with supported schema metadata", 45)
		}
	}
	if sig.Entities <= 0 {
		add("sample_size", "fail", "critical", "No local entities are available for analysis", "run sync with fixture, import, or supported child CLI data", 45)
	} else if sig.Entities < 3 {
		add("sample_size", "fail", "high", fmt.Sprintf("Only %d local entities are available", sig.Entities), "sync enough rows before trusting forecasts", 25)
	} else {
		add("sample_size", "pass", "high", fmt.Sprintf("%d local entities available", sig.Entities), "", 0)
	}
	switch report.Freshness {
	case "stale", "missing":
		add("freshness", "fail", "high", "Dataset is stale or missing a sync timestamp", "run sync with a current date range", 25)
	default:
		add("freshness", "pass", "high", "Dataset freshness is acceptable", "", 0)
	}
	if !sig.HasSearchImpressions {
		add("gsc_impressions", "fail", "high", "GSC impressions are missing; CTR and Strike Zone math would be fabricated", "sync GSC impressions before computing search-derived metrics", 25)
	} else {
		add("gsc_impressions", "pass", "high", "GSC impression signal is present", "", 0)
	}
	if !sig.HasRevenue {
		add("ga4_revenue", "fail", "high", "GA4 revenue is missing; revenue upside and risk would be fabricated", "sync GA4 revenue or import trusted revenue rows", 25)
	} else {
		add("ga4_revenue", "pass", "high", "Revenue signal is present", "", 0)
	}
	if !sig.HasConversions {
		add("conversion_signal", "fail", "medium", "Conversion signal is missing or zero", "verify conversion/key-event tracking before trusting forecasts", 15)
	} else {
		add("conversion_signal", "pass", "medium", "Conversion signal is present", "", 0)
	}
	for source, coverage := range sig.SourceCoverage {
		if coverage < 0.5 {
			add("coverage_"+source, "fail", "medium", fmt.Sprintf("%s coverage is %.0f%%", source, coverage*100), "sync missing "+source+" evidence", 10)
		}
	}
	if sig.DuplicateTrackingSuspected {
		add("duplicate_tracking", "fail", "critical", "Duplicate conversion/revenue counting is suspected", "deduplicate purchase/conversion tracking before optimization", 35)
	}
	if sig.MicroConversionPollution {
		add("micro_conversion_pollution", "fail", "high", "Micro-conversions appear mixed into revenue/conversion decisions", "separate purchase/key events from micro-conversions", 25)
	}
	if sig.ConversionsDivergence > 0.25 {
		add("conversion_divergence", "fail", "high", fmt.Sprintf("Conversions diverge by %.0f%%", sig.ConversionsDivergence*100), "reconcile conversions vs all_conversions before forecasting", 25)
	}
	if report.Score < 0 {
		report.Score = 0
	}
	switch {
	case len(sig.BrokenSchemaChecks) > 0 || report.Score < 35:
		report.Level = ConfidenceBroken
	case report.Score < 60:
		report.Level = ConfidenceLow
	case report.Score < 80:
		report.Level = ConfidenceMedium
	default:
		report.Level = ConfidenceHigh
	}
	report.Summary = fmt.Sprintf("%s confidence (%d/100)", report.Level, report.Score)
	return report
}

func (r ConfidenceReport) BlocksDerivedMetrics() bool {
	return r.Level == ConfidenceLow || r.Level == ConfidenceBroken
}

func freshness(syncedAt, now time.Time) string {
	if syncedAt.IsZero() {
		return "missing"
	}
	age := now.Sub(syncedAt)
	switch {
	case age <= 48*time.Hour:
		return "fresh"
	case age <= 14*24*time.Hour:
		return "recent"
	default:
		return "stale"
	}
}

func ExtractSchemaVersion(payload map[string]any) string {
	for _, key := range []string{"schema_version", "schemaVersion"} {
		if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	for _, key := range []string{"meta", "metadata"} {
		if nested, ok := payload[key].(map[string]any); ok {
			for _, nk := range []string{"schema_version", "schemaVersion"} {
				if v, ok := nested[nk].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	return ""
}

func SupportedSchema(source, version string, supported map[string][]string) bool {
	for _, allowed := range supported[source] {
		if version == allowed {
			return true
		}
	}
	return false
}
