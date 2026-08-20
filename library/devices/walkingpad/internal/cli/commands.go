// Hand-authored command layer (not generated). Adds the live control,
// telemetry, preferences, history, and analytics commands on top of the
// generated replay scaffold. registerWalkingPadCommands is the single hook
// called from the generated root.go via the novelCommands var.

package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/device"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/history"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/profile"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
)

const appName = "walkingpad-pp-cli"

// dogfoodMaxDuration caps long-running live commands under the dogfood matrix so
// they fit the runner's per-command timeout. Live commands also require --live,
// which the matrix does not pass, so this is defense in depth.
const dogfoodMaxDuration = 5 * time.Second

// verifyNoop short-circuits an actuator under verify mode before it parses
// arguments or touches hardware. It lets actuators accept the verifier's
// synthesized arguments without failing validation, and guarantees verify never
// actuates the belt (defense in depth alongside gateWrite's own verify guard).
func verifyNoop(cmd *cobra.Command, flags *rootFlags, name string) (bool, error) {
	if !cliutil.IsVerifyEnv() {
		return false, nil
	}
	return true, emit(cmd, flags, map[string]any{"command": name, "verify_noop": true, "success": false},
		"verify mode: not actuating the belt")
}

// emit prints value as JSON when --json/--agent is set, otherwise the text line.
func emit(cmd *cobra.Command, flags *rootFlags, value any, text string) error {
	if flags.asJSON {
		return writeJSON(cmd, value)
	}
	fmt.Fprintln(cmd.OutOrStdout(), text)
	return nil
}

// offlineGuard centralizes the "should this live command actually touch the
// belt" policy for read commands. It returns stop=true (and emits a notice)
// when the command must not contact hardware: under verify, or without --live.
func offlineGuard(cmd *cobra.Command, flags *rootFlags, what string) (stop bool, err error) {
	if cliutil.IsVerifyEnv() {
		return true, emit(cmd, flags, map[string]any{"verify_noop": true},
			"verify mode: not contacting the belt")
	}
	if !flags.live {
		return true, emit(cmd, flags, map[string]any{"live": false},
			"not contacting the belt; pass --live to "+what)
	}
	return false, nil
}

// gateWrite applies the dry-run / confirm / verify / live policy to a control
// or preference write, then dials the device and writes the payload.
func gateWrite(cmd *cobra.Command, flags *rootFlags, action, safety, payloadHex string, confirm bool) error {
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return fmt.Errorf("decode payload %q: %w", payloadHex, err)
	}
	needConfirm := safety == "physical-effect" || safety == "configuration-risk"

	if flags.dryRun {
		return emit(cmd, flags,
			map[string]any{"action": action, "dry_run": true, "payload_hex": payloadHex, "safety": safety},
			fmt.Sprintf("would %s (write %s, safety %s)", action, payloadHex, safety))
	}
	if needConfirm && !confirm && !cliutil.IsVerifyEnv() {
		return fmt.Errorf("%s is %s; pass --dry-run to preview or --confirm-physical-effect to send it", action, safety)
	}
	if cliutil.IsVerifyEnv() {
		return emit(cmd, flags,
			map[string]any{"action": action, "verify_noop": true, "success": false, "payload_hex": payloadHex},
			"verify mode: not actuating the belt")
	}
	if !flags.live {
		return emit(cmd, flags,
			map[string]any{"action": action, "sent": false, "reason": "not live", "payload_hex": payloadHex},
			fmt.Sprintf("not contacting the belt; pass --live to %s (or --dry-run to preview)", action))
	}
	link, err := dialBelt(cmd.Context(), flags)
	if err != nil {
		return err
	}
	defer func() { _ = link.Close() }()
	if err := link.Write(wpble.WriteUUID, payload); err != nil {
		return err
	}
	out := map[string]any{"action": action, "sent": true, "payload_hex": payloadHex}
	return emit(cmd, flags, out, action+" sent")
}

const (
	// modeSettleDelay is how long to wait after a mode switch before the belt
	// will honor a start; ph4r05/ph4-walkingpad waits this in start_belt. Recorded
	// as transport.settle_delays in the device spec.
	modeSettleDelay = 1500 * time.Millisecond
	// statsPollInterval is the ask-stats cadence that keeps the belt streaming
	// status; ph4r05 polls ~500ms. The generated transport already paces writes to
	// transport.command_spacing_ms. Recorded as transport.poll_cadence_ms.
	statsPollInterval = 500 * time.Millisecond
	// cmdSpacing is the minimum time between writes (ph4r05's minimal_cmd_space);
	// runBelt paces to it via reserved slots so the keep-alive never stalls.
	cmdSpacing = 690 * time.Millisecond
)

// sleepCtx waits for d, or returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// dialBelt opens a BLE link to the belt. The generated transport paces every
// write to transport.command_spacing_ms (690ms), so callers issue commands
// back-to-back without a hand-authored spacing wrapper.
func dialBelt(ctx context.Context, flags *rootFlags) (device.Link, error) {
	return device.Dial(ctx, flags.address, flags.timeout)
}

// openHistory opens the local history store at its default path.
func openHistory() (*history.Store, error) {
	dir, err := history.DefaultDir(appName)
	if err != nil {
		return nil, err
	}
	return history.Open(dir)
}

// profilePath returns the profile file path.
func profilePath() (string, error) {
	return profile.Path(appName)
}

// boundedContext derives a context that respects an optional duration and, under
// the dogfood matrix, is capped so a long-running command cannot trip the
// runner's timeout.
func boundedContext(cmd *cobra.Command, duration time.Duration) (ctxDeadline time.Duration, bounded bool) {
	if cliutil.IsDogfoodEnv() && (duration == 0 || duration > dogfoodMaxDuration) {
		return dogfoodMaxDuration, true
	}
	return duration, duration > 0
}

// dialAndMonitor opens a BLE link, subscribes to status notifications, runs the
// connection handshake, and calls onStatus for each decoded wpble.Status frame
// until ctx is cancelled. It sends CmdAskStats every interval to keep
// notifications flowing. Subscribe must precede the handshake so the belt's
// profile/status responses land in an active subscription.
func dialAndMonitor(ctx context.Context, flags *rootFlags, interval time.Duration, onStatus func(wpble.Status) error) error {
	link, err := dialBelt(ctx, flags)
	if err != nil {
		return err
	}
	defer func() { _ = link.Close() }()

	// subErr carries an onStatus error back to the caller; it is mutex-guarded
	// because the subscribe callback runs on the BLE notification goroutine while
	// this goroutine reads it in the poll loop.
	var subErrMu sync.Mutex
	var subErr error
	if err := link.Subscribe(wpble.NotifyUUID, func(data []byte) {
		s, ok := wpble.ParseStatus(data)
		if !ok {
			return
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

	if err := device.Handshake(ctx, link); err != nil {
		return err
	}

	// Poll the belt for status (>= ~0.69s spacing; faster polls are dropped).
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if err := link.Write(wpble.WriteUUID, wpble.CmdAskStats()); err != nil {
			return err
		}
		if err := loadSubErr(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return loadSubErr()
		case <-tick.C:
		}
	}
}
