// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"

	"github.com/spf13/cobra"
)

type setFlags struct {
	power          string
	speed          int
	mode           string
	oscillate      string
	oscillateAngle int
	timerOn        string
	timerOff       string
	heatLevel      int
	heatMode       string
	targetTemp     float64
	unit           string
	targetHumidity int
	fogLevel       int
	mistLevel      int
	direction      string
	light          string
	lightLevel     int
	colorTemp      int
	rgbColor       string
	rgbMode        string
	rgbLevel       int
	childLock      string
	display        string
	ledAlwaysOn    string
	voice          string
	mute           string
	waitForEcho    bool
}

func newSetCmd(rflags *rootFlags) *cobra.Command {
	sf := &setFlags{}
	cmd := &cobra.Command{
		Use:   "set <device>",
		Short: "Set state on one device (power, speed, mode, oscillation, timer) via WebSocket",
		Long: `Set state on a Dreo device over the WebSocket control plane.

Each flag maps to one or more fields in the control frame. Combine flags
to send a single combined frame (e.g. --power on --speed 4 --mode auto).

Use --dry-run to preview the frame without sending. Use --wait to block
up to 5 seconds for the server to echo a state update confirming the change.

Examples:
  dreo-pp-cli set bedroom-fan --power on --speed 4
  dreo-pp-cli set HTF008S-... --mode auto --oscillate horizontal --oscillate-angle 60
  dreo-pp-cli set humidifier --target-humidity 45 --mist-level 3`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			device := args[0]

			params, err := buildSetParams(cmd, sf)
			if err != nil {
				return usageErr(err)
			}
			if len(params) == 0 {
				return usageErr(errors.New("at least one state flag is required (--power, --speed, --mode, ...)"))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would set %s: %v\n", device, params)
				return nil
			}
			if dryRunOK(rflags) {
				out := map[string]any{
					"device":   device,
					"params":   params,
					"dry_run":  true,
					"endpoint": "wss://dreo-tech.com/websocket",
				}
				if rflags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), out, rflags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: device=%s params=%v\n", device, params)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dev, err := resolveDeviceByQuery(ctx, rflags, device)
			if err != nil {
				return notFoundErr(fmt.Errorf("set: %w", err))
			}
			wsConn, err := connectWS(ctx, rflags)
			if err != nil {
				return apiErr(fmt.Errorf("set: open WS: %w", err))
			}
			defer wsConn.Close()

			if err := wsConn.Send(dev.Sn, params); err != nil {
				return apiErr(fmt.Errorf("set: send frame: %w", err))
			}

			result := map[string]any{
				"device":      dev.Sn,
				"device_name": dev.Name,
				"sent":        params,
				"confirmed":   false,
			}

			if sf.waitForEcho {
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				defer wcancel()
				select {
				case upd, ok := <-wsConn.Updates():
					if ok {
						result["confirmed"] = true
						result["echo"] = upd.Fields
					}
				case <-wctx.Done():
					// timeout: leave confirmed=false
				}
			}

			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, rflags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s (%s): %v\n", dev.Name, dev.Sn, params)
			if sf.waitForEcho {
				fmt.Fprintf(cmd.OutOrStdout(), "  confirmed=%v\n", result["confirmed"])
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&sf.power, "power", "", "Turn device on or off (on|off)")
	f.IntVar(&sf.speed, "speed", 0, "Set fan/wind level (1-N depending on model)")
	f.StringVar(&sf.mode, "mode", "", "Set wind/operating mode (normal|natural|sleep|auto|turbo)")
	f.StringVar(&sf.oscillate, "oscillate", "", "Oscillation mode (horizontal|vertical|both|off)")
	f.IntVar(&sf.oscillateAngle, "oscillate-angle", 0, "Horizontal oscillation angle (30/60/90/120 typical)")
	f.StringVar(&sf.timerOn, "timer-on", "", "Timer to power on (e.g. 30m, 2h)")
	f.StringVar(&sf.timerOff, "timer-off", "", "Timer to power off (e.g. 30m, 2h)")
	f.IntVar(&sf.heatLevel, "heat-level", 0, "Heater level (1-3)")
	f.StringVar(&sf.heatMode, "heat-mode", "", "Heater mode (coolair|hotair|eco)")
	f.Float64Var(&sf.targetTemp, "target-temp", 0, "Target temperature (units set via --unit)")
	f.StringVar(&sf.unit, "unit", "", "Temperature unit (C|F)")
	f.IntVar(&sf.targetHumidity, "target-humidity", 0, "Target humidity (40-80 typical)")
	f.IntVar(&sf.fogLevel, "fog-level", 0, "Fog/cool-mist level")
	f.IntVar(&sf.mistLevel, "mist-level", 0, "Mist level (humidifier)")
	f.StringVar(&sf.direction, "direction", "", "Airflow direction (forward|reverse)")
	f.StringVar(&sf.light, "light", "", "Light power (on|off)")
	f.IntVar(&sf.lightLevel, "light-level", 0, "Light brightness (1-100)")
	f.IntVar(&sf.colorTemp, "color-temp", 0, "White-light color temperature")
	f.StringVar(&sf.rgbColor, "rgb-color", "", "RGB color (#RRGGBB hex)")
	f.StringVar(&sf.rgbMode, "rgb-mode", "", "RGB mode (static|cycle|breath)")
	f.IntVar(&sf.rgbLevel, "rgb-level", 0, "RGB brightness (1-100)")
	f.StringVar(&sf.childLock, "child-lock", "", "Child lock (on|off)")
	f.StringVar(&sf.display, "display", "", "Front panel display (on|off)")
	f.StringVar(&sf.ledAlwaysOn, "led-always-on", "", "Keep status LED on always (on|off)")
	f.StringVar(&sf.voice, "voice", "", "Voice prompts (on|off)")
	f.StringVar(&sf.mute, "mute", "", "Mute prompts/beeps (on|off)")
	f.BoolVar(&sf.waitForEcho, "wait", false, "Wait up to 5s for the server to echo the change")
	return cmd
}

// buildSetParams converts CLI flags into a Dreo WS params map. Only flags
// the user actually set show up in the output.
func buildSetParams(cmd *cobra.Command, sf *setFlags) (map[string]any, error) {
	p := map[string]any{}
	changed := cmd.Flags().Changed

	if changed("power") {
		b, err := parseOnOff(sf.power)
		if err != nil {
			return nil, fmt.Errorf("--power: %w", err)
		}
		p["poweron"] = b
	}
	if changed("speed") {
		p["windlevel"] = sf.speed
	}
	if changed("mode") {
		modeNum := map[string]int{
			"normal":  1,
			"natural": 2,
			"sleep":   3,
			"auto":    4,
			"turbo":   5,
		}[strings.ToLower(sf.mode)]
		if modeNum == 0 {
			return nil, fmt.Errorf("--mode: expected normal|natural|sleep|auto|turbo, got %q", sf.mode)
		}
		p["windmode"] = modeNum
		p["windtype"] = modeNum
	}
	if changed("oscillate") {
		v := strings.ToLower(sf.oscillate)
		oscNum := map[string]int{
			"off":        0,
			"horizontal": 1,
			"vertical":   2,
			"both":       3,
		}[v]
		if _, ok := map[string]bool{"off": true, "horizontal": true, "vertical": true, "both": true}[v]; !ok {
			return nil, fmt.Errorf("--oscillate: expected horizontal|vertical|both|off, got %q", sf.oscillate)
		}
		p["oscmode"] = oscNum
		p["shakehorizon"] = v == "horizontal" || v == "both"
	}
	if changed("oscillate-angle") {
		p["shakehorizonangle"] = sf.oscillateAngle
	}
	if changed("timer-on") {
		d, err := parseDuration(sf.timerOn)
		if err != nil {
			return nil, fmt.Errorf("--timer-on: %w", err)
		}
		p["timeron"] = int(d.Minutes())
	}
	if changed("timer-off") {
		d, err := parseDuration(sf.timerOff)
		if err != nil {
			return nil, fmt.Errorf("--timer-off: %w", err)
		}
		p["timeroff"] = int(d.Minutes())
	}
	if changed("heat-level") {
		p["htalevel"] = sf.heatLevel
	}
	if changed("heat-mode") {
		v := strings.ToLower(sf.heatMode)
		if v != "coolair" && v != "hotair" && v != "eco" {
			return nil, fmt.Errorf("--heat-mode: expected coolair|hotair|eco, got %q", sf.heatMode)
		}
		p["coolair"] = v == "coolair"
		p["hotair"] = v == "hotair"
		p["eco"] = v == "eco"
	}
	if changed("target-temp") {
		p["targetTemperature"] = sf.targetTemp
	}
	if changed("unit") {
		u := strings.ToUpper(strings.TrimSpace(sf.unit))
		switch u {
		case "C":
			p["tempunit"] = 0
		case "F":
			p["tempunit"] = 1
		default:
			return nil, fmt.Errorf("--unit: expected C|F, got %q", sf.unit)
		}
	}
	if changed("target-humidity") {
		p["targetHumidity"] = sf.targetHumidity
	}
	if changed("fog-level") {
		p["foglevel"] = sf.fogLevel
	}
	if changed("mist-level") {
		p["mistlevel"] = sf.mistLevel
	}
	if changed("direction") {
		v := strings.ToLower(sf.direction)
		if v != "forward" && v != "reverse" {
			return nil, fmt.Errorf("--direction: expected forward|reverse, got %q", sf.direction)
		}
		p["direction"] = v
	}
	if changed("light") {
		b, err := parseOnOff(sf.light)
		if err != nil {
			return nil, fmt.Errorf("--light: %w", err)
		}
		p["lighton"] = b
	}
	if changed("light-level") {
		p["lightlevel"] = sf.lightLevel
	}
	if changed("color-temp") {
		p["colortemp"] = sf.colorTemp
	}
	if changed("rgb-color") {
		c := strings.TrimSpace(sf.rgbColor)
		if !strings.HasPrefix(c, "#") {
			c = "#" + c
		}
		if len(c) != 7 {
			return nil, fmt.Errorf("--rgb-color: expected #RRGGBB, got %q", sf.rgbColor)
		}
		p["rgbcolor"] = c
	}
	if changed("rgb-mode") {
		v := strings.ToLower(sf.rgbMode)
		num := map[string]int{"static": 1, "cycle": 2, "breath": 3}[v]
		if num == 0 {
			return nil, fmt.Errorf("--rgb-mode: expected static|cycle|breath, got %q", sf.rgbMode)
		}
		p["rgbmode"] = num
	}
	if changed("rgb-level") {
		p["rgblevel"] = sf.rgbLevel
	}
	if changed("child-lock") {
		b, err := parseOnOff(sf.childLock)
		if err != nil {
			return nil, fmt.Errorf("--child-lock: %w", err)
		}
		p["childlockon"] = b
	}
	if changed("display") {
		b, err := parseOnOff(sf.display)
		if err != nil {
			return nil, fmt.Errorf("--display: %w", err)
		}
		p["displayon"] = b
	}
	if changed("led-always-on") {
		b, err := parseOnOff(sf.ledAlwaysOn)
		if err != nil {
			return nil, fmt.Errorf("--led-always-on: %w", err)
		}
		p["ledalwayson"] = b
	}
	if changed("voice") {
		b, err := parseOnOff(sf.voice)
		if err != nil {
			return nil, fmt.Errorf("--voice: %w", err)
		}
		p["voiceon"] = b
	}
	if changed("mute") {
		b, err := parseOnOff(sf.mute)
		if err != nil {
			return nil, fmt.Errorf("--mute: %w", err)
		}
		p["muteon"] = b
	}
	return p, nil
}
