package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newDeadlineHeatCmd(flags *rootFlags) *cobra.Command {
	var dbPath, cpv string
	var days, limit int

	cmd := &cobra.Command{
		Use:   "deadline-heat",
		Short: "Ranked calendar of expiring notices weighted by urgency × procedure-weight (mirrors eu-tenders deadline-heat)",
		Long: `Ranked list of notices closing in the next N days, weighted by urgency
(1/days-remaining) and a procedure-weight (open=1.0, restricted=0.7,
negotiated=0.4). Higher score = more time-sensitive triage.`,
		Example: "  tenderned-pp-cli deadline-heat --days 14",
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
			now := time.Now().UTC()
			horizon := now.Add(time.Duration(days) * 24 * time.Hour)

			type heat struct {
				PubID    string  `json:"publicatieId"`
				Title    string  `json:"title"`
				Buyer    string  `json:"buyer"`
				Closing  string  `json:"closing"`
				DaysLeft int     `json:"daysLeft"`
				Score    float64 `json:"score"`
			}
			var out []heat
			for _, n := range notices {
				if !tnHasCPV(n, cpvList) {
					continue
				}
				cl := tnParseDate(n.SluitingsDatum)
				if cl.IsZero() || cl.Before(now) || cl.After(horizon) {
					continue
				}
				dayLeft := cl.Sub(now).Hours() / 24
				if dayLeft < 0.5 {
					dayLeft = 0.5
				}
				w := procedureWeight(n.ProcedureCode.Code)
				score := w * (1.0 / dayLeft)
				out = append(out, heat{
					PubID:    n.PublicatieID.String(),
					Title:    n.AanbestedingNaam,
					Buyer:    n.OpdrachtgeverNaam,
					Closing:  cl.Format("2006-01-02"),
					DaysLeft: int(dayLeft),
					Score:    score,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			result := map[string]any{
				"heat":  out,
				"count": len(out),
				"days":  days,
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top %d urgent notices (next %d days):\n", len(out), days)
			for _, h := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %6.3f | %s | %3dd | %s | %s\n", h.Score, h.Closing, h.DaysLeft, h.Buyer, h.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes")
	cmd.Flags().IntVar(&days, "days", 14, "Horizon in days")
	cmd.Flags().IntVar(&limit, "limit", 25, "Limit on returned results")
	return cmd
}

func procedureWeight(code string) float64 {
	switch code {
	case "OPB", "OP":
		return 1.0
	case "NOP", "NIET-OPENBAAR":
		return 0.7
	case "CCD":
		return 0.6
	case "ONP":
		return 0.4
	default:
		return 0.5
	}
}
