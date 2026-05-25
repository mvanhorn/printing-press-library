// tesla cost ledger / cost what-if — charging cost analytics over the local
// charges table populated by sync. Hand-coded; out-of-tree from generator.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/store"
)

func newCostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Charging cost analytics: ledger and counterfactual",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCostLedgerCmd(flags))
	cmd.AddCommand(newCostWhatIfCmd(flags))
	return cmd
}

func newCostLedgerCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		group string
	)
	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Per-session cost, monthly spend, home-vs-Supercharger ratio",
		Long: `Aggregates rows in the local tesla_charges table. Run "tesla snap --all" on a
cron to capture vehicle_data snapshots, then "tesla timeline" to stitch them
into charge sessions before running this. Costs come from the upstream
CHARGING_HISTORY payload when present, or from the user-configured
tesla_tariffs table for home sessions.`,
		Example:     "  tesla-pp-cli cost ledger --since 30d --json\n  tesla-pp-cli cost ledger --since 90d --group supercharger --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}
			ctx := cmd.Context()
			sinceT, err := parseSinceDuration(since)
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, defaultDBPath("tesla-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := store.EnsureTeslaSchema(ctx, s); err != nil {
				return err
			}
			report, err := computeCostLedger(ctx, s, sinceT, group)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window start: duration (e.g. 30d, 7d, 24h) or ISO date")
	cmd.Flags().StringVar(&group, "group", "all", "Filter: all | home | supercharger")
	return cmd
}

func newCostWhatIfCmd(flags *rootFlags) *cobra.Command {
	var (
		since    string
		onlyHome bool
	)
	cmd := &cobra.Command{
		Use:         "what-if",
		Short:       "Counterfactual: \"if you only charged at home you would have saved $X\"",
		Long:        `Re-prices Supercharger sessions at your home tariff and reports the delta.`,
		Example:     "  tesla-pp-cli cost what-if --only-home --since 90d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}
			ctx := cmd.Context()
			sinceT, err := parseSinceDuration(since)
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, defaultDBPath("tesla-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := store.EnsureTeslaSchema(ctx, s); err != nil {
				return err
			}
			out, err := computeCostWhatIf(ctx, s, sinceT, onlyHome)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Window start: duration (e.g. 30d) or ISO date")
	cmd.Flags().BoolVar(&onlyHome, "only-home", true, "Re-price all Supercharger sessions at the home tariff")
	return cmd
}

type costSession struct {
	StartedAt      string  `json:"started_at"`
	LocationLabel  string  `json:"location_label"`
	EnergyAddedKwh float64 `json:"energy_added_kwh"`
	CostUSD        float64 `json:"cost_usd"`
	CostPerKwh     float64 `json:"cost_per_kwh"`
	TariffWindow   string  `json:"tariff_window,omitempty"`
	IsSupercharger bool    `json:"is_supercharger"`
}

type costLedgerReport struct {
	Window struct {
		Since string `json:"since"`
		Until string `json:"until"`
	} `json:"window"`
	TotalKwh          float64       `json:"total_kwh"`
	TotalUSD          float64       `json:"total_usd"`
	AveragePerKwh     float64       `json:"average_per_kwh"`
	SessionCount      int           `json:"session_count"`
	HomeKwh           float64       `json:"home_kwh"`
	HomeUSD           float64       `json:"home_usd"`
	SuperchargerKwh   float64       `json:"supercharger_kwh"`
	SuperchargerUSD   float64       `json:"supercharger_usd"`
	SuperchargerRatio float64       `json:"supercharger_ratio"`
	Sessions          []costSession `json:"sessions"`
	Note              string        `json:"note,omitempty"`
}

func computeCostLedger(ctx context.Context, s *store.Store, since time.Time, group string) (*costLedgerReport, error) {
	rep := &costLedgerReport{Sessions: []costSession{}}
	rep.Window.Since = since.UTC().Format(time.RFC3339)
	rep.Window.Until = time.Now().UTC().Format(time.RFC3339)

	rows, err := s.DB().QueryContext(ctx, `
        SELECT started_at, fast_charger_type, location_label,
               energy_added_kwh, cost_usd, cost_per_kwh, tariff_window
        FROM tesla_charges
        WHERE started_at >= ?
        ORDER BY started_at DESC`, since.UTC().Format(time.RFC3339))
	if err != nil {
		return rep, fmt.Errorf("query charges: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			startedAt             sql.NullString
			fct, locLabel, tWin   sql.NullString
			energy, cost, cPerKwh sql.NullFloat64
		)
		if err := rows.Scan(&startedAt, &fct, &locLabel, &energy, &cost, &cPerKwh, &tWin); err != nil {
			continue
		}
		isSC := fct.String == "Tesla" || strings.Contains(strings.ToLower(locLabel.String), "supercharger")
		if group == "home" && isSC {
			continue
		}
		if group == "supercharger" && !isSC {
			continue
		}
		sess := costSession{
			StartedAt:      startedAt.String,
			LocationLabel:  locLabel.String,
			EnergyAddedKwh: energy.Float64,
			CostUSD:        cost.Float64,
			CostPerKwh:     cPerKwh.Float64,
			TariffWindow:   tWin.String,
			IsSupercharger: isSC,
		}
		rep.Sessions = append(rep.Sessions, sess)
		rep.TotalKwh += sess.EnergyAddedKwh
		rep.TotalUSD += sess.CostUSD
		if sess.IsSupercharger {
			rep.SuperchargerKwh += sess.EnergyAddedKwh
			rep.SuperchargerUSD += sess.CostUSD
		} else {
			rep.HomeKwh += sess.EnergyAddedKwh
			rep.HomeUSD += sess.CostUSD
		}
	}
	rep.SessionCount = len(rep.Sessions)
	if rep.TotalKwh > 0 {
		rep.AveragePerKwh = rep.TotalUSD / rep.TotalKwh
		rep.SuperchargerRatio = rep.SuperchargerKwh / rep.TotalKwh
	}
	if rep.SessionCount == 0 {
		rep.Note = "no charges in window; run 'tesla snap --all' (to capture vehicle_data) then 'tesla timeline' (to stitch charge sessions)"
	}
	return rep, nil
}

type costWhatIf struct {
	Window struct {
		Since string `json:"since"`
		Until string `json:"until"`
	} `json:"window"`
	ActualUSD        float64 `json:"actual_usd"`
	WouldHaveBeenUSD float64 `json:"would_have_been_usd"`
	SavingsUSD       float64 `json:"savings_usd"`
	HomeRatePerKwh   float64 `json:"home_rate_per_kwh"`
	SuperchargerKwh  float64 `json:"supercharger_kwh"`
	SessionsRepriced int     `json:"sessions_repriced"`
	Note             string  `json:"note,omitempty"`
}

func computeCostWhatIf(ctx context.Context, s *store.Store, since time.Time, onlyHome bool) (*costWhatIf, error) {
	out := &costWhatIf{}
	out.Window.Since = since.UTC().Format(time.RFC3339)
	out.Window.Until = time.Now().UTC().Format(time.RFC3339)
	// Look up home tariff (use the lowest-cost configured home window as the substitute price)
	var homeRate sql.NullFloat64
	if err := s.DB().QueryRowContext(ctx, `
        SELECT MIN(cost_per_kwh) FROM tesla_tariffs WHERE location_label = 'home'`).Scan(&homeRate); err != nil && err != sql.ErrNoRows {
		return out, fmt.Errorf("query tariffs: %w", err)
	}
	if !homeRate.Valid || homeRate.Float64 == 0 {
		homeRate.Float64 = 0.13 // US national average residential, surface as note
		out.Note = "no home tariff configured; using $0.13/kWh (US residential average). Set yours with: tesla-pp-cli sql \"INSERT INTO tesla_tariffs(...)\""
	}
	out.HomeRatePerKwh = homeRate.Float64

	rows, err := s.DB().QueryContext(ctx, `
        SELECT energy_added_kwh, cost_usd, fast_charger_type, location_label
        FROM tesla_charges WHERE started_at >= ?`,
		since.UTC().Format(time.RFC3339))
	if err != nil {
		return out, fmt.Errorf("query charges: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			energy, cost  sql.NullFloat64
			fct, locLabel sql.NullString
		)
		if err := rows.Scan(&energy, &cost, &fct, &locLabel); err != nil {
			continue
		}
		isSC := fct.String == "Tesla" || strings.Contains(strings.ToLower(locLabel.String), "supercharger")
		out.ActualUSD += cost.Float64
		if isSC && onlyHome {
			out.WouldHaveBeenUSD += energy.Float64 * homeRate.Float64
			out.SuperchargerKwh += energy.Float64
			out.SessionsRepriced++
		} else {
			out.WouldHaveBeenUSD += cost.Float64
		}
	}
	out.SavingsUSD = out.ActualUSD - out.WouldHaveBeenUSD
	return out, nil
}

// parseSinceDuration is defined in sync.go (generator-emitted). Accepts "7d",
// "24h", "30m", "1w".
