package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/zoomurl"
)

// newZoomJoinCmd: `zoom-pp-cli join <id-or-url> [--pwd] [--name]`
// Builds a zoommtg:// URL and hands it to the platform-default URL opener.
func newZoomJoinCmd(flags *rootFlags) *cobra.Command {
	var (
		pwd    string
		uname  string
		action string
	)
	cmd := &cobra.Command{
		Use:   "join [meeting-id-or-url]",
		Short: "Join a Zoom meeting via the desktop client URL scheme",
		Long: "Accepts a bare meeting ID, a zoommtg:// scheme URL, or any https://*.zoom.us/j/* / /s/* / /my/* URL. " +
			"Builds the canonical zoommtg:// URL and hands it to `open` (macOS), `xdg-open` (Linux), or `start` (Windows). " +
			"With --dry-run, prints the URL it would launch. Encrypted passwords (Zoom's URL-shaped pwd) are detected and rejected — " +
			"use the raw numeric password instead.",
		Example: `  zoom-pp-cli join 85123456789 --pwd 123456
  zoom-pp-cli join "https://us02web.zoom.us/j/85123456789?pwd=abc" --dry-run --json
  zoom-pp-cli join 85123456789 --name "Maya" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			p, err := zoomurl.Parse(args[0])
			if err != nil {
				return fmt.Errorf("join: %w", err)
			}
			if pwd != "" {
				p.Pwd = pwd
				p.Encrypted = false
			}
			if uname != "" {
				p.Uname = uname
			}
			if action != "" {
				p.Action = zoomurl.Action(action)
			}
			if p.Encrypted {
				return errors.New("join: detected an encrypted_password (zoom web URL form); the desktop URL scheme requires the raw password — pass --pwd <raw-password>")
			}
			url, err := zoomurl.Build(p)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"url":        url,
				"action":     string(p.Action),
				"meeting_id": p.ConfNo,
				"has_pwd":    p.Pwd != "",
				"uname":      p.Uname,
				"platform":   runtime.GOOS,
			}

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				payload["status"] = "would_launch"
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "would launch:", url)
					return nil
				}
				return flags.printJSON(cmd, payload)
			}

			if err := openURL(cmd.Context(), url); err != nil {
				return fmt.Errorf("launching Zoom: %w", err)
			}
			payload["status"] = "launched"
			return flags.printJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVar(&pwd, "pwd", "", "Raw meeting password (overrides any in the URL)")
	cmd.Flags().StringVar(&uname, "name", "", "Display name to use in the meeting")
	cmd.Flags().StringVar(&action, "action", "", "URL scheme action: 'join' (default) or 'start'")
	return cmd
}

// newZoomStartCmd: `zoom-pp-cli start [meeting-id]`. With no arg, opens Zoom
// app (which lands on the home screen ready to start a meeting). With an arg,
// builds a start URL.
func newZoomStartCmd(flags *rootFlags) *cobra.Command {
	var (
		instant bool
		uname   string
		zak     string
		uid     string
	)
	cmd := &cobra.Command{
		Use:   "start [meeting-id]",
		Short: "Start a Zoom meeting (instant, personal-room, or by ID)",
		Long: "With no argument, launches the Zoom desktop app's home screen so you can click Start. " +
			"--instant uses the zoommtg://zoom.us/start?action=start URL (starts your personal meeting room). " +
			"With a <meeting-id>, builds a start URL targeting that meeting. ZAK token via --zak (or fetched " +
			"separately via the cloud `users token` command). Encrypted passwords cannot be used here.",
		Example: `  zoom-pp-cli start --json --dry-run
  zoom-pp-cli start --instant --name "Maya"
  zoom-pp-cli start 85123456789 --zak <token>`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) && len(args) == 0 && !instant {
				return flags.printJSON(cmd, map[string]any{
					"status":   "would_open_zoom_home",
					"platform": runtime.GOOS,
				})
			}
			if cliutil.IsVerifyEnv() && len(args) == 0 && !instant {
				fmt.Fprintln(cmd.OutOrStdout(), "would open Zoom home screen")
				return nil
			}
			// No args, no --instant: just launch the app.
			if len(args) == 0 && !instant {
				if err := openURL(cmd.Context(), "zoommtg://zoom.us"); err != nil {
					return fmt.Errorf("launching Zoom: %w", err)
				}
				return flags.printJSON(cmd, map[string]any{
					"status":   "launched_home",
					"platform": runtime.GOOS,
				})
			}
			p := zoomurl.Params{Action: zoomurl.ActionStart, Uname: uname, ZakToken: zak, UID: uid}
			if instant {
				// Instant meeting endpoint requires no confno; the Zoom client
				// interprets a missing confno as "create instant meeting". We
				// pass a placeholder of "0" so the URL builder is happy and
				// strip it client-side. Better: use the bare zoommtg://zoom.us URL.
				if err := launchOrDry(cmd, flags, "zoommtg://zoom.us/start?action=start", "instant"); err != nil {
					return err
				}
				return nil
			}
			p.ConfNo = args[0]
			// Reject obvious placeholder / non-meeting-ID values so dogfood's
			// error_path probes return an actionable typed error rather than
			// pretending to launch.
			if !looksLikeMeetingID(p.ConfNo) {
				return fmt.Errorf("start: %q does not look like a Zoom meeting ID (expected 9-12 digits)", p.ConfNo)
			}
			url, err := zoomurl.Build(p)
			if err != nil {
				return err
			}
			return launchOrDry(cmd, flags, url, "start")
		},
	}
	cmd.Flags().BoolVar(&instant, "instant", false, "Start an instant meeting in your personal room")
	cmd.Flags().StringVar(&uname, "name", "", "Display name")
	cmd.Flags().StringVar(&zak, "zak", "", "ZAK token (fetched via cloud users token endpoint)")
	cmd.Flags().StringVar(&uid, "uid", "", "Host user ID (for action=start)")
	return cmd
}

func launchOrDry(cmd *cobra.Command, flags *rootFlags, url, label string) error {
	payload := map[string]any{
		"url":      url,
		"action":   label,
		"platform": runtime.GOOS,
	}
	if dryRunOK(flags) || cliutil.IsVerifyEnv() {
		payload["status"] = "would_launch"
		if !flags.asJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "would launch:", url)
			return nil
		}
		return flags.printJSON(cmd, payload)
	}
	if err := openURL(cmd.Context(), url); err != nil {
		return fmt.Errorf("launching Zoom: %w", err)
	}
	payload["status"] = "launched"
	return flags.printJSON(cmd, payload)
}

// looksLikeMeetingID accepts strings of 9-12 digits (after stripping
// whitespace and dashes). Used to reject test placeholders.
func looksLikeMeetingID(s string) bool {
	stripped := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "-", "")
	if len(stripped) < 9 || len(stripped) > 12 {
		return false
	}
	for _, r := range stripped {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// openURL hands a URL to the platform default opener.
func openURL(ctx context.Context, url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", "", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", cmd.Path, strings.TrimSpace(string(out)), err)
	}
	return nil
}
