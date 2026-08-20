// WalkingPad protocol codec. Operator-owned (NOT generated): this is the
// device-specific wire format the generic transport cannot derive from static
// evidence. It implements device.DeviceCodec and registers itself, so the
// generated parameterized commands (set-speed, set-mode) and the generated
// status command route through it. The pure-Go framing/parsing lives in
// internal/wpble; this file only adapts it to the generated interface.

package device

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
)

func init() {
	codec = walkingPadCodec{}
	telemetrySnapshot = walkingPadSnapshot
}

// Handshake runs the WalkingPad connection ceremony (profile query + init beep,
// with the firmware's spacing) that makes the belt stream status and accept
// control. The caller must already be subscribed to the notify characteristic so
// the belt's responses are not dropped (per ph4r05/ph4-walkingpad's connect
// order). It is NOT a Dial-time connectInit: Dial runs before the caller
// subscribes. The held-connection commands (run, monitor) call it after
// Subscribe, and walkingPadSnapshot reuses it.
func Handshake(ctx context.Context, link Link) error {
	steps := []struct {
		wait time.Duration
		cmd  []byte
	}{
		{1500 * time.Millisecond, wpble.CmdAskProfile()},
		{1500 * time.Millisecond, wpble.CmdInitBeep()},
		{1000 * time.Millisecond, nil},
	}
	for _, step := range steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(step.wait):
		}
		if step.cmd != nil {
			if err := link.Write(wpble.WriteUUID, step.cmd); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkingPadSnapshot captures one current-status frame so the generated status
// command can report live telemetry from this notify-only belt: subscribe, run
// the handshake, then poll ask-stats until the first status notification
// arrives. Registered as telemetrySnapshot, so status and run/monitor share one
// read path. Polls no faster than the firmware's ~0.69s command floor.
func walkingPadSnapshot(ctx context.Context, link Link) ([]byte, error) {
	frames := make(chan []byte, 1)
	if err := link.Subscribe(wpble.NotifyUUID, func(data []byte) {
		if wpble.IsStatus(data) {
			select {
			case frames <- data:
			default:
			}
		}
	}); err != nil {
		return nil, err
	}
	if err := Handshake(ctx, link); err != nil {
		return nil, err
	}
	tick := time.NewTicker(700 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := link.Write(wpble.WriteUUID, wpble.CmdAskStats()); err != nil {
			return nil, err
		}
		select {
		case frame := <-frames:
			return frame, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}

type walkingPadCodec struct{}

// EncodeCommand builds the BLE frame for a command. Parameterized commands
// (set-speed, set-mode) compute their frame from the CLI args; fixed commands
// (start, stop, wake) write their captured static payload.
func (walkingPadCodec) EncodeCommand(command CommandDefinition, args []string) ([]byte, error) {
	switch command.Name {
	case "set-speed":
		if len(args) < 1 {
			return nil, fmt.Errorf("set-speed needs a km/h value (0 stops; %.1f-%.1f)", wpble.MinSpeedKmh, wpble.MaxSpeedKmh)
		}
		kmh, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return nil, fmt.Errorf("speed must be a number in km/h (e.g. 3.0): %w", err)
		}
		if kmh < 0 || kmh > wpble.MaxSpeedKmh || (kmh > 0 && kmh < wpble.MinSpeedKmh) {
			return nil, fmt.Errorf("speed %.1f out of range; want 0 (stop) or %.1f-%.1f km/h", kmh, wpble.MinSpeedKmh, wpble.MaxSpeedKmh)
		}
		return wpble.CmdSpeed(wpble.PadSpeed(kmh)), nil
	case "set-mode":
		if len(args) < 1 {
			return nil, fmt.Errorf("set-mode needs a mode: auto, manual, or standby")
		}
		mode, err := wpble.ModeFromName(args[0])
		if err != nil {
			return nil, err
		}
		return wpble.CmdMode(mode), nil
	default:
		return hex.DecodeString(command.PayloadHex)
	}
}

// DecodeTelemetry parses a WalkingPad status notification frame into the value
// for one telemetry field. (WalkingPad telemetry is notify-based, so the live
// snapshot comes from the hand-authored monitor command; this decoder lets the
// generated status surface decoded values wherever a frame is available.)
func (walkingPadCodec) DecodeTelemetry(field StatusField, raw []byte) (any, error) {
	status, ok := wpble.ParseStatus(raw)
	if !ok {
		return nil, nil
	}
	switch field.Name {
	case "belt_state":
		return status.BeltState, nil
	case "speed_kmh":
		return status.SpeedKmh, nil
	case "mode":
		return status.ModeName, nil
	case "time_s":
		return status.TimeS, nil
	case "distance_m":
		return status.DistanceM, nil
	case "steps":
		return status.Steps, nil
	default:
		return nil, nil
	}
}
