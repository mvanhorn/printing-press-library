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
	// PriorMissing is true when the sector has no entry in the prior
	// period's snapshot (NSDL added, renamed, or reclassified it since
	// then), so Delta reflects a classification change rather than a real
	// inflow/outflow swing.
	PriorMissing bool `json:"prior_missing,omitempty"`
	// CurrentMissing is true when a sector present in the prior period's
	// snapshot has no entry in the current period (removed, renamed, or
	// merged into another sector by NSDL). CurrentTotal is 0 in this case,
	// same caveat as PriorMissing applies in the opposite direction.
	CurrentMissing bool `json:"current_missing,omitempty"`
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
				prior, hadPrior := priorTotals[sector]
				movers = append(movers, sectorRotationEntry{
					Sector: sector, PriorTotal: prior, CurrentTotal: cur, Delta: cur - prior,
					PriorMissing: !hadPrior,
				})
			}
			// Sectors present in the prior period but absent from the
			// current one (removed, renamed, or merged by NSDL) never show
			// up in the currentTotals loop above; without this, that
			// classification change is silently invisible instead of just
			// excluded-from-ranking like the PriorMissing case.
			for sector, prior := range priorTotals {
				if _, stillPresent := currentTotals[sector]; stillPresent {
					continue
				}
				movers = append(movers, sectorRotationEntry{
					Sector: sector, PriorTotal: prior, CurrentTotal: 0, Delta: -prior,
					CurrentMissing: true,
				})
			}
			// Rank by real period-over-period flow change only. A sector
			// absent from one of the two periods isn't a flow swing — it's
			// NSDL adding, removing, renaming, or reclassifying a sector —
			// so ranking it by abs(0 -> current) or abs(prior -> 0) would
			// misreport a classification event as the biggest mover.
			// Surface those separately instead of silently mixing them
			// into the ranking (or, for CurrentMissing, dropping them
			// entirely).
			var ranked, changedSectors []sectorRotationEntry
			for _, m := range movers {
				if m.PriorMissing || m.CurrentMissing {
					changedSectors = append(changedSectors, m)
					continue
				}
				ranked = append(ranked, m)
			}
			sort.Slice(ranked, func(i, j int) bool { return abs(ranked[i].Delta) > abs(ranked[j].Delta) })
			if len(ranked) > top {
				ranked = ranked[:top]
			}
			movers = ranked
			view := sectorRotationView{CurrentPeriod: currentPeriod, PriorPeriod: priorPeriod, Movers: movers}
			if len(changedSectors) > 0 {
				sort.Slice(changedSectors, func(i, j int) bool { return changedSectors[i].Sector < changedSectors[j].Sector })
				var newNames, goneNames []string
				for _, m := range changedSectors {
					if m.PriorMissing {
						newNames = append(newNames, m.Sector)
					} else {
						goneNames = append(goneNames, m.Sector)
					}
				}
				var parts []string
				if len(newNames) > 0 {
					parts = append(parts, fmt.Sprintf("new in current period (no prior-period data): %s", strings.Join(newNames, ", ")))
				}
				if len(goneNames) > 0 {
					parts = append(parts, fmt.Sprintf("absent from current period (no current-period data): %s", strings.Join(goneNames, ", ")))
				}
				view.Note = "excluded from ranking, likely NSDL sector reclassification — " + strings.Join(parts, "; ")
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			for _, m := range view.Movers {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.0f -> %.0f (%+.0f)\n", m.Sector, m.PriorTotal, m.CurrentTotal, m.Delta)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTop, "top", "", "Number of sectors to return, ranked by magnitude of change (default 5)")
	cmd.Flags().StringVar(&flagPeriod, "period", "", "Reserved for future period selection; currently always compares the two most recently synced fortnights")
	return cmd
}
