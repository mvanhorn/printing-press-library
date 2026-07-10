// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type sectorRotationEntry struct {
	Sector       string  `json:"sector"`
	PriorTotal   float64 `json:"prior_total"`
	CurrentTotal float64 `json:"current_total"`
	Delta        float64 `json:"delta"`
}

type sectorRotationView struct {
	CurrentPeriod string                `json:"current_period"`
	PriorPeriod   string                `json:"prior_period"`
	Movers        []sectorRotationEntry `json:"movers"`
	Note          string                `json:"note,omitempty"`
}

// recentSectorPeriods returns the distinct synced fortnight labels ordered
// by most-recently-synced first (using the resources table's own synced_at
// column, since sector period labels like "JUNE 30, 2026" don't sort
// lexically in chronological order).
func recentSectorPeriods(dbConn *sql.DB) ([]string, error) {
	rows, err := dbConn.Query(
		`SELECT DISTINCT json_extract(data, '$.period_label') AS period
		   FROM resources WHERE resource_type = 'sector'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var period sql.NullString
		if err := rows.Scan(&period); err != nil {
			continue
		}
		if period.Valid && period.String != "" {
			labels = append(labels, period.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Sort by the actual fortnight date, not sync insertion order: multiple
	// periods synced in the same batch share near-identical synced_at
	// timestamps, which does not reliably reflect which period is more
	// recent. Labels are "MONTH DD, YYYY" (e.g. "JUNE 30, 2026").
	sort.Slice(labels, func(i, j int) bool {
		ti, oki := parseSectorPeriodLabel(labels[i])
		tj, okj := parseSectorPeriodLabel(labels[j])
		if oki && okj {
			return ti.After(tj)
		}
		return labels[i] > labels[j] // fallback: lexical, newest-looking first
	})
	return labels, nil
}

// parseSectorPeriodLabel parses "JUNE 30, 2026"-style labels. NSDL emits the
// month name uppercase; time.Parse's "January" layout only matches title
// case, so the month word is title-cased before parsing.
func parseSectorPeriodLabel(label string) (time.Time, bool) {
	parts := strings.SplitN(strings.TrimSpace(label), " ", 2)
	if len(parts) != 2 || len(parts[0]) == 0 {
		return time.Time{}, false
	}
	month := strings.ToUpper(parts[0][:1]) + strings.ToLower(parts[0][1:])
	t, err := time.Parse("January 2, 2006", month+" "+parts[1])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func newNovelSectorRotationCmd(flags *rootFlags) *cobra.Command {
	var flagTop string
	var flagPeriod string

	cmd := &cobra.Command{
		Use:         "rotation",
		Short:       "Rank which sectors saw the biggest FPI inflow/outflow swing between the two most recent synced periods.",
		Example:     "  fpi-india-pp-cli sector rotation --top 5 --period latest --json",
		Long:        "Ranks sectors by the magnitude of change in net-investment total between the two most recently synced fortnights. Requires at least two 'sync --resources sector' runs across different fortnights. Use 'sector' for a single period's absolute figures instead.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank sector rotation between the two most recent synced periods")
				return nil
			}
			top := 5
			if flagTop != "" {
				n, err := strconv.Atoi(flagTop)
				if err != nil || n < 1 {
					_ = cmd.Usage()
					return usageErrf("--top must be a positive integer")
				}
				top = n
			}
			_ = flagPeriod // reserved: "latest" is the only supported value today; a specific-period selector is future scope

			db, exists, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: fpi-india-pp-cli sync --resources sector\n", defaultDBPath("fpi-india-pp-cli"))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			defer db.Close()

			periods, err := recentSectorPeriods(db.DB())
			if err != nil {
				return fmt.Errorf("reading synced sector periods: %w", err)
			}
			if len(periods) < 2 {
				view := sectorRotationView{Note: "fewer than 2 distinct synced sector periods; run sync --resources sector again after a new fortnight is published to compare"}
				if len(periods) == 1 {
					view.CurrentPeriod = periods[0]
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			currentPeriod, priorPeriod := periods[0], periods[1]

			all, err := loadSectorSnapshots(db)
			if err != nil {
				return fmt.Errorf("reading synced sector data: %w", err)
			}

			currentTotals := map[string]float64{}
			priorTotals := map[string]float64{}
			for _, r := range all {
				total, ok := sectorTotal(r.Fields)
				if !ok {
					continue
				}
				switch r.PeriodLabel {
				case currentPeriod:
					currentTotals[r.SectorName] = total
				case priorPeriod:
					priorTotals[r.SectorName] = total
				}
			}

			var movers []sectorRotationEntry
			for sector, cur := range currentTotals {
				prior := priorTotals[sector]
				movers = append(movers, sectorRotationEntry{
					Sector: sector, PriorTotal: prior, CurrentTotal: cur, Delta: cur - prior,
				})
			}
			sort.Slice(movers, func(i, j int) bool { return abs(movers[i].Delta) > abs(movers[j].Delta) })
			if len(movers) > top {
				movers = movers[:top]
			}

			view := sectorRotationView{CurrentPeriod: currentPeriod, PriorPeriod: priorPeriod, Movers: movers}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			for _, m := range view.Movers {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.0f -> %.0f (%+.0f)\n", m.Sector, m.PriorTotal, m.CurrentTotal, m.Delta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTop, "top", "", "Number of sectors to return, ranked by magnitude of change (default 5)")
	cmd.Flags().StringVar(&flagPeriod, "period", "", "Reserved for future period selection; currently always compares the two most recently synced fortnights")
	return cmd
}
