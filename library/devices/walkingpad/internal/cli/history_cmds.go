package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/history"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/profile"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
)

func newRecordCmd(flags *rootFlags) *cobra.Command {
	var durationStr string
	cmd := &cobra.Command{
		Use: "record",
		// Hidden from MCP: holds a BLE connection for the whole walk (long-running).
		Annotations: map[string]string{"mcp:hidden": "true"},
		Short:       "Record a live walk into the local history store",
		Long:        "Connect to the belt, stream telemetry, and persist the walk as a session in the local history store. Read-only against the belt (no actuation). Requires --live; use --duration to bound the run.",
		Example:     "  walkingpad-pp-cli record --live --duration 30m",
		RunE: func(cmd *cobra.Command, args []string) error {
			var duration time.Duration
			if durationStr != "" {
				d, err := time.ParseDuration(durationStr)
				if err != nil {
					return fmt.Errorf("--duration: %w", err)
				}
				duration = d
			}
			if stop, err := offlineGuard(cmd, flags, "record a walk"); stop {
				return err
			}
			ctx := cmd.Context()
			if d, bounded := boundedContext(cmd, duration); bounded {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
			}
			return captureAndSave(cmd, flags, func(onStatus func(wpble.Status) error) error {
				return dialAndMonitor(ctx, flags, time.Second, onStatus)
			})
		},
	}
	cmd.Flags().StringVar(&durationStr, "duration", "", "How long to record (e.g. 30m); empty = until interrupted")
	return cmd
}

// buildSession derives a session summary from the first/last telemetry of a
// recording. Belt counters are cumulative since the belt's own session start,
// so the walk's totals are the delta; a negative delta (belt counter reset
// mid-recording) falls back to the last absolute value.
func buildSession(first, last wpble.Status, firstTS, lastTS, maxSpeed float64) history.Session {
	dist := last.DistanceM - first.DistanceM
	if dist < 0 {
		dist = last.DistanceM
	}
	steps := last.Steps - first.Steps
	if steps < 0 {
		steps = last.Steps
	}
	durS := int(lastTS - firstTS)
	s := history.Session{
		ID:          time.Unix(int64(firstTS), 0).Format("20060102-150405"),
		StartTS:     firstTS,
		EndTS:       lastTS,
		DurationS:   durS,
		DistanceM:   dist,
		Steps:       steps,
		MaxSpeedKmh: maxSpeed,
	}
	if durS > 0 {
		s.AvgSpeedKmh = (float64(dist) / 1000) / (float64(durS) / 3600)
	}
	return s
}

// captureAndSave runs a status-producing operation (record's passive monitor or
// run's active control), accumulates telemetry into a session, and persists it.
func captureAndSave(cmd *cobra.Command, flags *rootFlags, run func(onStatus func(wpble.Status) error) error) error {
	store, err := openHistory()
	if err != nil {
		return err
	}
	var (
		mu              sync.Mutex
		samples         []history.Sample
		maxSpeed        float64
		firstS, lastS   wpble.Status
		firstTS, lastTS float64
		haveFirst       bool
	)
	onStatus := func(s wpble.Status) error {
		now := float64(time.Now().Unix())
		mu.Lock()
		if !haveFirst {
			firstS, firstTS, haveFirst = s, now, true
		}
		lastS, lastTS = s, now
		if s.SpeedKmh > maxSpeed {
			maxSpeed = s.SpeedKmh
		}
		samples = append(samples, history.Sample{
			TS: now, SpeedKmh: s.SpeedKmh, DistanceM: s.DistanceM, Steps: s.Steps, BeltState: s.BeltState,
		})
		mu.Unlock()
		if !flags.asJSON {
			fmt.Fprintf(cmd.OutOrStdout(), "%.1f km/h  %dm  %d steps\n", s.SpeedKmh, s.DistanceM, s.Steps)
		}
		return nil
	}
	if err := run(onStatus); err != nil {
		return err
	}
	mu.Lock()
	haveFirstSnapshot := haveFirst
	firstSSnapshot := firstS
	lastSSnapshot := lastS
	firstTSSnapshot := firstTS
	lastTSSnapshot := lastTS
	maxSpeedSnapshot := maxSpeed
	samplesSnapshot := append([]history.Sample(nil), samples...)
	mu.Unlock()
	if !haveFirstSnapshot {
		return emit(cmd, flags, map[string]any{"recorded": false}, "no telemetry captured; was the belt moving?")
	}
	session := buildSession(firstSSnapshot, lastSSnapshot, firstTSSnapshot, lastTSSnapshot, maxSpeedSnapshot)
	for i := range samplesSnapshot {
		samplesSnapshot[i].SessionID = session.ID
	}
	if err := store.AddSession(session); err != nil {
		return err
	}
	if err := store.AddSamples(samplesSnapshot); err != nil {
		return err
	}
	return emit(cmd, flags, session,
		fmt.Sprintf("recorded session %s: %dm, %d steps, %ds (%d samples)",
			session.ID, session.DistanceM, session.Steps, session.DurationS, len(samplesSnapshot)))
}

func newTodayCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "today",
		Short:       "Show today's recorded walking totals",
		Long:        "Aggregate today's recorded sessions from the local history store: distance, steps, active minutes, session count.",
		Example:     "  walkingpad-pp-cli today --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openHistory()
			if err != nil {
				return err
			}
			now := time.Now()
			totals, err := store.TotalsOn(now.Format("2006-01-02"), now.Location())
			if err != nil {
				return err
			}
			return emit(cmd, flags, totals,
				fmt.Sprintf("%s: %.2f km, %d steps, %d min, %d session(s)",
					totals.Date, float64(totals.DistanceM)/1000, totals.Steps, totals.DurationS/60, totals.Sessions))
		},
	}
}

func newSessionsCmd(flags *rootFlags) *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:         "sessions",
		Short:       "List recorded sessions for a date (default today)",
		Example:     "  walkingpad-pp-cli sessions --date 2026-06-03 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openHistory()
			if err != nil {
				return err
			}
			now := time.Now()
			if date == "" {
				date = now.Format("2006-01-02")
			}
			sessions, err := store.SessionsOn(date, now.Location())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return writeJSON(cmd, map[string]any{"date": date, "sessions": sessions})
			}
			if len(sessions) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no sessions on %s\n", date)
				return nil
			}
			for _, s := range sessions {
				start := time.Unix(int64(s.StartTS), 0).Format("15:04")
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %.2f km  %d steps  %d min  max %.1f km/h\n",
					s.ID, start, float64(s.DistanceM)/1000, s.Steps, s.DurationS/60, s.MaxSpeedKmh)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Date to list (YYYY-MM-DD), default today")
	return cmd
}

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "trends",
		Short:       "Show per-day walking totals over a window",
		Long:        "Show distance, steps, and active minutes per day for the last N days, including zero-activity days.",
		Example:     "  walkingpad-pp-cli trends --days 14 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openHistory()
			if err != nil {
				return err
			}
			series, err := store.DailySeries(days, time.Now())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return writeJSON(cmd, map[string]any{"days": series})
			}
			for _, d := range series {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %.2f km  %d steps  %d min  %d session(s)\n",
					d.Date, float64(d.DistanceM)/1000, d.Steps, d.DurationS/60, d.Sessions)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Number of days to include")
	return cmd
}

func newStreakCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "streak",
		Short:       "Show the current consecutive-day walking streak",
		Example:     "  walkingpad-pp-cli streak --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openHistory()
			if err != nil {
				return err
			}
			streak, err := store.Streak(time.Now())
			if err != nil {
				return err
			}
			return emit(cmd, flags, map[string]any{"streak_days": streak},
				fmt.Sprintf("current streak: %d day(s)", streak))
		},
	}
}

func newCaloriesCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "calories",
		Short:       "Estimate calories burned (the belt never reports them)",
		Long:        "Estimate calories burned over the last N days from your recorded sessions and body weight, using the MET method. Set your weight first with `profile set --weight <kg>`.",
		Example:     "  walkingpad-pp-cli calories --days 7 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ppath, err := profilePath()
			if err != nil {
				return err
			}
			prof, ok, err := profile.Load(ppath)
			if err != nil {
				return err
			}
			if !ok {
				return emit(cmd, flags, map[string]any{"error": "no weight set"},
					"set your weight first: walkingpad-pp-cli profile set --weight <kg>")
			}
			store, err := openHistory()
			if err != nil {
				return err
			}
			series, err := store.DailySeries(days, time.Now())
			if err != nil {
				return err
			}
			now := time.Now()
			var totalKcal float64
			perDay := make([]map[string]any, 0, len(series))
			for _, d := range series {
				sessions, err := store.SessionsOn(d.Date, now.Location())
				if err != nil {
					return err
				}
				var dayKcal float64
				for _, s := range sessions {
					dayKcal += profile.Calories(prof.WeightKg, s.AvgSpeedKmh, s.DurationS)
				}
				totalKcal += dayKcal
				perDay = append(perDay, map[string]any{"date": d.Date, "kcal": round1(dayKcal)})
			}
			if flags.asJSON {
				return writeJSON(cmd, map[string]any{"weight_kg": prof.WeightKg, "total_kcal": round1(totalKcal), "days": perDay})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "estimated %.0f kcal over %d days (weight %.0f kg)\n", totalKcal, days, prof.WeightKg)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 1, "Number of days to include")
	return cmd
}

func newExportCmd(flags *rootFlags) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export daily totals as JSON (e.g. for an Apple Health Shortcut)",
		Long:        "Write daily.json (last 60 days) and yesterday.json from the local history store. Point an iPhone Shortcut at the file to log Walking workouts into Apple Health.",
		Example:     "  walkingpad-pp-cli export --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openHistory()
			if err != nil {
				return err
			}
			if path == "" {
				path = filepath.Join(store.Dir(), "export", "daily.json")
			}
			now := time.Now()
			series, err := store.DailySeries(60, now)
			if err != nil {
				return err
			}
			daily := map[string]any{
				"generated_at": now.Format(time.RFC3339),
				"days":         series,
			}
			if err := writeJSONFile(path, daily); err != nil {
				return err
			}
			yPath := filepath.Join(filepath.Dir(path), "yesterday.json")
			yDate := now.AddDate(0, 0, -1).Format("2006-01-02")
			yTotals, err := store.TotalsOn(yDate, now.Location())
			if err != nil {
				return err
			}
			if err := writeJSONFile(yPath, yTotals); err != nil {
				return err
			}
			return emit(cmd, flags, map[string]any{"daily": path, "yesterday": yPath},
				fmt.Sprintf("wrote %s and %s", path, yPath))
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Output path for daily.json (default: history dir/export/daily.json)")
	return cmd
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode export: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	return os.Rename(tmp, path)
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
