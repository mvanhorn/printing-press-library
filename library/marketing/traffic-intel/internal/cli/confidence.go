package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
	"github.com/mvanhorn/printing-press-library/library/marketing/traffic-intel/internal/store"
	"github.com/spf13/cobra"
)

func confidenceCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "confidence", Short: "Assess source coverage, freshness, sample size, and tracking confidence", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		report := confidenceForData(d)
		lines := confidenceHuman(report)
		return out(cmd, f, report, strings.Join(lines, "\n")+"\n")
	}}
}

func confidenceForData(d store.DataSet) intelcli.ConfidenceReport {
	total := len(d.Pages)
	sourceCounts := map[string]int{"gsc": 0, "ga4": 0, "ahrefs": 0}
	hasRevenue, hasConversions, hasImpressions := false, false, false
	duplicateTracking, microConversions := false, false
	for _, p := range d.Pages {
		if p.Sources.GSC.ChildCLICommand != "" || p.Impressions > 0 || p.Clicks > 0 {
			sourceCounts["gsc"]++
		}
		if p.Sources.GA4.ChildCLICommand != "" || p.Sessions > 0 || p.Revenue > 0 || p.Conversions > 0 {
			sourceCounts["ga4"]++
		}
		if p.Sources.Ahrefs.ChildCLICommand != "" || p.Backlinks > 0 || p.RefDomains > 0 {
			sourceCounts["ahrefs"]++
		}
		hasRevenue = hasRevenue || p.Revenue > 0
		hasConversions = hasConversions || p.Conversions > 0
		hasImpressions = hasImpressions || p.Impressions > 0
		if p.Sessions > 0 && p.Conversions > p.Sessions {
			duplicateTracking = true
		}
		if p.Conversions > 0 && p.Revenue == 0 {
			microConversions = true
		}
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
		HasSearchImpressions:       hasImpressions,
		DuplicateTrackingSuspected: duplicateTracking,
		MicroConversionPollution:   microConversions,
		BrokenSchemaChecks:         schemaChecks(d),
	}, time.Now().UTC())
}

func schemaChecks(d store.DataSet) []string {
	if d.Source == "" || !strings.HasPrefix(d.Source, "child-cli:") {
		return nil
	}
	if d.Provenance.SchemaVersion != "traffic-intel.provenance/v1" {
		return []string{"traffic_intel_provenance"}
	}
	return nil
}

func confidenceHuman(report intelcli.ConfidenceReport) []string {
	lines := []string{
		fmt.Sprintf("confidence: %s (%d/100)", report.Level, report.Score),
		"freshness: " + report.Freshness,
	}
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

func missingEvidenceRow(kind string, p store.PageMetrics) map[string]any {
	missing := []string{}
	if p.Impressions <= 0 {
		missing = append(missing, "gsc_impressions")
	}
	if p.Revenue <= 0 {
		missing = append(missing, "ga4_revenue")
	}
	return map[string]any{
		"skipped":       true,
		"skip_reason":   "refuse-to-fabricate: missing required evidence for derived metrics",
		"metric":        kind,
		"url":           p.URL,
		"title":         p.Title,
		"missing":       missing,
		"raw_evidence":  map[string]any{"impressions": p.Impressions, "clicks": p.Clicks, "sessions": p.Sessions, "conversions": p.Conversions, "revenue": p.Revenue},
		"next_action":   "sync GSC impressions and GA4 revenue before computing derived impact",
		"confidence_ok": false,
	}
}

func canComputeDerived(p store.PageMetrics) bool {
	return p.Impressions > 0 && p.Revenue > 0
}
