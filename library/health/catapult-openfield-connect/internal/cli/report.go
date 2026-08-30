// Copyright 2026 erash11 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type reportRow struct {
	AthleteName string  `json:"athlete_name"`
	TotalLoad   float64 `json:"total_load"`
	Sessions    int     `json:"sessions"`
	ACWR        float64 `json:"acwr"`
	RiskZone    string  `json:"risk_zone"`
	MaxVel      float64 `json:"max_vel"`
	MaxVelPBPct float64 `json:"max_vel_pb_pct"`
	Monotony    float64 `json:"monotony"`
	Strain      float64 `json:"strain"`
}

func newNovelReportCmd(flags *rootFlags) *cobra.Command {
	var flagWeek bool
	var flagRange string
	var flagSquad bool
	var flagFormat string
	var flagExport string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Weekly load summary: total load, sessions, ACWR, max vel vs PB, monotony, strain, risk flags",
		Long: strings.TrimSpace(`
Generates the coach-briefing load summary for a window (--week = last 7 days,
or --range <from>..<to>): total Player Load, session count, ACWR, max velocity
vs personal best, Foster's monotony and strain, and risk flags. Exports as
markdown or CSV.

Use this command for the multi-metric weekly export.
Do NOT use it for a live morning risk check; use 'acwr'.`),
		Example:     "  catapult-openfield-connect-pp-cli report --week --squad --format markdown --export ./week_load.md",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "report")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if flagFormat != "markdown" && flagFormat != "csv" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be markdown or csv"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			to := now
			from := now.AddDate(0, 0, -6)
			if flagRange != "" {
				parts := strings.SplitN(flagRange, "..", 2)
				if len(parts) != 2 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--range must be <from>..<to>, e.g. 2026-03-09..2026-03-15"))
				}
				f, errF := time.Parse("2006-01-02", parts[0])
				t, errT := time.Parse("2006-01-02", parts[1])
				if errF != nil || errT != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--range dates must be YYYY-MM-DD"))
				}
				if f.After(t) {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--range start %s is after end %s", parts[0], parts[1]))
				}
				from, to = f, t
			}
			// One 28-day fetch ending at `to` covers the report window,
			// the ACWR chronic window, and daily-load monotony.
			chronicStart := to.AddDate(0, 0, -(acwrChronicDays - 1))
			// The window and PB queries are independent; run them in
			// parallel so the command fits interactive/probe time budgets.
			var rows []statsRow
			var pbByAthlete map[string]float64
			var rowsErr, pbErr error
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				rows, rowsErr = fetchStatsWindow(ctx, c, []string{"total_player_load", "max_vel"}, chronicStart, to)
			}()
			go func() {
				defer wg.Done()
				pbByAthlete, pbErr = fetchPB(ctx, c, "max_vel", defaultPBWindowActivities)
			}()
			wg.Wait()
			if rowsErr != nil {
				return classifyAPIError(rowsErr, flags)
			}
			if pbErr != nil {
				return classifyAPIError(pbErr, flags)
			}
			type acc struct {
				name       string
				loadByDay  map[string]float64
				weekLoad   float64
				weekSess   int
				weekMaxVel float64
			}
			perAthlete := map[string]*acc{}
			fromKey, toKey := dayKey(from), dayKey(to)

			for _, r := range rows {
				aid := rowString(r, "athlete_id")
				if aid == "" {
					continue
				}
				day := rowDate(r)
				if day == "" {
					continue
				}
				a := perAthlete[aid]
				if a == nil {
					a = &acc{name: rowString(r, "athlete_name"), loadByDay: map[string]float64{}}
					perAthlete[aid] = a
				}
				load, hasLoad := rowNum(r, "total_player_load")
				if hasLoad {
					a.loadByDay[day] += load
				}
				if day >= fromKey && day <= toKey {
					if hasLoad {
						a.weekLoad += load
					}
					a.weekSess++
					if v, ok := rowNum(r, "max_vel"); ok && v > a.weekMaxVel {
						a.weekMaxVel = v
					}
				}
			}
			results := make([]reportRow, 0, len(perAthlete))
			for aid, a := range perAthlete {
				if a.weekSess == 0 {
					continue
				}
				// ACWR from the 28d daily buckets
				_, _, ratio := computeACWR(a.loadByDay, to)
				// Foster monotony over the report window's daily loads
				var daily []float64
				for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
					daily = append(daily, a.loadByDay[dayKey(d)])
				}
				mean := 0.0
				for _, v := range daily {
					mean += v
				}
				mean /= float64(len(daily))
				variance := 0.0
				for _, v := range daily {
					variance += (v - mean) * (v - mean)
				}
				sd := math.Sqrt(variance / float64(len(daily)))
				monotony := 0.0
				if sd > 0 {
					monotony = mean / sd
				}
				pbPct := 0.0
				if pb := pbByAthlete[aid]; pb > 0 {
					pbPct = a.weekMaxVel / pb * 100
				}
				results = append(results, reportRow{
					AthleteName: a.name,
					TotalLoad:   round2(a.weekLoad),
					Sessions:    a.weekSess,
					ACWR:        round2(ratio),
					RiskZone:    riskZone(ratio),
					MaxVel:      round2(a.weekMaxVel),
					MaxVelPBPct: round2(pbPct),
					Monotony:    round2(monotony),
					Strain:      round2(a.weekLoad * monotony),
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].TotalLoad > results[j].TotalLoad })
			if flagExport == "" && !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			var b strings.Builder
			title := fmt.Sprintf("Load report %s to %s", dayKey(from), dayKey(to))
			if flagFormat == "markdown" {
				b.WriteString("# " + title + "\n\n")
				b.WriteString("| Athlete | Total Load | Sessions | ACWR | Zone | Max Vel | % of PB | Monotony | Strain |\n")
				b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
				for _, r := range results {
					b.WriteString(fmt.Sprintf("| %s | %.0f | %d | %.2f | %s | %.2f | %.0f%% | %.2f | %.0f |\n",
						markdownCell(r.AthleteName), r.TotalLoad, r.Sessions, r.ACWR, r.RiskZone, r.MaxVel, r.MaxVelPBPct, r.Monotony, r.Strain))
				}
			} else {
				cw := csv.NewWriter(&b)
				_ = cw.Write([]string{"athlete_name", "total_load", "sessions", "acwr", "risk_zone", "max_vel", "max_vel_pb_pct", "monotony", "strain"})
				for _, r := range results {
					_ = cw.Write([]string{
						csvSafeCell(r.AthleteName),
						fmt.Sprintf("%.1f", r.TotalLoad), fmt.Sprintf("%d", r.Sessions),
						fmt.Sprintf("%.2f", r.ACWR), r.RiskZone,
						fmt.Sprintf("%.2f", r.MaxVel), fmt.Sprintf("%.1f", r.MaxVelPBPct),
						fmt.Sprintf("%.2f", r.Monotony), fmt.Sprintf("%.1f", r.Strain),
					})
				}
				cw.Flush()
				if err := cw.Error(); err != nil {
					return fmt.Errorf("encoding csv: %w", err)
				}
			}
			if flagExport != "" {
				tmp := flagExport + ".tmp"
				if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
					return fmt.Errorf("writing export: %w", err)
				}
				if err := os.Rename(tmp, flagExport); err != nil {
					return fmt.Errorf("renaming export into place: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d athletes)\n", flagExport, len(results))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), results, flags)
				}
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagWeek, "week", false, "Report on the last 7 days (default window)")
	cmd.Flags().StringVar(&flagRange, "range", "", "Explicit window <from>..<to> (YYYY-MM-DD..YYYY-MM-DD)")
	cmd.Flags().BoolVar(&flagSquad, "squad", true, "Include the full squad (default)")
	cmd.Flags().StringVar(&flagFormat, "format", "markdown", "Output format: markdown or csv")
	cmd.Flags().StringVar(&flagExport, "export", "", "Write the report to this file path")
	return cmd
}
