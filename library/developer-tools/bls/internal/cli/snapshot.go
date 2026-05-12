package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bls/internal/blsdata"

	"github.com/spf13/cobra"
)

// SnapshotRow is one indicator returned by `snapshot macro`.
type SnapshotRow struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Year     string `json:"year,omitempty"`
	Period   string `json:"period,omitempty"`
	Value    string `json:"value,omitempty"`
	MoM      string `json:"mom_pct,omitempty"`
	YoY      string `json:"yoy_pct,omitempty"`
	Footnote string `json:"footnote,omitempty"`
}

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Curated single-shot views of BLS data (macro snapshot, etc.).",
	}
	cmd.AddCommand(newSnapshotMacroCmd(flags))
	return cmd
}

func newSnapshotMacroCmd(flags *rootFlags) *cobra.Command {
	var startYear, endYear string
	cmd := &cobra.Command{
		Use:   "macro",
		Short: "Current state of the U.S. macro economy in one command (15 headline indicators).",
		Long: `Batch-fetches the 15 series that define the U.S. macro picture in a single POST:
headline + core CPI, food/energy/shelter components, U-3 unemployment, labor force
participation, payrolls, average hourly earnings, JOLTS openings and quits rate,
PPI final demand, ECI, and nonfarm business productivity.

With a registered BLS_API_KEY the call requests server-side YoY/MoM percent-change
columns (calculations: true). Without a key, only the value column is returned.`,
		Example: `  bls-pp-cli snapshot macro
  bls-pp-cli snapshot macro --csv > macro.csv
  bls-pp-cli snapshot macro --json --select id,label,value,yoy_pct`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			indicators := blsdata.MacroSnapshot()
			ids := make([]string, 0, len(indicators))
			for _, i := range indicators {
				ids = append(ids, i.ID)
			}
			body := map[string]any{
				"seriesid":     ids,
				"calculations": true,
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
			rows := parseSnapshotResponse(raw, indicators)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				outRaw, _ := json.Marshal(rows)
				return printOutputWithFlags(cmd.OutOrStdout(), outRaw, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					m = append(m, map[string]any{
						"id":      r.ID,
						"label":   r.Label,
						"period":  r.Year + " " + r.Period,
						"value":   r.Value,
						"mom_pct": r.MoM,
						"yoy_pct": r.YoY,
					})
				}
				if err := printAutoTable(cmd.OutOrStdout(), m); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "\nServer-side YoY/MoM is only returned with a registered BLS_API_KEY.")
				return nil
			}
			outRaw, _ := json.Marshal(rows)
			return printOutputWithFlags(cmd.OutOrStdout(), outRaw, flags)
		},
	}
	cmd.Flags().StringVar(&startYear, "start", "", "Earliest year for the snapshot batch (defaults to BLS's default window).")
	cmd.Flags().StringVar(&endYear, "end", "", "Latest year for the snapshot batch.")
	return cmd
}

// parseSnapshotResponse pulls the latest observation + calc columns for
// each indicator out of a BLS multi-series response.
func parseSnapshotResponse(raw json.RawMessage, indicators []blsdata.SnapshotEntry) []SnapshotRow {
	out := make([]SnapshotRow, 0, len(indicators))
	labelByID := make(map[string]string, len(indicators))
	for _, i := range indicators {
		labelByID[i.ID] = i.Label
	}
	var env struct {
		Results struct {
			Series []struct {
				SeriesID string `json:"seriesID"`
				Data     []struct {
					Year       string `json:"year"`
					Period     string `json:"period"`
					PeriodName string `json:"periodName"`
					Value      string `json:"value"`
					Footnotes  []struct {
						Code string `json:"code"`
						Text string `json:"text"`
					} `json:"footnotes"`
					Calculations struct {
						NetChanges map[string]string `json:"net_changes"`
						PctChanges map[string]string `json:"pct_changes"`
					} `json:"calculations"`
				} `json:"data"`
			} `json:"series"`
		} `json:"Results"`
	}
	_ = json.Unmarshal(raw, &env)
	// BLS doesn't guarantee order; index series by ID and reassemble in the
	// curated order so the macro snapshot is deterministic.
	byID := make(map[string]int, len(env.Results.Series))
	for i, s := range env.Results.Series {
		byID[s.SeriesID] = i
	}
	for _, ind := range indicators {
		idx, ok := byID[ind.ID]
		if !ok || len(env.Results.Series[idx].Data) == 0 {
			out = append(out, SnapshotRow{ID: ind.ID, Label: ind.Label})
			continue
		}
		latest := env.Results.Series[idx].Data[0]
		row := SnapshotRow{
			ID:     ind.ID,
			Label:  ind.Label,
			Year:   latest.Year,
			Period: latest.PeriodName,
			Value:  latest.Value,
			MoM:    latest.Calculations.PctChanges["1"],
			YoY:    latest.Calculations.PctChanges["12"],
		}
		if len(latest.Footnotes) > 0 {
			codes := make([]string, 0, len(latest.Footnotes))
			for _, f := range latest.Footnotes {
				if f.Code != "" {
					codes = append(codes, f.Code)
				}
			}
			row.Footnote = strings.Join(codes, ",")
		}
		out = append(out, row)
	}
	return out
}
