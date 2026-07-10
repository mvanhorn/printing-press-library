// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type limitsUtilizationEntry struct {
	ISIN             string `json:"isin"`
	Issuer           string `json:"issuer"`
	NRILimitPct      string `json:"nri_limit_pct"`
	FPILimitPct      string `json:"fpi_limit_pct"`
	SectoralCapPct   string `json:"sectoral_cap_pct"`
	MonitoredHolding string `json:"monitored_holding"`
	Remarks          string `json:"remarks"`
	ReportingDate    string `json:"reporting_date"`
}

type limitsUtilizationView struct {
	Query   string                   `json:"query"`
	Matches []limitsUtilizationEntry `json:"matches"`
	Note    string                   `json:"note,omitempty"`
}

func newNovelLimitsUtilizationCmd(flags *rootFlags) *cobra.Command {
	var flagSector string

	cmd := &cobra.Command{
		Use:     "utilization",
		Short:   "See what percentage of the regulatory FPI investment cap a sector or debt category has used.",
		Example: "  fpi-india-pp-cli limits utilization --sector Banking --json",
		Long: "Matches companies by issuer name against --sector (NSDL's limit-monitoring data is per-ISIN, not per official sector taxonomy, " +
			"so this is a name-substring match) and reports each match's NRI/FPI/sectoral cap percentages, current monitored FPI holding count, " +
			"and any regulatory remarks. Requires 'sync --resources limits' first. NSDL does not publish total outstanding shares alongside this " +
			"data, so a single derived percent-of-cap-used figure is not computable from the source; the declared caps and monitored holding are " +
			"reported as-is so the reader can judge proximity to the cap themselves.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would look up FPI limit status")
				return nil
			}
			if flagSector == "" {
				_ = cmd.Usage()
				return usageErrf("--sector is required (a company name or sector-ish keyword to search, e.g. --sector Banking)")
			}

			db, exists, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: fpi-india-pp-cli sync --resources limits\n", defaultDBPath("fpi-india-pp-cli"))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			defer db.Close()

			recs, err := loadLimitsMatching(db, flagSector)
			if err != nil {
				return fmt.Errorf("reading synced limits data: %w", err)
			}

			view := limitsUtilizationView{Query: flagSector}
			for _, r := range recs {
				view.Matches = append(view.Matches, limitsUtilizationEntry{
					ISIN: r.ISIN, Issuer: r.Issuer, NRILimitPct: r.NRILimit, FPILimitPct: r.FPILimit,
					SectoralCapPct: r.SectoralCap, MonitoredHolding: r.MonitoredLimit,
					Remarks: r.Remarks, ReportingDate: r.ReportingDate,
				})
			}
			if len(view.Matches) == 0 {
				view.Note = fmt.Sprintf("no companies matched %q in synced limits data; try a broader search term or run sync --resources limits --full", flagSector)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			for _, m := range view.Matches {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\tFPI cap %s%%\tsectoral cap %s%%\tmonitored %s\n", m.Issuer, m.FPILimitPct, m.SectoralCapPct, m.MonitoredHolding)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSector, "sector", "", "Company name or sector-ish keyword to search (matches issuer name or remarks)")
	return cmd
}
