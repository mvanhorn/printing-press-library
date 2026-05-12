package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bls/internal/blsdata"

	"github.com/spf13/cobra"
)

// SAComparisonRow pairs SA and NSA values from the same period.
type SAComparisonRow struct {
	Year       string   `json:"year"`
	Period     string   `json:"period"`
	PeriodName string   `json:"period_name"`
	ValueSA    string   `json:"value_sa"`
	ValueNSA   string   `json:"value_nsa"`
	DeltaPct   *float64 `json:"sa_minus_nsa_pct,omitempty"`
}

// SAComparison is the full output of `series compare-sa`.
type SAComparison struct {
	IDSA  string            `json:"id_sa"`
	IDNSA string            `json:"id_nsa"`
	Rows  []SAComparisonRow `json:"rows"`
	Notes []string          `json:"notes,omitempty"`
}

func newSeriesCompareSACmd(flags *rootFlags) *cobra.Command {
	var startYear, endYear string
	cmd := &cobra.Command{
		Use:   "compare-sa <seriesid>",
		Short: "Show seasonally-adjusted and not-seasonally-adjusted variants of a series side-by-side.",
		Long: `Decodes position 3 of the packed BLS series ID to derive both the SA and NSA
variants, then batch-fetches both and aligns them by (year, period). Supports the
prefixes CU, CW, CE, LN, JT, CI, WP — the surveys that publish dual-adjustment
series in a documented position-3 encoding.

Some surveys (AP, OE) publish only one variant; for those the command emits a
single-row response with a note explaining why a comparison is not available.`,
		Example: `  bls-pp-cli series compare-sa CUUR0000SA0
  bls-pp-cli series compare-sa LNS14000000 --start 2020 --end 2025 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := strings.TrimSpace(strings.ToUpper(args[0]))
			saID, nsaID := blsdata.CompareAdjustmentIDs(id)
			if saID == "" || nsaID == "" {
				return usageErr(fmt.Errorf("series %q is from a survey that does not have a documented SA/NSA toggle (supported prefixes: CU, CW, LN, JT, CE, CI, WP); comparison unavailable", id))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"seriesid": []string{saID, nsaID},
			}
			if startYear != "" {
				body["startyear"] = startYear
			}
			if endYear != "" {
				body["endyear"] = endYear
			}
			body = injectRegistrationKey(body)
			raw, _, err := c.Post("/timeseries/data/", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := parseSACompare(raw, saID, nsaID)
			out := SAComparison{IDSA: saID, IDNSA: nsaID, Rows: rows}
			if len(rows) == 0 {
				out.Notes = append(out.Notes, "BLS returned no data for one or both variants; the prefix may not have an active NSA/SA pair for these IDs")
			}
			outRaw, _ := json.Marshal(out)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlags(cmd.OutOrStdout(), outRaw, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					m = append(m, map[string]any{
						"year":   r.Year,
						"period": r.PeriodName,
						"sa":     r.ValueSA,
						"nsa":    r.ValueNSA,
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "SA: %s   NSA: %s\n", saID, nsaID)
				if len(m) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "(no aligned rows; try widening --start/--end)")
					return nil
				}
				return printAutoTable(cmd.OutOrStdout(), m)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), outRaw, flags)
		},
	}
	cmd.Flags().StringVar(&startYear, "start", "", "Earliest year (default: BLS default window).")
	cmd.Flags().StringVar(&endYear, "end", "", "Latest year.")
	return cmd
}

func parseSACompare(raw json.RawMessage, saID, nsaID string) []SAComparisonRow {
	var env struct {
		Results struct {
			Series []struct {
				SeriesID string `json:"seriesID"`
				Data     []struct {
					Year       string `json:"year"`
					Period     string `json:"period"`
					PeriodName string `json:"periodName"`
					Value      string `json:"value"`
				} `json:"data"`
			} `json:"series"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	type key struct{ Year, Period string }
	sa := make(map[key]string)
	nsa := make(map[key]string)
	periodName := make(map[key]string)
	for _, s := range env.Results.Series {
		dest := sa
		if s.SeriesID == nsaID {
			dest = nsa
		} else if s.SeriesID != saID {
			continue
		}
		for _, d := range s.Data {
			k := key{Year: d.Year, Period: d.Period}
			dest[k] = d.Value
			periodName[k] = d.PeriodName
		}
	}
	var out []SAComparisonRow
	for k := range sa {
		if _, ok := nsa[k]; !ok {
			continue
		}
		out = append(out, SAComparisonRow{
			Year:       k.Year,
			Period:     k.Period,
			PeriodName: periodName[k],
			ValueSA:    sa[k],
			ValueNSA:   nsa[k],
		})
	}
	// sort newest first by (year, period) descending
	sortRowsDesc(out)
	return out
}

func sortRowsDesc(rows []SAComparisonRow) {
	if len(rows) < 2 {
		return
	}
	// Simple insertion sort (rows count is small)
	for i := 1; i < len(rows); i++ {
		j := i
		for j > 0 && rowLess(rows[j-1], rows[j]) {
			rows[j-1], rows[j] = rows[j], rows[j-1]
			j--
		}
	}
}

func rowLess(a, b SAComparisonRow) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	return a.Period < b.Period
}
