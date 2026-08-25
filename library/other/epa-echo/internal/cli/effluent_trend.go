// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through echoGet in echo_novel_support.go

package cli

import "github.com/spf13/cobra"

func newNovelEffluentTrendCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "trend FACILITY_ID",
		Short:       "Return ECHO's reported Clean Water Act effluent compliance and exceedance records for one stable facility identifier.",
		Example:     "epa-echo-pp-cli effluent trend 110009441979 --agent",
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
			sections := selectedSections(report, "CWAEffluentCompliance", "CWAEffluentALRExceedences", "CWAEffluentComplianceEXP", "CWAEffluentALRExceedencesEXP", "DmrPollLoads", "CWA3YrCompliance")
			records := normalizedSectionRecords(sections)
			var exceedanceCount any
			if len(records) > 0 {
				exceedanceCount = explicitExceedanceCount(records)
			}
			return emitECHO(cmd, flags, "live", map[string]any{"facility_id": args[0], "summary": map[string]any{"data_available": len(records) > 0, "normalized_record_count": len(records), "explicit_exceedance_count": exceedanceCount}, "records": records, "source_sections": sections, "interpretation": "Normalized fields are copied mechanically from ECHO keys. explicit_exceedance_count is null when no records are available and otherwise counts only explicit positive exceedance fields; missing sections or fields are not zero.", "caveats": echoCaveats()})
		},
	}
	return cmd
}
