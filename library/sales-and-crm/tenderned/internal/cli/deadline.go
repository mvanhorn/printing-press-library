package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDeadlineCmd(flags *rootFlags) *cobra.Command {
	var dbPath, cpv, buyer, kind string
	var within int

	cmd := &cobra.Command{
		Use:   "deadline",
		Short: "List open notices closing within N days (mirrors eu-tenders deadline)",
		Long: `List notices with a sluitingsDatum (closing date) inside the next N days,
optionally filtered by CPV, buyer, or contract kind. Reads the local SQLite
snapshot. Mirrors eu-tenders deadline.`,
		Example: "  tenderned-pp-cli deadline --within 14 --cpv 45000000-7",
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
			horizon := now.Add(time.Duration(within) * 24 * time.Hour)

			type row struct {
				PubID    string    `json:"publicatieId"`
				Title    string    `json:"title"`
				Buyer    string    `json:"buyer"`
				Closing  time.Time `json:"closing"`
				DaysLeft int       `json:"daysLeft"`
				Code     string    `json:"publicatieCode"`
			}
			var rows []row
			for _, n := range notices {
				if buyer != "" && !strings.Contains(strings.ToLower(n.OpdrachtgeverNaam), strings.ToLower(buyer)) {
					continue
				}
				if kind != "" && !strings.EqualFold(n.TypeOpdrachtCode.Code, kind) {
					continue
				}
				if !tnHasCPV(n, cpvList) {
					continue
				}
				cl := tnParseDate(n.SluitingsDatum)
				if cl.IsZero() {
					continue
				}
				if cl.Before(now) || cl.After(horizon) {
					continue
				}
				rows = append(rows, row{
					PubID:    n.PublicatieID.String(),
					Title:    n.AanbestedingNaam,
					Buyer:    n.OpdrachtgeverNaam,
					Closing:  cl,
					DaysLeft: int(cl.Sub(now).Hours() / 24),
					Code:     n.PublicatieCode,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Closing.Before(rows[j].Closing) })

			result := map[string]any{
				"deadlines": rows,
				"count":     len(rows),
				"within":    within,
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Closing within %d days: %d notices\n", within, len(rows))
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s | %3d d | %s | %s\n", r.Closing.Format("2006-01-02"), r.DaysLeft, r.Buyer, r.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&within, "within", 14, "Days from now to include in the window")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes (full dash form)")
	cmd.Flags().StringVar(&buyer, "buyer", "", "Substring match on buyer name")
	cmd.Flags().StringVar(&kind, "kind", "", "Contract type: D=services, L=supplies, W=works")
	return cmd
}
