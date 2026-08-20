package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
)

// newPrefsCmd builds the prefs subtree. set-speed and set-mode are generated
// commands; this subtree only covers belt configuration preferences that have
// no generated equivalent.
func newPrefsCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	prefs := &cobra.Command{
		Use:         "prefs",
		Annotations: map[string]string{"mcp:hidden": "true"}, // configuration-risk; hides the whole prefs subtree from MCP
		Short:       "Configure belt preferences (max speed, sensitivity, units, ...)",
		Long:        "Set belt preferences. Each subcommand is a configuration-risk write: requires --live and --confirm-physical-effect (or --dry-run to preview).",
	}
	prefs.PersistentFlags().BoolVar(&confirm, "confirm-physical-effect", false, "Confirm a configuration-risk write")

	speedPref := func(use, short, action string, key byte) *cobra.Command {
		return &cobra.Command{
			Use:     use,
			Short:   short,
			Example: "  walkingpad-pp-cli prefs " + use + " --live --confirm-physical-effect",
			RunE: func(cmd *cobra.Command, args []string) error {
				if done, err := verifyNoop(cmd, flags, "prefs"); done {
					return err
				}
				if len(args) == 0 {
					return cmd.Help()
				}
				kmh, err := strconv.ParseFloat(args[0], 64)
				if err != nil || kmh < wpble.MinSpeedKmh || kmh > wpble.MaxSpeedKmh {
					return fmt.Errorf("%s must be %.1f-%.1f km/h", action, wpble.MinSpeedKmh, wpble.MaxSpeedKmh)
				}
				payload := fmt.Sprintf("%x", wpble.CmdSetPref(key, int(kmh*10+0.5), 0))
				return gateWrite(cmd, flags, fmt.Sprintf("%s to %.1f km/h", action, kmh), "configuration-risk", payload, confirm)
			},
		}
	}
	prefs.AddCommand(speedPref("max-speed [kmh]", "Set the maximum allowed belt speed", "set max speed", wpble.PrefMaxSpeed))
	prefs.AddCommand(speedPref("start-speed [kmh]", "Set the belt's start speed", "set start speed", wpble.PrefStartSpeed))

	togglePref := func(use, short, action string, key byte) *cobra.Command {
		return &cobra.Command{
			Use:     use,
			Short:   short,
			Example: "  walkingpad-pp-cli prefs " + use + " --live --confirm-physical-effect",
			RunE: func(cmd *cobra.Command, args []string) error {
				if done, err := verifyNoop(cmd, flags, "prefs"); done {
					return err
				}
				if len(args) == 0 {
					return cmd.Help()
				}
				val, err := parseOnOff(args[0])
				if err != nil {
					return fmt.Errorf("%s: %w", action, err)
				}
				payload := fmt.Sprintf("%x", wpble.CmdSetPref(key, val, 0))
				return gateWrite(cmd, flags, fmt.Sprintf("%s %s", action, args[0]), "configuration-risk", payload, confirm)
			},
		}
	}
	prefs.AddCommand(togglePref("child-lock [on|off]", "Enable or disable the child lock", "set child-lock", wpble.PrefChildLock))
	prefs.AddCommand(togglePref("auto-start [on|off]", "Enable or disable intelligent auto-start", "set auto-start", wpble.PrefStartIntel))

	prefs.AddCommand(&cobra.Command{
		Use:     "sensitivity [high|medium|low]",
		Short:   "Set auto-mode sensitivity",
		Example: "  walkingpad-pp-cli prefs sensitivity medium --live --confirm-physical-effect",
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyNoop(cmd, flags, "prefs"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			level, ok := map[string]int{"high": 1, "medium": 2, "low": 3}[args[0]]
			if !ok {
				return fmt.Errorf("sensitivity must be high, medium, or low")
			}
			payload := fmt.Sprintf("%x", wpble.CmdSetPref(wpble.PrefSensitivity, level, 0))
			return gateWrite(cmd, flags, "set sensitivity to "+args[0], "configuration-risk", payload, confirm)
		},
	})

	prefs.AddCommand(&cobra.Command{
		Use:     "units [km|miles]",
		Short:   "Set the belt display units",
		Example: "  walkingpad-pp-cli prefs units km --live --confirm-physical-effect",
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyNoop(cmd, flags, "prefs"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			val, ok := map[string]int{"km": 0, "miles": 1}[args[0]]
			if !ok {
				return fmt.Errorf("units must be km or miles")
			}
			payload := fmt.Sprintf("%x", wpble.CmdSetPref(wpble.PrefUnits, val, 0))
			return gateWrite(cmd, flags, "set units to "+args[0], "configuration-risk", payload, confirm)
		},
	})
	return prefs
}

func parseOnOff(s string) (int, error) {
	switch s {
	case "on", "true", "1", "yes":
		return 1, nil
	case "off", "false", "0", "no":
		return 0, nil
	default:
		return 0, fmt.Errorf("want on or off, got %q", s)
	}
}
