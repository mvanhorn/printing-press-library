package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/juneoven/internal/june"

	"github.com/spf13/cobra"
)

// ready — block until the oven reaches its active target.
// pp:data-source live
func newReadyCmd(flags *rootFlags) *cobra.Command {
	var timeoutMin int
	var tolerance int
	cmd := &cobra.Command{
		Use:         "ready",
		Short:       "Block until the oven finishes preheating",
		Long:        "Wait for the active cook's cavity temperature to reach its target, then exit 0. Exits non-zero on timeout. Use this to synchronously gate the next step of a cook. Do NOT use 'watch' for this — watch streams telemetry and never ends on its own. Do NOT use 'eta', which predicts and returns immediately without waiting.",
		Example:     "  juneoven-pp-cli ready --timeout 20",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would block until the oven reaches target")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			res, err := june.WaitReady(cmd.Context(), id, tolerance, time.Duration(timeoutMin)*time.Minute)
			if err != nil {
				return err
			}
			_ = printJSONFiltered(cmd.OutOrStdout(), res, flags)
			if !res.Ready {
				return exitErr(4, fmt.Errorf("timed out after %dm at %d°F (target %d°F)", timeoutMin, res.FinalF, res.TargetF))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutMin, "timeout", 20, "Give up after N minutes")
	cmd.Flags().IntVar(&tolerance, "tolerance", 1, "Consider ready within this many °F of target")
	return cmd
}

// record — persist a live cook + telemetry to the local store.
// pp:data-source live
func newRecordCmd(flags *rootFlags) *cobra.Command {
	var label string
	var maxMin int
	cmd := &cobra.Command{
		Use:     "record",
		Short:   "Capture the current cook and its temperature stream to local history",
		Long:    "Attach to the active cook and write the session plus every telemetry sample into local SQLite, so 'log', 'curve', and 'preheat-stats' have data. June's cloud keeps no history; this is the only durable record. Do NOT use 'watch', which prints telemetry but persists nothing.",
		Example: "  juneoven-pp-cli record --label sourdough",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would record the active cook to local history")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			cs, err := june.OpenCookStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cs.Close()
			ctx := cmd.Context()
			if maxMin > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(maxMin)*time.Minute)
				defer cancel()
			}
			res, err := june.Record(ctx, id, cs, label)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Optional label for this cook (e.g. sourdough)")
	cmd.Flags().IntVar(&maxMin, "max-minutes", 0, "Stop recording after N minutes (0 = until the cook ends)")
	return cmd
}

// log — list past cooks from the local store.
// pp:data-source local
func newLogCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sinceStr string
	cmd := &cobra.Command{
		Use:         "log",
		Short:       "List recorded cooks from local history",
		Long:        "List past cook sessions (mode, target, duration, outcome) captured by 'record'. This data does not exist in June's cloud.",
		Example:     "  juneoven-pp-cli log --limit 20\n  juneoven-pp-cli log --since 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list recorded cooks")
				return nil
			}
			since, err := parseSince(sinceStr)
			if err != nil {
				return usageErr(err)
			}
			cs, err := june.OpenCookStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cs.Close()
			sessions, err := cs.ListSessions(cmd.Context(), limit, since)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), sessions, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max cooks to return")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only cooks within this window (e.g. 7d, 24h)")
	return cmd
}

// curve — export one session's temperature samples.
// pp:data-source local
func newCurveCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:         "curve <session-id>",
		Short:       "Export one cook's temperature curve",
		Long:        "Emit the timestamp/cavity-temp/progress samples for one recorded session as JSON or CSV, for plotting or piping. Use 'log' to find session ids and 'preheat-stats' for aggregate timing.",
		Example:     "  juneoven-pp-cli curve 3 --format csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export a session's temperature curve")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("session id is required"))
			}
			sid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("session id must be a number"))
			}
			cs, err := june.OpenCookStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cs.Close()
			samples, err := cs.SessionSamples(cmd.Context(), sid)
			if err != nil {
				return err
			}
			if format == "csv" {
				w := csv.NewWriter(cmd.OutOrStdout())
				_ = w.Write([]string{"ts", "cavity_f", "progress"})
				for _, s := range samples {
					_ = w.Write([]string{s.TS, strconv.Itoa(s.CavityF), strconv.Itoa(s.Progress)})
				}
				w.Flush()
				return w.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), samples, flags)
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	return cmd
}

// preheat-stats — derive per-mode preheat timing from history.
// pp:data-source computed
func newPreheatStatsCmd(flags *rootFlags) *cobra.Command {
	var cook string
	cmd := &cobra.Command{
		Use:         "preheat-stats",
		Short:       "How fast your oven preheats, per cook, from recorded history",
		Long:        "Compute median, fastest, and slowest time-to-target grouped by June's cook name over recorded sessions (the CLI's own preheat/repeat report 'bake'/'roast'; app recipes report the dish name). Requires cooks captured with 'record'.",
		Example:     "  juneoven-pp-cli preheat-stats\n  juneoven-pp-cli preheat-stats --cook bake",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute preheat statistics")
				return nil
			}
			cs, err := june.OpenCookStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cs.Close()
			stats, err := cs.PreheatStats(cmd.Context(), cook)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}
	cmd.Flags().StringVar(&cook, "cook", "", "Restrict to one cook name (e.g. bake, roast)")
	return cmd
}

// eta — non-blocking predicted time-to-ready.
// pp:data-source computed
func newEtaCmd(flags *rootFlags) *cobra.Command {
	var sampleSec int
	cmd := &cobra.Command{
		Use:         "eta",
		Short:       "Predict time until the oven reaches target, without blocking",
		Long:        "Sample live telemetry briefly, estimate the climb rate, and return a machine-readable ETA to target. Returns immediately. Use 'ready' instead if you want to block until preheat actually completes.",
		Example:     "  juneoven-pp-cli eta",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would estimate time-to-ready")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			res, err := june.LiveETA(cmd.Context(), id, time.Duration(sampleSec)*time.Second)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().IntVar(&sampleSec, "sample-seconds", 8, "How long to sample telemetry for the climb rate")
	return cmd
}

// repeat — save and replay named cooks.
// pp:data-source local
func newRepeatCmd(flags *rootFlags) *cobra.Command {
	var save, list bool
	var mode string
	var temp float64
	var timerMin int
	var celsius bool
	cmd := &cobra.Command{
		Use:     "repeat [name]",
		Short:   "Save and re-run named cooks",
		Long:    "Save a named mode+temp(+timer) preset and replay it later with one word. Use 'preheat' for a one-off cook with explicit arguments; 'repeat' stores and re-runs.",
		Example: "  juneoven-pp-cli repeat --save sunday-roast --mode roast --temp 375 --timer 90\n  juneoven-pp-cli repeat sunday-roast\n  juneoven-pp-cli repeat --list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			cs, err := june.OpenCookStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cs.Close()
			ctx := cmd.Context()

			switch {
			case list:
				presets, err := cs.ListPresets(ctx)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), presets, flags)
			case save:
				if len(args) == 0 {
					return usageErr(fmt.Errorf("a name is required to --save"))
				}
				tf := int(temp)
				if celsius {
					tf = june.MilliCToFahrenheit(june.CelsiusToMilliC(temp))
				}
				p := june.Preset{Name: args[0], Mode: mode, TargetF: tf, TimerMin: timerMin}
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_save": p}, flags)
				}
				if err := cs.SavePreset(ctx, p); err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"saved": p}, flags)
			default:
				if len(args) == 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("a preset name is required (or use --save / --list)"))
				}
				p, ok, err := cs.GetPreset(ctx, args[0])
				if err != nil {
					return err
				}
				if !ok {
					return usageErr(fmt.Errorf("no preset named %q — save one with --save", args[0]))
				}
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_run": p}, flags)
				}
				id, err := june.LoadIdentity()
				if err != nil {
					return err
				}
				res, err := june.SendCommand(ctx, id, june.CodePreheat, june.PreheatData(p.Mode, june.FahrenheitToMilliC(float64(p.TargetF))), ackListen)
				if err != nil {
					return err
				}
				out := map[string]any{"preset": p.Name, "preheat": res}
				if p.TimerMin > 0 && res.Status == "success" {
					tres, terr := june.SendCommand(ctx, id, june.CodeTimer, june.TimerData(p.TimerMin*60*1000), ackListen)
					if terr == nil {
						out["timer"] = tres
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
		},
	}
	cmd.Flags().BoolVar(&save, "save", false, "Save a preset instead of running one")
	cmd.Flags().BoolVar(&list, "list", false, "List saved presets")
	cmd.Flags().StringVar(&mode, "mode", "bake", "Cook mode when saving")
	cmd.Flags().Float64Var(&temp, "temp", 0, "Target temperature when saving")
	cmd.Flags().IntVar(&timerMin, "timer", 0, "Optional timer in minutes when saving")
	cmd.Flags().BoolVar(&celsius, "celsius", false, "Interpret --temp as °C when saving")
	return cmd
}

// exitErr returns an error that maps to a specific process exit code.
func exitErr(code int, err error) error { return &cliError{code: code, err: err} }

func parseSince(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	// support 7d / 1w in addition to Go durations
	if n := len(s); n >= 2 {
		unit := s[n-1]
		if unit == 'd' || unit == 'w' {
			val, err := strconv.Atoi(s[:n-1])
			if err != nil {
				return 0, fmt.Errorf("invalid --since %q", s)
			}
			if unit == 'd' {
				return time.Duration(val) * 24 * time.Hour, nil
			}
			return time.Duration(val) * 7 * 24 * time.Hour, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q (try 7d, 24h)", s)
	}
	return d, nil
}
