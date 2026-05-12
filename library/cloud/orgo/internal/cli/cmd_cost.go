// Hand-authored: novel feature `cost` — reconstruct per-workspace
// running-hours from local action timestamps and multiply by a tier
// rate table to estimate burn. Created during Phase 3 build; survives
// regen because it lives in its own file.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

// tierRates maps "<cpu>cpu_<ram>gb" to a placeholder per-hour USD rate.
// These are deliberately public-facing approximations; the brief notes
// that real rates require Orgo billing data the CLI does not have. The
// command's --help/Long advertises this so users don't read the output
// as authoritative billing.
var tierRates = map[string]float64{
	"1cpu_4gb":   0.05,
	"2cpu_8gb":   0.10,
	"4cpu_16gb":  0.20,
	"8cpu_32gb":  0.40,
	"16cpu_64gb": 0.80,
}

const defaultTierRate = 0.05

func newCostCmd(flags *rootFlags) *cobra.Command {
	var workspace string
	var since string
	var forecast bool

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Reconstruct per-workspace running-hours from local action timestamps and apply a tier rate to estimate burn.",
		Long: `Estimate per-workspace cost by reconstructing running-hours from
the local actions ledger.

Running-hours per computer are approximated as the time delta between
the first and last recorded action inside the window. This is a CRUDE
approximation — real running-hours need the API's status-transition
history, which this CLI cannot inspect. Use the output for ranking and
trend-spotting, not for invoicing.

Tier rates are placeholders defined in the command source:
  1 CPU /  4 GB  ->  $0.05/h
  2 CPU /  8 GB  ->  $0.10/h
  4 CPU / 16 GB  ->  $0.20/h
  8 CPU / 32 GB  ->  $0.40/h
 16 CPU / 64 GB  ->  $0.80/h
Anything outside this map falls back to $0.05/h.`,
		Example: strings.Trim(`
  # 30-day rollup
  orgo-pp-cli cost --since 30d

  # Project to month-end based on month-to-date burn rate
  orgo-pp-cli cost --since 30d --forecast

  # JSON for piping into a finance dashboard
  orgo-pp-cli cost --since 7d --agent --select workspace,total_cost_usd
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			windowSince, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			computerRows, source, err := loadFleetComputers(flags, db)
			if err != nil {
				return fmt.Errorf("loading computers: %w", err)
			}
			if len(computerRows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No computers found locally or via the live API.")
				return notFoundErr(fmt.Errorf("no computers found"))
			}
			_ = source // provenance label, unused for now

			type wsAgg struct {
				Workspace      string  `json:"workspace"`
				ComputerCount  int     `json:"computer_count"`
				TotalHours     float64 `json:"total_hours"`
				TotalCostUSD   float64 `json:"total_cost_usd"`
				ProjectedMonth float64 `json:"projected_month_end_usd,omitempty"`
			}
			agg := map[string]*wsAgg{}

			for _, raw := range computerRows {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				id := stringField(obj, "id")
				if id == "" {
					continue
				}
				wsID := stringField(obj, "workspace_id")
				if wsID == "" {
					wsID = "(unassigned)"
				}
				if workspace != "" && wsID != workspace {
					continue
				}
				cpu := intField(obj, "cpu")
				ram := intField(obj, "ram")
				rateKey := fmt.Sprintf("%dcpu_%dgb", cpu, ram)
				rate, ok := tierRates[rateKey]
				if !ok {
					rate = defaultTierRate
				}

				// Approximate running-hours from action span within window.
				actions, err := db.QueryActions(cmd.Context(), store.ActionFilter{
					ComputerID: id,
					Since:      windowSince,
					Limit:      10000,
				})
				if err != nil || len(actions) == 0 {
					continue
				}

				// QueryActions returns DESC; first slot is newest, last is oldest.
				newest := actions[0].Timestamp
				oldest := actions[len(actions)-1].Timestamp
				hours := newest.Sub(oldest).Hours()
				if hours < 0 {
					hours = 0
				}

				w, ok := agg[wsID]
				if !ok {
					w = &wsAgg{Workspace: wsID}
					agg[wsID] = w
				}
				w.ComputerCount++
				w.TotalHours += hours
				w.TotalCostUSD += hours * rate
			}

			rows := make([]wsAgg, 0, len(agg))
			for _, w := range agg {
				rows = append(rows, *w)
			}

			if forecast {
				now := time.Now().UTC()
				daysIn := daysInMonth(now)
				daysSoFar := float64(now.Day())
				if daysSoFar < 1 {
					daysSoFar = 1
				}
				ratio := float64(daysIn) / daysSoFar
				for i := range rows {
					rows[i].ProjectedMonth = rows[i].TotalCostUSD * ratio
				}
			}

			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].TotalCostUSD > rows[j].TotalCostUSD
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No actions in the window — nothing to cost out.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			header := bold("WORKSPACE") + "\t" + bold("COMPUTERS") + "\t" + bold("HOURS") + "\t" + bold("USD")
			if forecast {
				header += "\t" + bold("MONTH_END_USD")
			}
			fmt.Fprintln(tw, header)
			for _, r := range rows {
				if forecast {
					fmt.Fprintf(tw, "%s\t%d\t%.2f\t$%.2f\t$%.2f\n",
						truncate(r.Workspace, 20), r.ComputerCount, r.TotalHours, r.TotalCostUSD, r.ProjectedMonth)
				} else {
					fmt.Fprintf(tw, "%s\t%d\t%.2f\t$%.2f\n",
						truncate(r.Workspace, 20), r.ComputerCount, r.TotalHours, r.TotalCostUSD)
				}
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter by workspace ID")
	cmd.Flags().StringVar(&since, "since", "30d", "Time window (e.g. 7d, 30d)")
	cmd.Flags().BoolVar(&forecast, "forecast", false, "Add a month-end projection based on month-to-date burn rate")

	return cmd
}

func daysInMonth(t time.Time) int {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	next := first.AddDate(0, 1, 0)
	return int(next.Sub(first).Hours() / 24)
}
