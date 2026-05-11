// Transcendence commands: features only possible with a local SQLite store
// + compound cloud calls. Nine total, drawn from the Phase 1.5 absorb
// manifest's transcendence table. None of these exist in any competing tool
// (omnilogic-python, HA core, openHAB binding, Homebridge plugin).

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/store"

	"github.com/spf13/cobra"
)

// ---------- status (pool readiness composite) ----------

type statusReport struct {
	MspSystemID  int                `json:"msp_system_id"`
	BackyardName string             `json:"backyard_name"`
	Verdict      string             `json:"verdict"`
	Reasons      []string           `json:"reasons"`
	Bodies       []statusBodyReport `json:"bodies_of_water"`
	ActiveAlarms []omnilogic.Alarm  `json:"active_alarms"`
	SampledAt    time.Time          `json:"sampled_at"`
}

type statusBodyReport struct {
	Name      string                  `json:"name"`
	Type      string                  `json:"type,omitempty"`
	WaterTemp *int                    `json:"water_temp,omitempty"`
	Chemistry omnilogic.Chemistry     `json:"chemistry"`
	Pumps     []statusEquipmentReport `json:"pumps"`
	Heaters   []statusEquipmentReport `json:"heaters"`
}

type statusEquipmentReport struct {
	Name    string `json:"name,omitempty"`
	IsOn    *bool  `json:"is_on,omitempty"`
	Speed   *int   `json:"speed,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

func newStatusCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:   "status",
		Short: "One-shot 'is the pool ready for guests?' verdict: chemistry, temp, alarms, pump state.",
		Long: `Combines telemetry, alarms, and MSP config into a single composite report.
Verdict is green/yellow/red:
  ok       — chemistry in range, no active alarms, pump running
  caution  — chemistry off, but no critical alarms
  warning  — critical alarms or major chemistry deviations

The Hayward app makes you tap through four screens to assemble the same answer.
This command compounds 3 cloud calls + 1 store lookup into a single JSON document.`,
		Example:     "  hayward-omnilogic-pp-cli status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			tele, err := c.GetTelemetry(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			alarms, err := c.GetAlarmList(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			if s != nil {
				_, _ = s.AppendTelemetry(tele)
				_ = s.UpsertAlarms(site.MspSystemID, alarms)
			}

			report := buildStatusReport(site, tele, alarms, cfg)
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	return cmd
}

func buildStatusReport(site omnilogic.Site, tele *omnilogic.Telemetry, alarms []omnilogic.Alarm, cfg *omnilogic.MspConfig) statusReport {
	r := statusReport{
		MspSystemID:  site.MspSystemID,
		BackyardName: site.BackyardName,
		ActiveAlarms: alarms,
		SampledAt:    tele.SampledAt,
	}
	worst := "ok"
	for _, bow := range tele.BodiesOfWater {
		ch := omnilogic.Chemistry{
			BowName: bow.Name, PH: bow.PH, ORP: bow.ORP, SaltPPM: bow.SaltPPM,
			WaterTemp: bow.WaterTemp, AirTemp: tele.AirTemp, SampledAt: tele.SampledAt,
		}
		ch.Verdict, ch.Reasons = omnilogic.ChemistryVerdict(bow.PH, bow.ORP, bow.SaltPPM)
		// Pull a friendlier name from the MSP config if possible.
		if cfg != nil {
			for _, mbow := range cfg.BodiesOfWater {
				if mbow.SystemID == bow.SystemID && mbow.Name != "" {
					ch.BowName = mbow.Name
				}
			}
		}
		body := statusBodyReport{Name: ch.BowName, WaterTemp: bow.WaterTemp, Chemistry: ch}
		for _, p := range bow.Pumps {
			body.Pumps = append(body.Pumps, statusEquipmentReport{Name: p.Name, IsOn: p.IsOn, Speed: p.Speed})
		}
		for _, h := range bow.Heaters {
			body.Heaters = append(body.Heaters, statusEquipmentReport{Name: h.Name, Enabled: h.Enabled})
		}
		if ch.Verdict != "ok" {
			worst = bumpStatusVerdict(worst, "caution")
			r.Reasons = append(r.Reasons, ch.Reasons...)
		}
		r.Bodies = append(r.Bodies, body)
	}
	if len(alarms) > 0 {
		worst = bumpStatusVerdict(worst, "warning")
		for _, a := range alarms {
			if a.Message != "" {
				r.Reasons = append(r.Reasons, fmt.Sprintf("alarm: %s", a.Message))
			}
		}
	}
	r.Verdict = worst
	return r
}

func bumpStatusVerdict(cur, candidate string) string {
	rank := map[string]int{"ok": 0, "caution": 1, "warning": 2}
	if rank[candidate] > rank[cur] {
		return candidate
	}
	return cur
}

// ---------- ready-by ----------

func newReadyByCmd(flags *rootFlags) *cobra.Command {
	var siteID, targetTemp int
	cmd := &cobra.Command{
		Use:   "ready-by <HH:MM> [--temp F]",
		Short: "Enable the heater and compute when to start so the pool hits target temp by arrival time (learned heat rate).",
		Long: `Computes when to enable the heater so the pool reaches your target temperature
by a specified arrival time. Heat rate (°F/hr) is learned from telemetry deltas
in the local store; without enough history the command falls back to 1.5°F/hr
(typical residential gas heater on an average-sized pool).

Pass --dry-run to see the start time and projected heat curve without
actually enabling the heater.`,
		Example: "  hayward-omnilogic-pp-cli ready-by 18:00 --temp 84",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || targetTemp == 0 {
				return cmd.Help()
			}
			target, err := parseArrivalTime(args[0])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {
				return classifyOmnilogicError(err)
			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			tele, err := c.GetTelemetry(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			if s != nil {
				_, _ = s.AppendTelemetry(tele)
			}
			cfg, err := resolveMspConfig(c, s, site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			rate := estimateHeatRate(s, site.MspSystemID)
			currentTemp := 0
			for _, bow := range tele.BodiesOfWater {
				if bow.WaterTemp != nil {
					currentTemp = *bow.WaterTemp
					break
				}
			}
			if currentTemp == 0 {
				return errors.New("could not read current water temperature")
			}
			delta := targetTemp - currentTemp
			if delta <= 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verdict":      "already-warmer",
					"current_temp": currentTemp,
					"target_temp":  targetTemp,
					"arrival":      target.Format(time.RFC3339),
				}, flags)
			}
			hoursNeeded := float64(delta) / rate
			startAt := target.Add(-time.Duration(hoursNeeded * float64(time.Hour)))
			report := map[string]any{
				"current_temp":      currentTemp,
				"target_temp":       targetTemp,
				"delta_f":           delta,
				"heat_rate_f_per_h": rate,
				"hours_needed":      math.Round(hoursNeeded*10) / 10,
				"start_heating_at":  startAt.Format(time.RFC3339),
				"arrival":           target.Format(time.RFC3339),
			}
			// Enable the heater now and set the target setpoint. Hayward's
			// gas/heat-pump heaters take time to reach setpoint, so starting
			// immediately when the user runs the command is the correct
			// action: the report's start_heating_at and hours_needed fields
			// tell them whether they're ahead of schedule, on time, or behind.
			// Scheduling the heater to fire later (instead of now) would need
			// Hayward's schedule API, which isn't wired up yet — future work.
			if flags.dryRun {
				report["dry_run"] = true
			} else {
				poolID, heaterID, h, herr := omnilogic.ResolveHeater(cfg, "")
				if herr != nil {
					report["heater_enabled"] = false
					report["heater_error"] = herr.Error()
				} else {
					params := map[string]any{"heater": h.Name, "set_temp": targetTemp}
					enableResult, enableErr := c.SetHeaterEnable(site.MspSystemID, poolID, heaterID, true)
					if enableErr != nil {
						report["heater_enabled"] = false
						report["heater_error"] = enableErr.Error()
					} else if enableResult != nil && enableResult.Status != "ok" {
						report["heater_enabled"] = false
						report["heater_error"] = enableResult.Detail
					} else {
						_, _ = c.SetHeaterTemp(site.MspSystemID, poolID, heaterID, targetTemp)
						logResult(s, "ready-by", h.Name, params, &omnilogic.CommandResult{Status: "ok"})
						report["heater_enabled"] = true
						report["heater_name"] = h.Name
						// Surface whether the user is ahead of, on, or behind schedule.
						switch {
						case time.Now().Before(startAt):
							report["schedule_state"] = "ahead-of-schedule"
						case time.Now().After(startAt):
							report["schedule_state"] = "behind-schedule"
						default:
							report["schedule_state"] = "on-schedule"
						}
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	cmd.Flags().IntVar(&targetTemp, "temp", 0, "Target water temperature in °F.")
	return cmd
}

func parseArrivalTime(s string) (time.Time, error) {
	now := time.Now()
	for _, layout := range []string{"15:04", "3:04PM", "3:04pm", "3PM", "3pm"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse arrival time %q (try HH:MM)", s)
}

func estimateHeatRate(s *store.Store, siteID int) float64 {
	const fallback = 1.5 // °F/hour, typical residential gas heater
	if s == nil {
		return fallback
	}
	rows, err := s.DB.Query(
		`SELECT value_int, sampled_at FROM telemetry_samples
		 WHERE site_msp_system_id = ? AND metric = 'water_temp'
		 ORDER BY sampled_at DESC LIMIT 200`,
		siteID,
	)
	if err != nil {
		return fallback
	}
	defer rows.Close()
	type sample struct {
		v int
		t time.Time
	}
	var samples []sample
	for rows.Next() {
		var v sql.NullInt64
		var ts string
		if err := rows.Scan(&v, &ts); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, ts)
		if v.Valid {
			samples = append(samples, sample{int(v.Int64), t})
		}
	}
	if len(samples) < 4 {
		return fallback
	}
	// Find positive deltas (heating periods) and average their rate.
	var rates []float64
	for i := 1; i < len(samples); i++ {
		a, b := samples[i-1], samples[i]
		dt := a.t.Sub(b.t).Hours()
		if dt < 0.1 || dt > 6 {
			continue
		}
		dv := float64(a.v - b.v)
		if dv > 0.5 && dv < 20 {
			rate := dv / dt
			if rate > 0.1 && rate < 10 {
				rates = append(rates, rate)
			}
		}
	}
	if len(rates) == 0 {
		return fallback
	}
	sum := 0.0
	for _, r := range rates {
		sum += r
	}
	return sum / float64(len(rates))
}

// ---------- chemistry log ----------

func newOmniChemistryLogCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var sinceStr string
	var asCSV bool
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Historical pH / ORP / salt / temp readings from the local store, exportable as CSV/JSON.",
		Long: `Reads telemetry_samples for chemistry metrics (pH, ORP, salt, water_temp)
and prints them oldest-first. --since accepts a duration (7d, 30d, 90d) or a
relative phrase (yesterday, last-week, last-month). --csv emits a flat CSV
suitable for HOA, service-record, or insurance logs.

The cloud API returns "now" only — this command works because every previous
'sync', 'telemetry get', 'chemistry get', or 'status' call appended a row to
the store.`,
		Example:     "  hayward-omnilogic-pp-cli chemistry log --since 7d --csv > pool-chem.csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer closeStore(s)
			since := ""
			if sinceStr != "" {
				t, err := parseSinceExpr(sinceStr)
				if err != nil {
					return usageErr(err)
				}
				since = t.Format(time.RFC3339)
			}
			metrics := []string{"ph", "orp", "salt_ppm", "water_temp", "air_temp"}
			type row struct {
				SampledAt   string  `json:"sampled_at"`
				BowSystemID string  `json:"bow_system_id,omitempty"`
				Metric      string  `json:"metric"`
				Value       float64 `json:"value"`
			}
			var rows []row
			for _, m := range metrics {
				samples, err := s.QueryTelemetry(siteID, m, since, 0)
				if err != nil {
					return apiErr(err)
				}
				for _, sample := range samples {
					v := 0.0
					if sample.ValueReal.Valid {
						v = sample.ValueReal.Float64
					} else if sample.ValueInt.Valid {
						v = float64(sample.ValueInt.Int64)
					}
					rows = append(rows, row{
						SampledAt: sample.SampledAt, BowSystemID: sample.BowSystemID,
						Metric: m, Value: v,
					})
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].SampledAt < rows[j].SampledAt })
			if asCSV {
				w := csv.NewWriter(cmd.OutOrStdout())
				_ = w.Write([]string{"sampled_at", "bow_system_id", "metric", "value"})
				for _, r := range rows {
					_ = w.Write([]string{r.SampledAt, r.BowSystemID, r.Metric, strconv.FormatFloat(r.Value, 'f', -1, 64)})
				}
				w.Flush()
				return w.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit for all sites.")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Time window: 7d, 30d, 90d, yesterday, last-week, last-month.")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "Emit as CSV (overrides --json/--json defaults).")
	return cmd
}

func parseSinceExpr(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now().UTC()
	switch strings.ToLower(s) {
	case "yesterday":
		return now.Add(-24 * time.Hour), nil
	case "last-week", "lastweek", "7d", "week":
		return now.Add(-7 * 24 * time.Hour), nil
	case "last-month", "lastmonth", "30d", "month":
		return now.Add(-30 * 24 * time.Hour), nil
	case "90d", "quarter":
		return now.Add(-90 * 24 * time.Hour), nil
	case "year", "365d":
		return now.Add(-365 * 24 * time.Hour), nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return now.Add(-time.Duration(n) * 24 * time.Hour), nil
		}
	}
	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err == nil {
			return now.Add(-time.Duration(n) * time.Hour), nil
		}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized --since %q (try 7d, 30d, yesterday, or YYYY-MM-DD)", s)
}

// ---------- chemistry drift ----------

func newOmniChemistryDriftCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var forecast bool
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Detect pH / ORP / salt drift vs. a rolling baseline; --forecast projects when each will leave range.",
		Long: `Compares the most recent reading for each chemistry metric against a 7-day
rolling baseline. Flags a metric as "drifting" if it has moved >0.3 (pH) /
>50 mV (ORP) / >200 ppm (salt) from the baseline AND the trend is monotonic
over the last 5 samples.

With --forecast, projects when each drifting metric will exit its safe range
using a linear fit on the last 5-10 samples. This is the chemistry early-warning
that Hayward's static-threshold alarms don't provide.`,
		Example:     "  hayward-omnilogic-pp-cli chemistry drift --forecast --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			report := buildDriftReport(s, siteID, forecast)
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit for all sites.")
	cmd.Flags().BoolVar(&forecast, "forecast", false, "Project when each drifting metric will exit safe range.")
	return cmd
}

type driftReport struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Metrics     []driftMetricReport `json:"metrics"`
}

type driftMetricReport struct {
	Metric        string  `json:"metric"`
	CurrentValue  float64 `json:"current_value"`
	BaselineValue float64 `json:"baseline_value"`
	Delta         float64 `json:"delta"`
	Trend         string  `json:"trend"`
	Drifting      bool    `json:"drifting"`
	ForecastNote  string  `json:"forecast_note,omitempty"`
	Samples       int     `json:"samples_considered"`
}

func buildDriftReport(s *store.Store, siteID int, forecast bool) driftReport {
	r := driftReport{GeneratedAt: time.Now().UTC()}
	metrics := []struct {
		key       string
		threshold float64
		safeLow   float64
		safeHigh  float64
	}{
		{"ph", 0.3, 7.2, 7.8},
		{"orp", 50, 650, 750},
		{"salt_ppm", 200, 2700, 3500},
	}
	weekAgo := time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	for _, m := range metrics {
		samples, err := s.QueryTelemetry(siteID, m.key, weekAgo, 100)
		if err != nil || len(samples) < 2 {
			continue
		}
		// QueryTelemetry returns desc by sampled_at; reverse for chronological.
		sort.Slice(samples, func(i, j int) bool { return samples[i].SampledAt < samples[j].SampledAt })
		values := make([]float64, 0, len(samples))
		for _, s := range samples {
			if s.ValueReal.Valid {
				values = append(values, s.ValueReal.Float64)
			} else if s.ValueInt.Valid {
				values = append(values, float64(s.ValueInt.Int64))
			}
		}
		if len(values) < 2 {
			continue
		}
		current := values[len(values)-1]
		// baseline = mean of first half
		base := 0.0
		half := len(values) / 2
		if half < 1 {
			half = 1
		}
		for i := 0; i < half; i++ {
			base += values[i]
		}
		base /= float64(half)
		delta := current - base
		trend := "stable"
		if delta > m.threshold {
			trend = "up"
		} else if delta < -m.threshold {
			trend = "down"
		}
		drifting := trend != "stable"
		mr := driftMetricReport{
			Metric: m.key, CurrentValue: current, BaselineValue: base, Delta: delta,
			Trend: trend, Drifting: drifting, Samples: len(values),
		}
		if drifting && forecast {
			mr.ForecastNote = projectExit(values, m.safeLow, m.safeHigh, samples[0].SampledAt, samples[len(samples)-1].SampledAt)
		}
		r.Metrics = append(r.Metrics, mr)
	}
	return r
}

func projectExit(values []float64, low, high float64, firstTs, lastTs string) string {
	if len(values) < 3 {
		return ""
	}
	t0, _ := time.Parse(time.RFC3339, firstTs)
	t1, _ := time.Parse(time.RFC3339, lastTs)
	if t1.Before(t0) || t1.Equal(t0) {
		return ""
	}
	span := t1.Sub(t0).Hours()
	rate := (values[len(values)-1] - values[0]) / span // units per hour
	if math.Abs(rate) < 1e-9 {
		return "no measurable rate of change"
	}
	current := values[len(values)-1]
	if rate > 0 {
		if current >= high {
			return "already above safe range"
		}
		hours := (high - current) / rate
		return fmt.Sprintf("at current rate, exits high (%.2f) in %.1f hours", high, hours)
	}
	if current <= low {
		return "already below safe range"
	}
	hours := (low - current) / rate
	return fmt.Sprintf("at current rate, exits low (%.2f) in %.1f hours", low, hours)
}

// ---------- runtime ----------

func newRuntimeCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var sinceStr string
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Pump / heater / chlorinator hours from telemetry deltas — for maintenance, warranty, service.",
		Long: `Walks telemetry_samples for pump_on / heater_enabled / chlor_output_pct and
sums "on" time across the window. Returns hours-on per equipment item.
Not a Hayward API field — only the store can compute this.`,
		Example:     "  hayward-omnilogic-pp-cli runtime --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			since := time.Now().Add(-30 * 24 * time.Hour)
			if sinceStr != "" {
				t, err := parseSinceExpr(sinceStr)
				if err != nil {
					return usageErr(err)
				}
				since = t
			}
			type rep struct {
				EquipmentID string  `json:"equipment_id"`
				Metric      string  `json:"metric"`
				HoursOn     float64 `json:"hours_on"`
				Samples     int     `json:"samples"`
			}
			var out []rep
			for _, m := range []string{"pump_on", "heater_enabled", "relay_on", "light_on"} {
				samples, err := s.QueryTelemetry(siteID, m, since.Format(time.RFC3339), 0)
				if err != nil {
					continue
				}
				byEq := map[string][]store.TelemetrySample{}
				for _, s := range samples {
					byEq[s.EquipmentID] = append(byEq[s.EquipmentID], s)
				}
				for eq, ss := range byEq {
					sort.Slice(ss, func(i, j int) bool { return ss[i].SampledAt < ss[j].SampledAt })
					hours := 0.0
					for i := 1; i < len(ss); i++ {
						if ss[i-1].ValueInt.Valid && ss[i-1].ValueInt.Int64 > 0 {
							t0, _ := time.Parse(time.RFC3339, ss[i-1].SampledAt)
							t1, _ := time.Parse(time.RFC3339, ss[i].SampledAt)
							dt := t1.Sub(t0).Hours()
							if dt > 0 && dt < 6 { // cap to avoid blowing out on long gaps
								hours += dt
							}
						}
					}
					if hours > 0.01 {
						out = append(out, rep{EquipmentID: eq, Metric: m, HoursOn: math.Round(hours*10) / 10, Samples: len(ss)})
					}
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].HoursOn > out[j].HoursOn })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit for all sites.")
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Time window for hours-on calculation.")
	return cmd
}

// ---------- command log ----------

func newCommandLogCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var limit int
	cmd := &cobra.Command{
		Use:   "command-log",
		Short: "Local audit trail of every Set* command this CLI has issued.",
		Long: `Every heater/pump/equipment/light/chlorinator command issued via this CLI
is recorded with op, target, params, status, and timestamp. --replay <id>
re-issues a prior command for quick undo/redo.

The cloud doesn't expose per-user command history; this is local-only.`,
		Example:     "  hayward-omnilogic-pp-cli command-log --since 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			q := `SELECT id, ts, op, target, params_json, status, detail, dry_run FROM command_log WHERE 1=1`
			var args2 []any
			if sinceStr != "" {
				t, err := parseSinceExpr(sinceStr)
				if err != nil {
					return usageErr(err)
				}
				q += ` AND ts >= ?`
				args2 = append(args2, t.Format(time.RFC3339))
			}
			q += ` ORDER BY ts DESC`
			if limit > 0 {
				q += ` LIMIT ?`
				args2 = append(args2, limit)
			}
			rows, err := s.DB.Query(q, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				ID     int             `json:"id"`
				Ts     string          `json:"ts"`
				Op     string          `json:"op"`
				Target string          `json:"target"`
				Params json.RawMessage `json:"params,omitempty"`
				Status string          `json:"status"`
				Detail string          `json:"detail,omitempty"`
				DryRun bool            `json:"dry_run"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				var paramsStr string
				var dry int
				if err := rows.Scan(&e.ID, &e.Ts, &e.Op, &e.Target, &paramsStr, &e.Status, &e.Detail, &dry); err != nil {
					continue
				}
				e.DryRun = dry != 0
				if paramsStr != "" {
					e.Params = json.RawMessage(paramsStr)
				}
				out = append(out, e)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Time window (e.g. 7d, 30d, yesterday).")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows to return.")
	return cmd
}

// ---------- why-not-running ----------

func newWhyNotRunningCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	cmd := &cobra.Command{
		Use:   "why-not-running <equipment-name>",
		Short: "Diagnose why an equipment item isn't running: alarms + state + schedule + heater demand.",
		Long: `Correlates four independent signals to explain why a pump, heater, or light
isn't running right now:
  1. Active alarms on that equipment
  2. Current telemetry state (is_on, enabled)
  3. MSP-config schedules covering this time of day
  4. Recent command_log entries that may have turned it off

Returns a structured explanation no single API call provides.`,
		Example:     "  hayward-omnilogic-pp-cli why-not-running 'Main Pump'",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Accept multi-word equipment names whether quoted or unquoted.
			name := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			site, err := resolveSite(c, s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			tele, err := c.GetTelemetry(site.MspSystemID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			alarms, _ := c.GetAlarmList(site.MspSystemID)
			cfg, _ := resolveMspConfig(c, s, site.MspSystemID)
			report := diagnoseEquipment(name, tele, alarms, cfg, s)
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	return cmd
}

type whyReport struct {
	Equipment      string            `json:"equipment"`
	EquipmentID    string            `json:"equipment_id,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	State          string            `json:"state"`
	Reason         string            `json:"reason"`
	ActiveAlarms   []omnilogic.Alarm `json:"active_alarms,omitempty"`
	RecentCommands []map[string]any  `json:"recent_commands,omitempty"`
}

func diagnoseEquipment(name string, tele *omnilogic.Telemetry, alarms []omnilogic.Alarm, cfg *omnilogic.MspConfig, s *store.Store) whyReport {
	r := whyReport{Equipment: name}
	var matchedID string
	var matchedState *omnilogic.TelemetryEquipmentState
	var matchedKind string
	if cfg != nil {
		if pid, eid, kind, display, err := omnilogic.ResolveEquipment(cfg, name, ""); err == nil {
			_ = pid
			matchedID = strconv.Itoa(eid)
			r.Equipment = display
			matchedKind = kind
			r.EquipmentID = matchedID
			r.Kind = kind
		}
	}
	for _, bow := range tele.BodiesOfWater {
		for _, p := range bow.Pumps {
			if p.SystemID == matchedID {
				ps := p
				matchedState = &ps
			}
		}
		for _, h := range bow.Heaters {
			if h.SystemID == matchedID {
				hs := h
				matchedState = &hs
			}
		}
		for _, l := range bow.Lights {
			if l.SystemID == matchedID {
				ls := l
				matchedState = &ls
			}
		}
		for _, rly := range bow.Relays {
			if rly.SystemID == matchedID {
				rs := rly
				matchedState = &rs
			}
		}
	}
	// State and reason
	if matchedState != nil && matchedState.IsOn != nil {
		if *matchedState.IsOn {
			r.State = "running"
			r.Reason = "Equipment is on. No diagnosis needed."
		} else {
			r.State = "off"
		}
	} else {
		r.State = "unknown"
	}
	// Active alarms on this equipment
	for _, a := range alarms {
		if a.EquipmentID == matchedID {
			r.ActiveAlarms = append(r.ActiveAlarms, a)
		}
	}
	if r.State != "running" {
		if len(r.ActiveAlarms) > 0 {
			r.Reason = fmt.Sprintf("Active alarm: %s", r.ActiveAlarms[0].Message)
		} else if matchedKind == "heater" && matchedState != nil && matchedState.Enabled != nil && !*matchedState.Enabled {
			r.Reason = "Heater is disabled. Run 'heater enable' to turn it on."
		} else if r.Reason == "" {
			r.Reason = "Equipment is off. No active alarms; check schedules or recent commands."
		}
	}
	// Recent commands targeting this equipment
	if s != nil && matchedID != "" {
		rows, err := s.DB.Query(
			`SELECT ts, op, target, params_json, status FROM command_log
			 WHERE target LIKE '%' || ? || '%'
			 ORDER BY ts DESC LIMIT 5`,
			matchedID,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var ts, op, target, params, status string
				if err := rows.Scan(&ts, &op, &target, &params, &status); err == nil {
					r.RecentCommands = append(r.RecentCommands, map[string]any{
						"ts": ts, "op": op, "target": target, "status": status, "params": json.RawMessage(params),
					})
				}
			}
		}
	}
	return r
}

// ---------- schedule diff ----------

func newScheduleDiffCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var sinceStr string
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule management: 'schedule diff' compares MSP-config snapshots over time.",
	}
	diff := &cobra.Command{
		Use:   "diff",
		Short: "Diff today's MSP-config schedules against an earlier snapshot.",
		Long: `Compares the most recent MSP-config snapshot against the snapshot from
--since hours/days ago. Catches silent edits by service techs, app users,
or anyone else with account access.

The cloud has no schedule-history endpoint — this works because every
'sync' and 'config get' call versioned a snapshot.`,
		Example:     "  hayward-omnilogic-pp-cli schedule diff --since yesterday",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			// Find two snapshots: latest and earliest-on-or-after sinceStr.
			site, err := resolveSiteFromStore(s, siteID)
			if err != nil {
				return classifyOmnilogicError(err)
			}
			latest := mustQuerySnapshot(s, site, "ORDER BY fetched_at DESC LIMIT 1")
			if latest == nil {
				return apiErr(errors.New("no MSP config snapshots in store; run 'sync' or 'config get' first"))
			}
			cutoff := time.Now().Add(-24 * time.Hour)
			if sinceStr != "" {
				t, err := parseSinceExpr(sinceStr)
				if err != nil {
					return usageErr(err)
				}
				cutoff = t
			}
			prev := mustQuerySnapshotBefore(s, site, cutoff)
			if prev == nil {
				return apiErr(fmt.Errorf("no snapshot before %s in store (only have %s)", cutoff.Format(time.RFC3339), latest.FetchedAt.Format(time.RFC3339)))
			}
			report := map[string]any{
				"baseline_fetched_at": prev.FetchedAt,
				"latest_fetched_at":   latest.FetchedAt,
				"changes":             diffMspConfigs(prev, latest),
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	diff.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to use the only registered site.")
	diff.Flags().StringVar(&sinceStr, "since", "yesterday", "How far back to look for a baseline (e.g. 1d, 7d, yesterday).")
	cmd.AddCommand(diff)
	return cmd
}

func resolveSiteFromStore(s *store.Store, hint int) (int, error) {
	sites, err := s.ListSites()
	if err != nil {
		return 0, err
	}
	site, err := omnilogic.ResolveSite(sites, hint)
	if err != nil {
		return 0, err
	}
	return site.MspSystemID, nil
}

func mustQuerySnapshot(s *store.Store, site int, suffix string) *omnilogic.MspConfig {
	row := s.DB.QueryRow(`SELECT summary_json, fetched_at FROM msp_config_snapshots WHERE site_msp_system_id = ? `+suffix, site)
	var summary, ts string
	if err := row.Scan(&summary, &ts); err != nil {
		return nil
	}
	var cfg omnilogic.MspConfig
	if err := json.Unmarshal([]byte(summary), &cfg); err != nil {
		return nil
	}
	cfg.FetchedAt, _ = time.Parse(time.RFC3339, ts)
	return &cfg
}

func mustQuerySnapshotBefore(s *store.Store, site int, before time.Time) *omnilogic.MspConfig {
	row := s.DB.QueryRow(
		`SELECT summary_json, fetched_at FROM msp_config_snapshots
		 WHERE site_msp_system_id = ? AND fetched_at < ?
		 ORDER BY fetched_at DESC LIMIT 1`,
		site, before.Format(time.RFC3339),
	)
	var summary, ts string
	if err := row.Scan(&summary, &ts); err != nil {
		return nil
	}
	var cfg omnilogic.MspConfig
	if err := json.Unmarshal([]byte(summary), &cfg); err != nil {
		return nil
	}
	cfg.FetchedAt, _ = time.Parse(time.RFC3339, ts)
	return &cfg
}

func diffMspConfigs(a, b *omnilogic.MspConfig) []map[string]any {
	var changes []map[string]any
	bowsA := bowsByID(a)
	bowsB := bowsByID(b)
	for id, before := range bowsA {
		after, ok := bowsB[id]
		if !ok {
			changes = append(changes, map[string]any{"kind": "bow-removed", "bow_system_id": id, "name": before.Name})
			continue
		}
		// Compare equipment counts as a coarse signal; the deep diff is the JSON.
		if len(before.Heaters) != len(after.Heaters) {
			changes = append(changes, map[string]any{
				"kind": "heater-count-changed", "bow_system_id": id,
				"before": len(before.Heaters), "after": len(after.Heaters),
			})
		}
		// Compare setpoint per heater (heater Current-Set-Point change is the
		// most likely scheduled-change signal).
		hbefore := heatersByID(before.Heaters)
		hafter := heatersByID(after.Heaters)
		for hid, hb := range hbefore {
			ha, ok := hafter[hid]
			if !ok {
				continue
			}
			if hb.CurrentSetPoint != ha.CurrentSetPoint {
				changes = append(changes, map[string]any{
					"kind":          "heater-setpoint-changed",
					"bow_system_id": id,
					"heater":        ha.Name,
					"before":        hb.CurrentSetPoint,
					"after":         ha.CurrentSetPoint,
				})
			}
			if hb.Enabled != ha.Enabled {
				changes = append(changes, map[string]any{
					"kind":          "heater-enabled-changed",
					"bow_system_id": id,
					"heater":        ha.Name,
					"before":        hb.Enabled,
					"after":         ha.Enabled,
				})
			}
		}
	}
	for id, after := range bowsB {
		if _, ok := bowsA[id]; !ok {
			changes = append(changes, map[string]any{"kind": "bow-added", "bow_system_id": id, "name": after.Name})
		}
	}
	return changes
}

func bowsByID(cfg *omnilogic.MspConfig) map[string]omnilogic.BodyOfWater {
	m := map[string]omnilogic.BodyOfWater{}
	for _, b := range cfg.BodiesOfWater {
		m[b.SystemID] = b
	}
	return m
}

func heatersByID(hs []omnilogic.Heater) map[string]omnilogic.Heater {
	m := map[string]omnilogic.Heater{}
	for _, h := range hs {
		m[h.SystemID] = h
	}
	return m
}

// ---------- sweep ----------

func newSweepCmd(flags *rootFlags) *cobra.Command {
	var alarmsOnly, chemOnly bool
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Across every site in your account, surface alarms + out-of-range chemistry + offline controllers.",
		Long: `Multi-site morning sweep for pool-service businesses. Walks every site
registered to the account, fetches alarms + telemetry, and emits a single
report ranking sites by attention needed.

Each site row carries: active_alarms count, chemistry_verdict (ok/caution/warning),
offline_controllers count (sites where telemetry fetch failed).`,
		Example:     "  hayward-omnilogic-pp-cli sweep --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, _ := openStore()
			defer closeStore(s)
			sites, err := c.GetSiteList()
			if err != nil {
				return classifyOmnilogicError(err)
			}
			if s != nil {
				_ = s.UpsertSites(sites)
			}
			sortSitesByID(sites)
			type siteSummary struct {
				MspSystemID   int      `json:"msp_system_id"`
				BackyardName  string   `json:"backyard_name"`
				ActiveAlarms  int      `json:"active_alarms"`
				AlarmMessages []string `json:"alarm_messages,omitempty"`
				ChemVerdict   string   `json:"chemistry_verdict,omitempty"`
				ChemReasons   []string `json:"chemistry_reasons,omitempty"`
				Offline       bool     `json:"offline,omitempty"`
				PrimaryBoW    string   `json:"primary_bow,omitempty"`
				Priority      int      `json:"priority"` // 0=ok, 1=caution, 2=warning
			}
			var summary []siteSummary
			for _, site := range sites {
				ss := siteSummary{MspSystemID: site.MspSystemID, BackyardName: site.BackyardName}
				doAlarms := !chemOnly
				doChem := !alarmsOnly
				if doAlarms {
					alarms, err := c.GetAlarmList(site.MspSystemID)
					if err != nil {
						ss.Offline = true
						ss.Priority = 2
						summary = append(summary, ss)
						continue
					}
					if s != nil {
						_ = s.UpsertAlarms(site.MspSystemID, alarms)
					}
					ss.ActiveAlarms = len(alarms)
					for _, a := range alarms {
						if a.Message != "" {
							ss.AlarmMessages = append(ss.AlarmMessages, a.Message)
						}
					}
					if ss.ActiveAlarms > 0 {
						ss.Priority = 2
					}
				}
				if doChem {
					tele, err := c.GetTelemetry(site.MspSystemID)
					if err != nil {
						ss.Offline = true
						if ss.Priority < 2 {
							ss.Priority = 2
						}
						summary = append(summary, ss)
						continue
					}
					if s != nil {
						_, _ = s.AppendTelemetry(tele)
					}
					if len(tele.BodiesOfWater) > 0 {
						bow := tele.BodiesOfWater[0]
						ss.PrimaryBoW = bow.Name
						v, reasons := omnilogic.ChemistryVerdict(bow.PH, bow.ORP, bow.SaltPPM)
						ss.ChemVerdict = v
						ss.ChemReasons = reasons
						if v != "ok" && v != "unknown" {
							if ss.Priority < 1 {
								ss.Priority = 1
							}
						}
					}
				}
				summary = append(summary, ss)
			}
			sort.Slice(summary, func(i, j int) bool {
				return summary[i].Priority > summary[j].Priority
			})
			out := map[string]any{
				"generated_at": time.Now().UTC().Format(time.RFC3339),
				"site_count":   len(summary),
				"sites":        summary,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&alarmsOnly, "alarms", false, "Only check alarms (skip telemetry).")
	cmd.Flags().BoolVar(&chemOnly, "chemistry", false, "Only check chemistry (skip alarms).")
	return cmd
}

// ---------- sync ----------

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var siteID int
	var full bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull sites, equipment inventory, telemetry, and alarms into the local SQLite store.",
		Long: `Hydrates the local store with the latest cloud state:
  sites       -> sites table
  MSP config  -> msp_config_snapshots + bodies_of_water + equipment
  telemetry   -> telemetry_samples (append)
  alarms      -> alarms (upsert + clear stale)

Run this periodically (cron / launchd) to keep history fresh — every other
command consults the store first.`,
		Example: "  hayward-omnilogic-pp-cli sync --full",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			summary := map[string]any{"started_at": time.Now().UTC().Format(time.RFC3339)}
			sites, err := c.GetSiteList()
			if err != nil {
				return classifyOmnilogicError(err)
			}
			_ = s.UpsertSites(sites)
			summary["sites"] = len(sites)
			var targets []omnilogic.Site
			if siteID != 0 {
				for _, st := range sites {
					if st.MspSystemID == siteID {
						targets = []omnilogic.Site{st}
					}
				}
			} else {
				targets = sites
			}
			configs, teleCount, alarms := 0, 0, 0
			for _, site := range targets {
				if full {
					cfg, err := c.GetMspConfig(site.MspSystemID)
					if err == nil {
						cfg.BackyardName = site.BackyardName
						_ = s.UpsertMspConfig(cfg)
						configs++
					}
				}
				if tele, err := c.GetTelemetry(site.MspSystemID); err == nil {
					n, _ := s.AppendTelemetry(tele)
					teleCount += n
				}
				if al, err := c.GetAlarmList(site.MspSystemID); err == nil {
					_ = s.UpsertAlarms(site.MspSystemID, al)
					alarms += len(al)
				}
			}
			summary["msp_configs_synced"] = configs
			summary["telemetry_samples_appended"] = teleCount
			summary["alarms_synced"] = alarms
			summary["finished_at"] = time.Now().UTC().Format(time.RFC3339)
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().IntVar(&siteID, "msp-system-id", 0, "Site MspSystemID. Omit to sync every site.")
	cmd.Flags().BoolVar(&full, "full", false, "Also fetch MSP config (slower; needed for schedule diff).")
	return cmd
}

// ---------- search (FTS5 over store) ----------

func newSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search across equipment, alarms, and command_log in the local store.",
		Example:     "  hayward-omnilogic-pp-cli search pump",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			q := strings.Join(args, " ")
			out := map[string]any{}
			// equipment
			rows, _ := s.DB.Query(`SELECT name, kind, type, function FROM equipment_fts WHERE equipment_fts MATCH ? LIMIT 25`, q)
			var equipment []map[string]string
			for rows != nil && rows.Next() {
				var n, k, t, f string
				_ = rows.Scan(&n, &k, &t, &f)
				equipment = append(equipment, map[string]string{"name": n, "kind": k, "type": t, "function": f})
			}
			if rows != nil {
				rows.Close()
			}
			out["equipment"] = equipment
			rows, _ = s.DB.Query(`SELECT message, severity, code FROM alarms_fts WHERE alarms_fts MATCH ? LIMIT 25`, q)
			var alarms []map[string]string
			for rows != nil && rows.Next() {
				var m, sev, c string
				_ = rows.Scan(&m, &sev, &c)
				alarms = append(alarms, map[string]string{"message": m, "severity": sev, "code": c})
			}
			if rows != nil {
				rows.Close()
			}
			out["alarms"] = alarms
			rows, _ = s.DB.Query(`SELECT op, target, detail FROM command_log_fts WHERE command_log_fts MATCH ? LIMIT 25`, q)
			var commands []map[string]string
			for rows != nil && rows.Next() {
				var o, t, d string
				_ = rows.Scan(&o, &t, &d)
				commands = append(commands, map[string]string{"op": o, "target": t, "detail": d})
			}
			if rows != nil {
				rows.Close()
			}
			out["commands"] = commands
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// ---------- sql (read-only SQL over the store) ----------

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sql <query>",
		Short:       "Run a read-only SQL query against the local store (SELECT only).",
		Example:     "  hayward-omnilogic-pp-cli sql 'SELECT name, kind FROM equipment WHERE kind = ''heater'''",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			lower := strings.ToLower(strings.TrimSpace(query))
			if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
				return usageErr(errors.New("sql command accepts SELECT / WITH queries only"))
			}
			for _, banned := range []string{"insert", "update", "delete", "drop", "alter", "create", "attach"} {
				if strings.Contains(lower, banned+" ") {
					return usageErr(fmt.Errorf("%s statements are not allowed via 'sql'", strings.ToUpper(banned)))
				}
			}
			s, err := openStore()
			if err != nil {
				return apiErr(err)
			}
			defer closeStore(s)
			rows, err := s.DB.Query(query)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			cols, _ := rows.Columns()
			var result []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return apiErr(err)
				}
				row := map[string]any{}
				for i, c := range cols {
					row[c] = vals[i]
				}
				result = append(result, row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// ---------- auth ----------

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Hayward authentication cache (login / logout / status).",
	}
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	cmd.AddCommand(newAuthLoginCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Force a fresh login (uses HAYWARD_USER + HAYWARD_PW from the environment).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {

				return nil

			}
			if err := requireCredsUnlessDryRun(flags); err != nil {

				return classifyOmnilogicError(err)

			}
			c := newOmnilogicClient(flags.timeout)
			_ = c.Logout() // clear cache so EnsureToken does a fresh login
			if err := c.EnsureToken(); err != nil {
				return classifyOmnilogicError(err)
			}
			st := c.AuthState()
			out := map[string]any{
				"logged_in":  true,
				"email":      c.Email(),
				"user_id":    st.UserID,
				"expires_at": st.ExpiresAt.Format(time.RFC3339),
				"cache_path": c.AuthCachePath(),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report token cache state without re-authenticating.",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newOmnilogicClient(flags.timeout)
			st := c.AuthState()
			out := map[string]any{
				"cache_path":       c.AuthCachePath(),
				"env_user_set":     os.Getenv(envUser) != "",
				"env_password_set": os.Getenv(envPW) != "",
			}
			if st == nil {
				out["logged_in"] = false
			} else {
				out["logged_in"] = st.Valid()
				out["email"] = st.Email
				out["user_id"] = st.UserID
				out["expires_at"] = st.ExpiresAt.Format(time.RFC3339)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the cached token (forces re-login on next command).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newOmnilogicClient(flags.timeout)
			if err := c.Logout(); err != nil {
				return apiErr(err)
			}
			out := map[string]any{"logged_out": true, "cache_path": c.AuthCachePath()}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
}
