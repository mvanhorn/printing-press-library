package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store"
	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
	"github.com/spf13/cobra"
)

func confidenceCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "confidence", Short: "Assess source coverage, freshness, sample size, and tracking confidence", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		report := confidenceForData(d)
		return out(cmd, f, report, strings.Join(confidenceHuman(report), "\n")+"\n")
	}}
}

func confidenceForData(d store.DataSet) intelcli.ConfidenceReport {
	total := len(d.Products) + len(d.Pages) + len(d.Categories) + len(d.Emails)
	sourceCounts := map[string]int{"shopify": 0, "klaviyo": 0, "ga4": 0, "gsc": 0, "ahrefs": 0}
	hasRevenue, hasConversions, hasSearch := false, false, false
	duplicateTracking, microConversions := false, false
	add := func(sources store.MetricSources, revenue float64, conversions bool, searchClicks int, searchPosition float64) {
		if sources.Shopify.Synced {
			sourceCounts["shopify"]++
		}
		if sources.Klaviyo.Synced {
			sourceCounts["klaviyo"]++
		}
		if sources.GA4.Synced || revenue > 0 {
			sourceCounts["ga4"]++
		}
		if sources.GSC.Synced || searchClicks > 0 || searchPosition > 0 {
			sourceCounts["gsc"]++
		}
		if sources.Ahrefs.Synced {
			sourceCounts["ahrefs"]++
		}
		hasRevenue = hasRevenue || revenue > 0
		hasConversions = hasConversions || conversions
		hasSearch = hasSearch || searchClicks > 0 || searchPosition > 0
		if conversions && revenue == 0 {
			microConversions = true
		}
	}
	for _, p := range d.Products {
		if p.ConversionRate > 1 {
			duplicateTracking = true
		}
		add(p.Source, p.Revenue, p.ConversionRate > 0, p.SearchClicks, p.SearchPosition)
	}
	for _, p := range d.Pages {
		add(p.Source, p.Revenue, false, p.SearchClicks, p.SearchPosition)
	}
	for _, c := range d.Categories {
		add(c.Source, c.Revenue, false, c.SearchClicks, 0)
	}
	for _, e := range d.Emails {
		add(e.Source, e.Revenue, e.AttributedSales > 0 || e.ConversionRate > 0, 0, 0)
	}
	coverage := map[string]float64{}
	for source, count := range sourceCounts {
		if total > 0 {
			coverage[source] = float64(count) / float64(total)
		} else {
			coverage[source] = 0
		}
	}
	return intelcli.EvaluateConfidence(intelcli.ConfidenceSignals{
		Profile:                    d.Profile,
		SyncedAt:                   d.SyncedAt,
		Entities:                   total,
		SourceCoverage:             coverage,
		HasRevenue:                 hasRevenue,
		HasConversions:             hasConversions,
		HasSearchImpressions:       hasSearch,
		DuplicateTrackingSuspected: duplicateTracking,
		MicroConversionPollution:   microConversions,
		BrokenSchemaChecks:         schemaChecks(d),
	}, time.Now().UTC())
}

func schemaChecks(d store.DataSet) []string {
	if d.Source == "" || !strings.HasPrefix(d.Source, "child-cli:") {
		return nil
	}
	if d.Provenance.SchemaVersion != "ecommerce-intel.provenance/v1" {
		return []string{"ecommerce_intel_provenance"}
	}
	return nil
}

func confidenceHuman(report intelcli.ConfidenceReport) []string {
	lines := []string{fmt.Sprintf("confidence: %s (%d/100)", report.Level, report.Score), "freshness: " + report.Freshness}
	if report.BlocksDerivedMetrics() {
		lines = append(lines, "fix tracking first:")
		for _, fix := range report.FixFirst {
			lines = append(lines, "- "+fix)
		}
	}
	lines = append(lines, "checks:")
	for _, check := range report.Checks {
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", check.ID, check.Status, check.Message))
	}
	return lines
}

func derivedMetricRefusal(report intelcli.ConfidenceReport) []map[string]any {
	return []map[string]any{{
		"refused":            true,
		"reason":             "Low/Broken confidence: fix tracking first before computing derived impact metrics.",
		"confidence":         report,
		"failing_checks":     report.FailingChecks,
		"fix_tracking_first": report.FixFirst,
	}}
}

func productMissingEvidenceRow(kind string, p store.Product) map[string]any {
	missing := []string{}
	if p.SearchClicks <= 0 && p.SearchPosition <= 0 {
		missing = append(missing, "gsc_search_evidence")
	}
	if p.Revenue <= 0 {
		missing = append(missing, "ga4_revenue")
	}
	return map[string]any{
		"skipped":       true,
		"skip_reason":   "refuse-to-fabricate: missing required evidence for derived metrics",
		"metric":        kind,
		"product":       first(p.Handle, p.Title, p.URL, p.ID),
		"missing":       missing,
		"raw_evidence":  map[string]any{"search_clicks": p.SearchClicks, "search_position": p.SearchPosition, "sessions": p.Sessions, "conversion_rate": p.ConversionRate, "revenue": p.Revenue},
		"next_action":   "sync GSC search evidence and GA4 revenue before computing derived impact",
		"confidence_ok": false,
	}
}

func canComputeProductDerived(p store.Product) bool {
	return p.Revenue > 0 && (p.SearchClicks > 0 || p.SearchPosition > 0)
}

func statusHeader(d store.DataSet, command, mode string) map[string]any {
	confidence := confidenceForData(d)
	entities := len(d.Products) + len(d.Pages) + len(d.Categories) + len(d.Emails)
	requested, used, reason := analysisWindow(entities)
	return map[string]any{
		"profile":          d.Profile,
		"command":          command,
		"mode":             mode,
		"synced_at":        d.SyncedAt,
		"date_range":       d.Provenance.DateRange,
		"date_range_used":  used,
		"range_fallback":   map[string]any{"requested": requested, "used": used, "reason": reason},
		"source_coverage":  confidence.SourceCoverage,
		"confidence_level": confidence.Level,
		"confidence_score": confidence.Score,
	}
}

func statusHuman(header map[string]any) []string {
	fallback := header["range_fallback"].(map[string]any)
	return []string{
		fmt.Sprintf("profile: %s", header["profile"]),
		fmt.Sprintf("mode: %s", header["mode"]),
		fmt.Sprintf("date range used: %s (%s)", header["date_range_used"], fallback["reason"]),
		fmt.Sprintf("confidence: %s/%v", header["confidence_level"], header["confidence_score"]),
		fmt.Sprintf("source coverage: %v", header["source_coverage"]),
	}
}

func analysisWindow(entities int) (string, string, string) {
	requested := "last_30d"
	switch {
	case entities < 3:
		return requested, "last_12mo", "thin sample; widened from 30d to 12mo"
	case entities < 10:
		return requested, "last_90d", "thin sample; widened from 30d to 90d"
	default:
		return requested, requested, "30d sample is sufficient"
	}
}

func decorateOpportunityRows(rows []map[string]any, confidence intelcli.ConfidenceReport) {
	for _, row := range rows {
		tier := "Refinement"
		deps := []string{}
		switch row["type"] {
		case "inventory", "revenue-drop":
			tier = "Fix-first"
		case "email":
			tier = "Quick-win"
		case "geo":
			tier = "Strategic"
		}
		if confidence.BlocksDerivedMetrics() {
			deps = append(deps, "close tracking gap before trusting forecast magnitude")
		}
		if row["type"] == "geo" {
			deps = append(deps, "confirm product facts and structured data before scaling content recommendations")
		}
		row["tier"] = tier
		row["dependencies"] = deps
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := tierOrder(fmt.Sprint(rows[i]["tier"])), tierOrder(fmt.Sprint(rows[j]["tier"]))
		if left == right {
			return asFloat(rows[i]["impact"]) > asFloat(rows[j]["impact"])
		}
		return left < right
	})
}

func tierOrder(tier string) int {
	switch tier {
	case "Fix-first":
		return 0
	case "Quick-win":
		return 1
	case "Strategic":
		return 2
	default:
		return 3
	}
}
