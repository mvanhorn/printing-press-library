package cli

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/device"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
)

// traceEnvVar enables runBelt's stderr command/status trace when set.
const traceEnvVar = "WALKINGPAD_TRACE"

// startRunTimeout bounds how long runBelt waits for the belt to report the
// running state after start before setting the speed anyway. The 3-2-1 start
// countdown normally completes in ~3.5s.
const startRunTimeout = 8 * time.Second

// newRunCmd is the headline control command: it holds one BLE connection for the
// whole walk (the only reliable way to keep the belt running), starts the belt,
// sets the speed, streams live status, records the walk to history, and stops the
// belt when --duration elapses or on interrupt. A one-shot `start` does not
// sustain the belt; `run` is what actually walks.
func newRunCmd(flags *rootFlags) *cobra.Command {
	var speed float64
	var durationStr string
	var confirm bool
	cmd := &cobra.Command{
		Use: "run",
		// Hidden from MCP: holds a BLE connection for the whole walk (long-running)
		// and is physical-effect — not viable as a one-shot MCP tool.
		Annotations: map[string]string{"mcp:hidden": "true"},
		Short:       "Start a guided walk: run the belt over a held connection and record it",
		Long:        "Hold one BLE connection, switch to manual, start the belt at --speed, stream live status, record the walk to local history, and stop the belt when --duration elapses (or on Ctrl-C). This is the reliable way to run the belt; a one-shot `start` does not sustain it. Physical-effect: requires --live and --confirm-physical-effect (or --dry-run to preview). The pad must be physically awake (display on).",
		Example:     "  walkingpad-pp-cli run --speed 2.0 --duration 30m --live --confirm-physical-effect",
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyNoop(cmd, flags, "run"); done {
				return err
			}
			if speed < 0 || speed > wpble.MaxSpeedKmh || (speed > 0 && speed < wpble.MinSpeedKmh) {
				return fmt.Errorf("--speed %.1f out of range; want 0 or %.1f-%.1f km/h", speed, wpble.MinSpeedKmh, wpble.MaxSpeedKmh)
			}
			var duration time.Duration
			if durationStr != "" {
				d, err := time.ParseDuration(durationStr)
				if err != nil {
					return fmt.Errorf("--duration: %w", err)
				}
				duration = d
			}
			if flags.dryRun {
				return emit(cmd, flags,
					map[string]any{"action": "run", "dry_run": true, "speed_kmh": speed, "safety": "physical-effect"},
					fmt.Sprintf("would run the belt at %.1f km/h for %s", speed, runDurationLabel(duration)))
			}
			if !confirm {
				return fmt.Errorf("run is physical-effect; pass --dry-run to preview or --confirm-physical-effect to start the belt")
			}
			if !flags.live {
				return emit(cmd, flags, map[string]any{"action": "run", "sent": false, "reason": "not live"},
					"not contacting the belt; pass --live to run it (or --dry-run to preview)")
			}
			ctx := cmd.Context()
			if d, bounded := boundedContext(cmd, duration); bounded {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
			}
			return captureAndSave(cmd, flags, func(onStatus func(wpble.Status) error) error {
				return runBelt(ctx, flags, speed, onStatus)
			})
		},
	}
	cmd.Flags().Float64Var(&speed, "speed", 0, "Belt speed in km/h (0.5-6.0; 0 lets you set speed manually on the belt)")
	cmd.Flags().StringVar(&durationStr, "duration", "", "How long to run (e.g. 30m); empty = until interrupted")
	cmd.Flags().BoolVar(&confirm, "confirm-physical-effect", false, "Confirm a physical-effect write")
	return cmd
}

// stopBelt idles the belt with the firmware's stop ceremony: zero the speed, let
// it settle, then switch to standby. Speed-0 alone leaves the belt in manual mode
// (still running under a walker's weight); the standby switch is what actually
// stops it. Mirrors ph4r05/ph4-walkingpad's stop_belt -> sleep -> switch_mode(STANDBY).
func stopBelt(ctx context.Context, link device.Link) error {
	if err := link.Write(wpble.WriteUUID, wpble.CmdStop()); err != nil {
		return err
	}
	if err := sleepCtx(ctx, modeSettleDelay); err != nil {
		return err
	}
	if err := link.Write(wpble.WriteUUID, wpble.CmdMode(wpble.ModeStandby)); err != nil {
		return err
	}
	// Brief flush so a write-without-response standby reaches the belt before the
	// caller closes the link.
	return sleepCtx(ctx, 300*time.Millisecond)
}

// newStopCmd stops the belt with the firmware's stop ceremony over a held
// connection. There is no one-shot stop: a single speed-0 write leaves the belt
// in manual mode (still running under weight); the standby switch is what idles it.
func newStopCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:         "stop",
		Annotations: map[string]string{"mcp:hidden": "true"}, // held-connection physical-effect; not a one-shot MCP tool
		Short:       "Stop the belt (speed-0, settle, then switch to standby)",
		Long:        "Stop the belt with the firmware's stop ceremony: zero the speed, let it settle, then switch to standby. A one-shot speed-0 write does not idle a walked belt. Physical-effect: requires --live and --confirm-physical-effect (or --dry-run to preview).",
		Example:     "  walkingpad-pp-cli stop --live --confirm-physical-effect",
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyNoop(cmd, flags, "stop"); done {
				return err
			}
			if flags.dryRun {
				return emit(cmd, flags, map[string]any{"action": "stop", "dry_run": true, "safety": "physical-effect"},
					"would stop the belt (speed-0 -> settle -> standby)")
			}
			if !confirm {
				return fmt.Errorf("stop is physical-effect; pass --dry-run to preview or --confirm-physical-effect to stop the belt")
			}
			if !flags.live {
				return emit(cmd, flags, map[string]any{"action": "stop", "sent": false, "reason": "not live"},
					"not contacting the belt; pass --live to stop it (or --dry-run to preview)")
			}
			ctx := cmd.Context()
			link, err := dialBelt(ctx, flags)
			if err != nil {
				return err
			}
			defer func() { _ = link.Close() }()
			// Subscribe before the handshake so the belt's responses land, establish
			// the control session, then run the stop ceremony.
			if err := link.Subscribe(wpble.NotifyUUID, func([]byte) {}); err != nil {
				return err
			}
			if err := device.Handshake(ctx, link); err != nil {
				return err
			}
			if err := stopBelt(ctx, link); err != nil {
				return err
			}
			return emit(cmd, flags, map[string]any{"action": "stop", "sent": true}, "stop sent")
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm-physical-effect", false, "Confirm a physical-effect write")
	return cmd
}

// runBelt opens a BLE link, switches to manual mode, starts the belt, sets the
// desired speed, streams status to onStatus, and stops the belt on ctx done.
func runBelt(ctx context.Context, flags *rootFlags, speedKmh float64, onStatus func(wpble.Status) error) error {
	link, err := dialBelt(ctx, flags)
	if err != nil {
		return err
	}
	defer func() { _ = link.Close() }()

	// WALKINGPAD_TRACE=1 traces every write and status frame to stderr, for
	// diagnosing whether mode and belt_state match expectations during a live run.
	trace := os.Getenv(traceEnvVar) != ""
	traceStart := time.Now()
	tracef := func(format string, a ...any) {
		if !trace {
			return
		}
		fmt.Fprintf(os.Stderr, "[wp-trace +%6.2fs] "+format+"\n",
			append([]any{time.Since(traceStart).Seconds()}, a...)...)
	}

	// Pace writes >= cmdSpacing apart globally (the belt drops commands sent
	// closer) without blocking other writers during the wait. Each caller reserves
	// the next time slot under pacingMu, sleeps until it outside any lock, then
	// writes under writeMu, so the status poll and control writes never stall each
	// other.
	var pacingMu, writeMu sync.Mutex
	var nextSlot time.Time
	write := func(payload []byte) error {
		pacingMu.Lock()
		slot := nextSlot
		if now := time.Now(); slot.Before(now) {
			slot = now
		}
		nextSlot = slot.Add(cmdSpacing)
		pacingMu.Unlock()
		if d := time.Until(slot); d > 0 {
			time.Sleep(d)
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		if trace {
			tracef("-> write %s", wpble.DescribeCommand(payload))
		}
		return link.Write(wpble.WriteUUID, payload)
	}

	// running fires once belt_state reaches the running state, letting the start
	// ceremony wait out the start countdown before setting speed. subErr carries an
	// onStatus error back to the caller; it is mutex-guarded because the subscribe
	// callback runs on the BLE notification goroutine while the main goroutine reads
	// it after cancellation.
	running := make(chan struct{}, 1)
	var subErrMu sync.Mutex
	var subErr error
	if err := link.Subscribe(wpble.NotifyUUID, func(data []byte) {
		s, ok := wpble.ParseStatus(data)
		if !ok {
			return
		}
		if trace {
			tracef("<- status mode=%d(%s) belt_state=%d speed=%.1f steps=%d",
				s.Mode, s.ModeName, s.BeltState, s.SpeedKmh, s.Steps)
		}
		if s.BeltState == wpble.BeltStateRunning {
			select {
			case running <- struct{}{}:
			default:
			}
		}
		if callErr := onStatus(s); callErr != nil {
			subErrMu.Lock()
			subErr = callErr
			subErrMu.Unlock()
		}
	}); err != nil {
		return err
	}
	loadSubErr := func() error {
		subErrMu.Lock()
		defer subErrMu.Unlock()
		return subErr
	}
	// Subscribe must precede the handshake so the belt's profile/status responses
	// land in an active subscription; without the handshake the belt accepts the
	// connection but never streams status.
	tracef("handshake start")
	if err := device.Handshake(ctx, link); err != nil {
		return err
	}
	tracef("handshake done")

	// Poll ask_stats in a dedicated goroutine for the whole session so the live
	// readout and the recorded session keep receiving status frames — the belt
	// only reports status in response to a poll. This is NOT a keep-alive: a
	// started belt sustains itself in manual mode with no traffic at all (ph4r05
	// only polls when asked for --stats). The goroutine stays lock-free so the
	// poll never stalls behind a control write.
	pollCtx, stopPoll := context.WithCancel(ctx)
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		tick := time.NewTicker(statsPollInterval)
		defer tick.Stop()
		for {
			_ = write(wpble.CmdAskStats())
			select {
			case <-pollCtx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	// Stop the monitor and let it drain before the stop ceremony writes, so the
	// two never write the link concurrently. Runs before the link.Close above.
	defer func() {
		stopPoll()
		<-pollDone
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = stopBelt(stopCtx, link)
	}()

	// Drive ph4r05's start ceremony: switch to manual, let the switch commit, then
	// start. Speed is a separate command on the already-running belt, as ph4r05
	// does it.
	if err := write(wpble.CmdMode(wpble.ModeManual)); err != nil {
		return fmt.Errorf("switch to manual: %w", err)
	}
	// Let the mode switch commit before start; ph4r05 waits this long ("needs to
	// sleep a bit") and the belt ignores a start that arrives too soon.
	if err := sleepCtx(ctx, modeSettleDelay); err != nil {
		return err
	}
	if err := write(wpble.CmdStart()); err != nil {
		return fmt.Errorf("start belt: %w", err)
	}
	// Set the requested speed once, after the belt is actually running. A speed
	// command sent during the start countdown is ignored and the belt keeps its
	// start-speed preference instead of --speed, so wait for the running state
	// first. Set it once and never re-send: the firmware beeps on every
	// change_speed, so re-asserting on a timer beeps the whole walk. The belt holds
	// the speed on its own while you walk; it auto-stops by design only when you
	// step off and steps stop registering.
	if speedKmh > 0 {
		select {
		case <-running:
		case <-time.After(startRunTimeout):
		case <-ctx.Done():
			return loadSubErr()
		}
		if err := write(wpble.CmdSpeed(wpble.PadSpeed(speedKmh))); err != nil {
			return fmt.Errorf("set speed: %w", err)
		}
	}

	// Hold the connection until the run ends (--duration elapses or Ctrl-C); the
	// background poll keeps status flowing for the live readout and recording.
	<-ctx.Done()
	return loadSubErr()
}

func runDurationLabel(d time.Duration) string {
	if d <= 0 {
		return "until interrupted"
	}
	return d.String()
}
