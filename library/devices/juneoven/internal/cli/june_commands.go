package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/juneoven/internal/june"

	"github.com/spf13/cobra"
)

const ackListen = 6 * time.Second

// resolveTarget converts a --temp value plus --celsius into milli-°C.
func resolveTarget(temp float64, celsius bool) int {
	if celsius {
		return june.CelsiusToMilliC(temp)
	}
	return june.FahrenheitToMilliC(temp)
}

func newPairCmd(flags *rootFlags) *cobra.Command {
	var deviceName string
	cmd := &cobra.Command{
		Use:     "pair",
		Short:   "Pair this CLI directly with your June oven",
		Long:    "Register a fresh companion and pair by typing an 8-digit code on the oven's screen (swipe left twice from home, tap Connect). No June account login. Credentials are stored at 0600 and control the oven, so treat them as secrets.",
		Example: "  juneoven-pp-cli pair",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would start the pairing flow")
				return nil
			}
			out := cmd.OutOrStdout()
			id, err := june.Pair(cmd.Context(), deviceName, func(p june.PairProgress) {
				if p.Code != "" {
					fmt.Fprintf(out, "\n  On the oven, tap Connect and enter this code:  %s %s\n\n", p.Code[:4], p.Code[4:])
					return
				}
				if p.Status != "" && !flags.quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", p.Status)
				}
			})
			if err != nil {
				return err
			}
			result := map[string]string{"paired": "true", "oven_id": id.OvenID}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(out, result, flags)
			}
			fmt.Fprintf(out, "Paired with oven %s. Run 'juneoven-pp-cli status' to confirm.\n", id.OvenID)
			return nil
		},
	}
	cmd.Flags().StringVar(&deviceName, "name", "June CLI", "Display name shown on the oven's device list")
	return cmd
}

func newStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show the oven's connection and cook state",
		Long:        "Read the oven's current status from June's cloud: whether it's online, idle or active, and the current target temperature.",
		Example:     "  juneoven-pp-cli status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch oven status")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			raw, err := june.NewClient(id).Status(cmd.Context())
			if err != nil {
				return err
			}
			view, err := june.ParseStatus(raw)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
}

func newPreheatCmd(flags *rootFlags) *cobra.Command {
	var temp float64
	var mode string
	var celsius bool
	cmd := &cobra.Command{
		Use:     "preheat",
		Short:   "Start a preheat/cook at a target temperature",
		Long:    "Start a cook using the oven's bake or roast primitive at a target temperature (°F by default, °C with --celsius). June cannot retarget a running cook, so issuing preheat while active cancels and restarts.",
		Example: "  juneoven-pp-cli preheat --temp 350\n  juneoven-pp-cli preheat --mode roast --temp 400",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode = strings.ToLower(mode)
			if mode != "bake" && mode != "roast" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--mode must be bake or roast"))
			}
			milliC := resolveTarget(temp, celsius)
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would preheat %s to %.0f%s (%d milli-°C)\n", mode, temp, unitLabel(celsius), milliC)
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			res, err := june.SendCommand(cmd.Context(), id, june.CodePreheat, june.PreheatData(mode, milliC), ackListen)
			if err != nil {
				return err
			}
			return emitAck(cmd, flags, res, fmt.Sprintf("preheat %s to %.0f%s", mode, temp, unitLabel(celsius)))
		},
	}
	cmd.Flags().Float64Var(&temp, "temp", 350, "Target temperature")
	cmd.Flags().StringVar(&mode, "mode", "bake", "Cook primitive: bake or roast")
	cmd.Flags().BoolVar(&celsius, "celsius", false, "Interpret --temp as °C")
	return cmd
}

func newTempCmd(flags *rootFlags) *cobra.Command {
	var temp float64
	var celsius bool
	cmd := &cobra.Command{
		Use:     "temp",
		Short:   "Change the target temperature of the active cook",
		Long:    "Change the target temperature of a running cook. Note June often rejects a live retarget; if so the ack is not-allowed and you should cancel and preheat instead.",
		Example: "  juneoven-pp-cli temp --temp 375",
		RunE: func(cmd *cobra.Command, args []string) error {
			milliC := resolveTarget(temp, celsius)
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would set target to %.0f%s (%d milli-°C)\n", temp, unitLabel(celsius), milliC)
				return nil
			}
			if temp <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--temp is required"))
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			res, err := june.SendCommand(cmd.Context(), id, june.CodeTemp, june.TempData(milliC), ackListen)
			if err != nil {
				return err
			}
			return emitAck(cmd, flags, res, fmt.Sprintf("set target to %.0f%s", temp, unitLabel(celsius)))
		},
	}
	cmd.Flags().Float64Var(&temp, "temp", 0, "New target temperature")
	cmd.Flags().BoolVar(&celsius, "celsius", false, "Interpret --temp as °C")
	return cmd
}

func newTimerCmd(flags *rootFlags) *cobra.Command {
	var minutes float64
	cmd := &cobra.Command{
		Use:     "timer",
		Short:   "Set a cook timer",
		Long:    "Set a timer on the active cook, in minutes.",
		Example: "  juneoven-pp-cli timer --minutes 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			ms := int(minutes * 60 * 1000)
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would set a %.0f-minute timer\n", minutes)
				return nil
			}
			if minutes <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--minutes must be greater than 0"))
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			res, err := june.SendCommand(cmd.Context(), id, june.CodeTimer, june.TimerData(ms), ackListen)
			if err != nil {
				return err
			}
			return emitAck(cmd, flags, res, fmt.Sprintf("set a %.0f-minute timer", minutes))
		},
	}
	cmd.Flags().Float64Var(&minutes, "minutes", 0, "Timer duration in minutes")
	return cmd
}

func newCancelCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "cancel",
		Short:   "Cancel the active cook",
		Long:    "Stop the current cook. Cancelling while the oven is already idle returns a not-allowed ack, which is reported as a normal result, not an error.",
		Example: "  juneoven-pp-cli cancel",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would cancel the active cook")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			res, err := june.SendCommand(cmd.Context(), id, june.CodeCancel, june.CancelData(), ackListen)
			if err != nil {
				return err
			}
			return emitAck(cmd, flags, res, "cancel")
		},
	}
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var seconds int
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Stream live cook telemetry as JSON lines",
		Long:        "Hold the oven socket open and emit one JSON line per telemetry, state, or camera event until the cook ends, the timeout elapses, or you interrupt.",
		Example:     "  juneoven-pp-cli watch --seconds 120",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would stream oven telemetry")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if seconds > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
				defer cancel()
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			return june.Watch(ctx, id, func(ev june.TelemetryEvent) { _ = enc.Encode(ev) })
		},
	}
	cmd.Flags().IntVar(&seconds, "seconds", 0, "Stop after N seconds (0 = until cook ends or interrupted)")
	return cmd
}

func newCamCmd(flags *rootFlags) *cobra.Command {
	var timeout int
	cmd := &cobra.Command{
		Use:         "cam",
		Short:       "Fetch the next interior camera frame URL",
		Long:        "Wait up to --timeout seconds for the oven to push a camera frame and print its signed URL. Frames are typically only pushed during an active cook.",
		Example:     "  juneoven-pp-cli cam --timeout 15",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would wait for a camera frame")
				return nil
			}
			id, err := june.LoadIdentity()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second)
			defer cancel()
			var url string
			err = june.Watch(ctx, id, func(ev june.TelemetryEvent) {
				if ev.Type == "camera" && url == "" {
					url = ev.CameraURL
					cancel()
				}
			})
			if url == "" {
				out := map[string]any{"frame_available": false, "note": "no camera frame within timeout; frames are typically only pushed during an active cook. Raise --timeout or start a cook."}
				_ = printJSONFiltered(cmd.OutOrStdout(), out, flags)
				return usageErr(fmt.Errorf("no camera frame available"))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"signed_url": url}, flags)
		},
	}
	cmd.Flags().IntVar(&timeout, "timeout", 15, "Seconds to wait for a frame")
	return cmd
}

func unitLabel(celsius bool) string {
	if celsius {
		return "°C"
	}
	return "°F"
}

// emitAck reports the oven's ack for a command, in JSON or human form.
func emitAck(cmd *cobra.Command, flags *rootFlags, res june.CommandResult, action string) error {
	out := map[string]any{"action": action, "acked": res.Acked, "status": res.Status}
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}
	if !res.Acked {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: no ack from oven within %s\n", action, ackListen)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", action, res.Status)
	return nil
}
