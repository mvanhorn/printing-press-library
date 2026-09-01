// PATCH(library): semantic KVMD control surface preserved from Python kvmctl.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/results"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/switcher"
	"github.com/spf13/cobra"
)

// pp:data-source live
func init() { registerNovelCommand(registerKVMDCommands) }

func registerKVMDCommands(root *cobra.Command, flags *rootFlags) {
	addNovelCommandIfAbsent(root, newKVMDStatusCmd(flags))
	addNovelCommandIfAbsent(root, newKVMDCapabilitiesCmd(flags))
	addNovelCommandIfAbsent(root, newKVMDScreenshotCmd(flags))
	addNovelCommandIfAbsent(root, newKeyboardCmd(flags))
	addNovelCommandIfAbsent(root, newMouseCmd(flags))
	addNovelCommandIfAbsent(root, newTargetSwitchCmd(flags))
	if parent, _, err := root.Find([]string{"kvmd-compatible-kvm-auth"}); err == nil && parent != nil {
		addNovelCommandIfAbsent(parent, newKVMDLoginCmd(flags))
	}
}

func kvmdJSON(flags *rootFlags, cmd *cobra.Command, v any) error {
	if flags.asJSON || flags.agent {
		return flags.printJSON(cmd, v)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), v)
	return err
}
func kvmdClient(flags *rootFlags) (*client.Client, error) { return flags.newClient() }

func newKVMDStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show KVMD authentication and device status", Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		info, err := c.Get(cmd.Context(), "/api/info", nil)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		return kvmdJSON(flags, cmd, map[string]any{"info": rawJSON(info), "endpoint": c.RequestBaseURL()})
	}}
}
func newKVMDCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "capabilities", Short: "Discover KVMD capabilities", Example: "  kvmctl-pp-cli capabilities\n  kvmctl-pp-cli capabilities --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		caps, err := c.KVMDCapabilities(cmd.Context())
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		return kvmdJSON(flags, cmd, caps)
	}}
}
func rawJSON(b []byte) any { return json.RawMessage(b) }

func newKVMDScreenshotCmd(flags *rootFlags) *cobra.Command {
	var out string
	cmd := &cobra.Command{Use: "screenshot", Short: "Capture a KVMD JPEG screenshot", Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		if flags.dryRun {
			return kvmdJSON(flags, cmd, map[string]any{"dry_run": true, "path": "/api/streamer/snapshot"})
		}
		data, err := c.GetWithHeaders(cmd.Context(), "/api/streamer/snapshot", map[string]string{"preview": "true"}, map[string]string{"Accept": "image/jpeg", "X-Printing-Press-Binary-Response": "true"})
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		if out == "" {
			out = "-"
		}
		var w io.Writer = cmd.OutOrStdout()
		var f *os.File
		if out != "-" {
			f, err = os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
			w = f
		}
		_, err = w.Write(data)
		return err
	}}
	cmd.Flags().StringVar(&out, "output", "-", "JPEG output path, or - for stdout")
	return cmd
}

func newKVMDLoginCmd(flags *rootFlags) *cobra.Command {
	var user, password string
	cmd := &cobra.Command{Use: "login", Aliases: []string{"create"}, Short: "Login to KVMD and persist the token", Annotations: map[string]string{"pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if user == "" || password == "" {
			return usageErr(fmt.Errorf("--user and --passwd are required"))
		}
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return err
		}
		c := client.New(cfg, flags.timeout, flags.rateLimit)
		if flags.dryRun {
			return kvmdJSON(flags, cmd, map[string]any{"dry_run": true, "path": "/api/auth/login", "user": user})
		}
		tok, err := c.KVMDLogin(cmd.Context(), user, password)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		if err := cfg.SaveCredential(tok); err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"authenticated": true})
	}}
	cmd.Flags().StringVar(&user, "user", "", "KVMD username")
	cmd.Flags().StringVar(&password, "passwd", "", "KVMD password")
	return cmd
}

func newKeyboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "keyboard", Short: "Control KVMD keyboard", Annotations: map[string]string{"pp:novel": "true"}}
	var key string
	press := &cobra.Command{Use: "key", Short: "Press or release a key", RunE: func(cmd *cobra.Command, args []string) error {
		if key == "" {
			return usageErr(fmt.Errorf("--key is required"))
		}
		c, e := kvmdClient(flags)
		if e != nil {
			return e
		}
		if flags.dryRun {
			return kvmdJSON(flags, cmd, map[string]any{"dry_run": true, "key": key})
		}
		if e = c.KVMDKey(cmd.Context(), key, true); e != nil {
			return e
		}
		e = c.KVMDKey(cmd.Context(), key, false)
		if e != nil {
			return e
		}
		return kvmdJSON(flags, cmd, map[string]any{"key": key, "pressed": true})
	}}
	press.Flags().StringVar(&key, "key", "", "browser key name")
	cmd.AddCommand(press)
	var text string
	typec := &cobra.Command{Use: "text", Short: "Type text using browser key names", RunE: func(cmd *cobra.Command, args []string) error {
		if text == "" {
			return usageErr(fmt.Errorf("--text is required"))
		}
		c, e := kvmdClient(flags)
		if e != nil {
			return e
		}
		sent := 0
		for _, r := range text {
			k, shift, ok := asciiKey(r)
			if !ok {
				return fmt.Errorf("unsupported character %q", r)
			}
			if !flags.dryRun && shift {
				if e = c.KVMDKey(cmd.Context(), "ShiftLeft", true); e != nil {
					return e
				}
			}
			if !flags.dryRun {
				if e = c.KVMDKey(cmd.Context(), k, true); e != nil {
					return e
				}
				if e = c.KVMDKey(cmd.Context(), k, false); e != nil {
					return e
				}
				if shift {
					if e = c.KVMDKey(cmd.Context(), "ShiftLeft", false); e != nil {
						return e
					}
				}
			}
			sent++
		}
		return kvmdJSON(flags, cmd, map[string]any{"chars": sent})
	}}
	typec.Flags().StringVar(&text, "text", "", "text to type")
	cmd.AddCommand(typec)
	var keys string
	chord := &cobra.Command{Use: "chord", Short: "Send a KVMD shortcut", RunE: func(cmd *cobra.Command, args []string) error {
		c, e := kvmdClient(flags)
		if e != nil {
			return e
		}
		if keys == "" {
			return usageErr(fmt.Errorf("--keys is required"))
		}
		if flags.dryRun {
			return kvmdJSON(flags, cmd, map[string]any{"dry_run": true, "keys": keys})
		}
		e = c.KVMDShortcut(cmd.Context(), keys)
		if e != nil {
			return e
		}
		return kvmdJSON(flags, cmd, map[string]any{"keys": keys})
	}}
	chord.Flags().StringVar(&keys, "keys", "", "comma-separated key names")
	cmd.AddCommand(chord)
	var holdKey string
	hold := &cobra.Command{Use: "hold", Short: "Hold a key briefly", RunE: func(cmd *cobra.Command, args []string) error {
		if holdKey == "" {
			return usageErr(fmt.Errorf("--key is required"))
		}
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		if !flags.dryRun {
			if err = c.KVMDKey(cmd.Context(), holdKey, true); err != nil {
				return err
			}
			time.Sleep(120 * time.Millisecond)
			if err = c.KVMDKey(cmd.Context(), holdKey, false); err != nil {
				return err
			}
		}
		return kvmdJSON(flags, cmd, map[string]any{"key": holdKey, "held_ms": 120})
	}}
	hold.Flags().StringVar(&holdKey, "key", "", "browser key name")
	cmd.AddCommand(hold)
	var releaseKeys string
	release := &cobra.Command{Use: "release-all", Short: "Release listed held keys", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(releaseKeys) == "" {
			return usageErr(fmt.Errorf("--keys is required (the CLI cannot observe keys held by another process)"))
		}
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		released := 0
		for _, k := range strings.Split(releaseKeys, ",") {
			if strings.TrimSpace(k) == "" {
				return usageErr(fmt.Errorf("--keys contains an empty key"))
			}
			if !flags.dryRun {
				if err = c.KVMDKey(cmd.Context(), strings.TrimSpace(k), false); err != nil {
					return err
				}
			}
			released++
		}
		return kvmdJSON(flags, cmd, map[string]any{"released": released})
	}}
	release.Flags().StringVar(&releaseKeys, "keys", "", "comma-separated keys to release")
	cmd.AddCommand(release)
	return cmd
}
func asciiKey(r rune) (string, bool, bool) {
	if r >= 'a' && r <= 'z' {
		return "Key" + strings.ToUpper(string(r)), false, true
	}
	if r >= 'A' && r <= 'Z' {
		return "Key" + string(r), true, true
	}
	if r >= '0' && r <= '9' {
		return "Digit" + string(r), false, true
	}
	if r == ' ' {
		return "Space", false, true
	}
	return "", false, false
}

func newMouseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "mouse", Short: "Control KVMD mouse", Annotations: map[string]string{"pp:novel": "true"}}
	var x, y int
	move := &cobra.Command{Use: "move", Short: "Move mouse", RunE: func(cmd *cobra.Command, args []string) error {
		c, e := kvmdClient(flags)
		if e != nil {
			return e
		}
		if !flags.dryRun {
			e = c.KVMDMouseMove(cmd.Context(), x, y)
			if e != nil {
				return e
			}
		}
		return kvmdJSON(flags, cmd, map[string]any{"x": x, "y": y})
	}}
	move.Flags().IntVar(&x, "x", 0, "normalized x (-32768..32767)")
	move.Flags().IntVar(&y, "y", 0, "normalized y (-32768..32767)")
	cmd.AddCommand(move)
	var button string
	var state bool
	buttonCmd := &cobra.Command{Use: "button", Short: "Set mouse button state", RunE: func(cmd *cobra.Command, args []string) error {
		c, e := kvmdClient(flags)
		if e != nil {
			return e
		}
		if !flags.dryRun {
			e = c.KVMDMouseButton(cmd.Context(), button, state)
			if e != nil {
				return e
			}
		}
		return kvmdJSON(flags, cmd, map[string]any{"button": button, "state": state})
	}}
	buttonCmd.Flags().StringVar(&button, "button", "", "left, middle, right, up, or down")
	buttonCmd.Flags().BoolVar(&state, "state", true, "pressed state")
	cmd.AddCommand(buttonCmd)
	var dx, dy int
	wheel := &cobra.Command{Use: "wheel", Short: "Scroll mouse wheel", RunE: func(cmd *cobra.Command, args []string) error {
		c, e := kvmdClient(flags)
		if e != nil {
			return e
		}
		if !flags.dryRun {
			e = c.KVMDMouseWheel(cmd.Context(), dx, dy)
			if e != nil {
				return e
			}
		}
		return kvmdJSON(flags, cmd, map[string]any{"delta_x": dx, "delta_y": dy})
	}}
	wheel.Flags().IntVar(&dx, "dx", 0, "horizontal delta (-127..127)")
	wheel.Flags().IntVar(&dy, "dy", 0, "vertical delta (-127..127)")
	cmd.AddCommand(wheel)
	return cmd
}

func newTargetSwitchCmd(flags *rootFlags) *cobra.Command {
	var port int
	var yes bool
	cmd := &cobra.Command{Use: "target-switch", Short: "Switch a sequential TH41-3 KVM target", Annotations: map[string]string{"pp:novel": "true", "mcp:destructive": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		planned, err := switcher.Plan(switcher.TH413, port)
		if err != nil {
			return usageErr(err)
		}
		if !flags.dryRun && !yes {
			return fmt.Errorf("target switching sends physical key events; re-run with --yes")
		}
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		if !flags.dryRun {
			for i, ev := range planned {
				if err = c.KVMDKey(cmd.Context(), ev.Key, ev.State == "down"); err != nil {
					return err
				}
				if i+1 < len(planned) {
					time.Sleep(switcher.TH413.InterKeyDelay)
				}
			}
			time.Sleep(switcher.TH413.SettleDelay)
		}
		events := make([]map[string]string, 0, len(planned))
		for _, ev := range planned {
			events = append(events, map[string]string{"key": ev.Key, "state": ev.State})
		}
		out := results.Build("target-switch", "kvm", false, "", true, !flags.dryRun, "completed", map[string]any{"profile": switcher.TH413.Name, "port": port, "events": events}, nil)
		return kvmdJSON(flags, cmd, out)
	}}
	cmd.Flags().IntVar(&port, "port", 0, "target port (1..4)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm physical key events")
	return cmd
}
