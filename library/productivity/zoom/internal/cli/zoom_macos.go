package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/macosctl"
)

// newZoomMuteCmd: `zoom-pp-cli mute [toggle]`
func newZoomMuteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mute [toggle]",
		Short: "Mute the running Zoom meeting (macOS)",
		Long: "Clicks 'Mute audio' in the running Zoom app's Meeting menu via osascript. " +
			"Pass 'toggle' to flip whichever of Mute/Unmute is currently available. " +
			"Safe no-op when Zoom is not running or not in a meeting. macOS only.",
		Example: `  zoom-pp-cli mute --json
  zoom-pp-cli mute toggle --json
  zoom-pp-cli mute --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "mute"
			if len(args) > 0 && args[0] == "toggle" {
				action = "mute-toggle"
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printOsascriptPreview(cmd, flags, action)
			}
			if !macosctl.IsSupported() {
				return &macosctl.ErrUnsupported{GOOS: "non-darwin"}
			}
			var (
				fired bool
				err   error
			)
			if action == "mute-toggle" {
				fired, err = macosctl.MuteToggle(cmd.Context())
			} else {
				fired, err = macosctl.Mute(cmd.Context())
			}
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{
				"action": action,
				"fired":  fired,
				"note":   muteNote(fired),
			})
		},
	}
	return cmd
}

func newZoomUnmuteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "unmute",
		Short: "Unmute the running Zoom meeting (macOS)",
		Example: `  zoom-pp-cli unmute --json
  zoom-pp-cli unmute --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printOsascriptPreview(cmd, flags, "unmute")
			}
			if !macosctl.IsSupported() {
				return &macosctl.ErrUnsupported{GOOS: "non-darwin"}
			}
			fired, err := macosctl.Unmute(cmd.Context())
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"action": "unmute", "fired": fired, "note": muteNote(fired)})
		},
	}
}

func newZoomVideoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "video <on|off|toggle>",
		Short: "Control the meeting camera (macOS)",
		Long:  "Clicks Start/Stop video in the running Zoom app's Meeting menu via osascript. macOS only.",
		Example: `  zoom-pp-cli video on --json
  zoom-pp-cli video off --json
  zoom-pp-cli video toggle --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			action := args[0]
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printOsascriptPreview(cmd, flags, "video-"+action)
			}
			if !macosctl.IsSupported() {
				return &macosctl.ErrUnsupported{GOOS: "non-darwin"}
			}
			var (
				fired bool
				err   error
			)
			switch action {
			case "on":
				fired, err = macosctl.StartVideo(cmd.Context())
			case "off":
				fired, err = macosctl.StopVideo(cmd.Context())
			case "toggle":
				fired, err = macosctl.VideoToggle(cmd.Context())
			default:
				return fmt.Errorf("video: action must be on|off|toggle, got %q", action)
			}
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"action": "video-" + action, "fired": fired, "note": videoNote(fired)})
		},
	}
	return cmd
}

func newZoomLeaveCmd(flags *rootFlags) *cobra.Command {
	var endForAll bool
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave the current Zoom meeting (macOS)",
		Long:  "Closes the meeting window via osascript. With --end, clicks 'End Meeting for All' (host only). macOS only.",
		Example: `  zoom-pp-cli leave --json
  zoom-pp-cli leave --end --json
  zoom-pp-cli leave --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printOsascriptPreview(cmd, flags, "leave")
			}
			if !macosctl.IsSupported() {
				return &macosctl.ErrUnsupported{GOOS: "non-darwin"}
			}
			fired, err := macosctl.Leave(cmd.Context(), endForAll)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"action": "leave", "fired": fired, "end_for_all": endForAll})
		},
	}
	cmd.Flags().BoolVar(&endForAll, "end", false, "End Meeting for All (host only)")
	return cmd
}

func newZoomStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report whether Zoom is installed, running, in a meeting, muted, video on, etc. (macOS)",
		Example: `  zoom-pp-cli status --json
  zoom-pp-cli status --select running,in_meeting,muted --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printOsascriptPreview(cmd, flags, "status")
			}
			if !macosctl.IsSupported() {
				// Still return a structured response so agents can probe on
				// any platform without an error.
				return flags.printJSON(cmd, map[string]any{
					"supported": false,
					"goos":      "non-darwin",
					"note":      "Zoom desktop control is macOS-only; status probe unavailable",
				})
			}
			st, err := macosctl.CheckStatus(cmd.Context())
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{
				"supported":  true,
				"installed":  st.Installed,
				"running":    st.Running,
				"in_meeting": st.InMeeting,
				"muted":      st.Muted,
				"video_on":   st.VideoOn,
				"topic":      st.Topic,
			})
		},
	}
}

func muteNote(fired bool) string {
	if fired {
		return "audio state toggled"
	}
	return "no menu item available — Zoom not running or not in a meeting"
}

func videoNote(fired bool) string {
	if fired {
		return "video state toggled"
	}
	return "no menu item available — Zoom not running or not in a meeting"
}

func printOsascriptPreview(cmd *cobra.Command, flags *rootFlags, action string) error {
	preview := macosctl.AppleScriptFor(action)
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{
			"action":      action,
			"status":      "would_run_osascript",
			"applescript": preview,
		})
	}
	fmt.Fprintln(cmd.OutOrStdout(), "would run osascript:", preview)
	return nil
}
