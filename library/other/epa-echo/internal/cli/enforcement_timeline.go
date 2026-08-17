// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through echoGet in echo_novel_support.go

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelEnforcementTimelineCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:         "timeline FACILITY_ID",
		Short:       "Collect dated inspection, violation, formal-action, and enforcement evidence from a detailed report.",
		Example:     "epa-echo-pp-cli enforcement timeline 110009441979 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			report, err := getDFR(ctx, flags, args[0])
			if err != nil {
				return err
			}
			lookback, err := parseLookback(since)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			cutoff := time.Time{}
			if lookback > 0 {
				cutoff = time.Now().UTC().Add(-lookback).Truncate(24 * time.Hour)
			}
			entries := filterTimelineSince(timelineEntries(report), cutoff)
			return emitECHO(cmd, flags, "live", map[string]any{"facility_id": args[0], "since": fmt.Sprint(since), "cutoff": func() any {
				if cutoff.IsZero() {
					return nil
				}
				return cutoff.Format("2006-01-02")
			}(), "timeline": entries, "source_sections": selectedSections(report, "InspectionEnforcementSummary", "ComplianceHistory", "ViolationsEnforcementActions", "FormalActions", "ICISFormalActions", "CaseFormalActions", "EnforcementComplianceSummaries"), "interpretation": "Chronology is descriptive; sequence does not establish causation. identifiers contains only IDs or numbers present on each source record.", "caveats": echoCaveats()})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only include dated records in this lookback window (for example 30d or 5y)")
	return cmd
}
