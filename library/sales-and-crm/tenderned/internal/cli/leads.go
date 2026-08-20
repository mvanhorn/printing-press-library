package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newLeadsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, until, cpv string
	var national bool
	var maxValue int
	var limit int

	cmd := &cobra.Command{
		Use:   "leads",
		Short: "Sub-threshold lead finder — national-only notices in a CPV stripe that EU TED never sees",
		Long: `Find Dutch tender notices that are TenderNed-only — national-scope (sub-EU-threshold)
notices below an estimated-value cap inside a CPV stripe. This is the long tail
TED never publishes; for many Dutch SMEs and municipality-focused suppliers,
this is the entire market.

Operates on the local SQLite snapshot. Run 'tenderned-pp-cli sync' first.

Different from 'eu-tenders leads' (which surfaces recent TED award winners);
both commands are intentionally named the same so the two CLIs feel parallel.`,
		Example: "  tenderned-pp-cli leads --national --cpv 45000000-7,71000000-8 --since 2026-04-01",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			notices, err := tnLoadNotices(cmd.Context(), s)
			if err != nil {
				return err
			}
			cpvList := tnSplitCSV(cpv)
			out := []map[string]any{}
			for _, n := range notices {
				if national && !tnMatchesNational(n) {
					continue
				}
				if !tnHasCPV(n, cpvList) {
					continue
				}
				// PATCH: parse publicatieDatum via tnParseDate and compare as
				// time.Time. Raw string compare like `d > until+"T23:59:59"`
				// drops values with sub-second precision ("2026-05-18T23:59:59.000001")
				// or a "Z" tz suffix that are lexicographically > the boundary
				// but logically within the window.
				pd := tnParseDate(n.PublicatieDatum)
				if since != "" {
					if sinceT := tnParseDate(since); !sinceT.IsZero() && !pd.IsZero() && pd.Before(sinceT) {
						continue
					}
				}
				if until != "" {
					if untilT := tnParseDate(until); !untilT.IsZero() && !pd.IsZero() && pd.After(untilT.Add(24*time.Hour-time.Nanosecond)) {
						continue
					}
				}
				// Value-cap filter — most TenderNed JSON responses don't carry
				// the contract value directly; treat maxValue as informational
				// when the field isn't present. Real value lives in eForms XML.
				_ = maxValue
				lead := map[string]any{
					"publicatieId":      n.PublicatieID,
					"aanbestedingNaam":  n.AanbestedingNaam,
					"opdrachtgeverNaam": n.OpdrachtgeverNaam,
					"publicatieDatum":   n.PublicatieDatum,
					"sluitingsDatum":    n.SluitingsDatum,
					"publicatieCode":    n.PublicatieCode,
					"scope":             n.NationaalOfEuropees,
					"cpv":               n.CPVCodes,
				}
				out = append(out, lead)
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			result := map[string]any{
				"leads": out,
				"count": len(out),
				"filters": map[string]any{
					"national":  national,
					"cpv":       cpvList,
					"since":     since,
					"until":     until,
					"max_value": maxValue,
				},
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d sub-threshold leads.\n", len(out))
			for _, l := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %v | %s | %v\n", l["publicatieId"], l["opdrachtgeverNaam"], l["aanbestedingNaam"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Latest publication date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes (full dash form, e.g. 45000000-7,71000000-8)")
	cmd.Flags().BoolVar(&national, "national", false, "Only national-scope (sub-EU-threshold) notices — TED never sees these")
	cmd.Flags().IntVar(&maxValue, "max-value", 0, "Estimated-value cap in EUR (informational; per-notice value is in eForms XML)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Cap on results")
	return cmd
}
