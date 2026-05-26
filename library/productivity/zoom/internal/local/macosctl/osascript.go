// Package macosctl drives the macOS Zoom desktop app via osascript. The
// scripts target the running "zoom.us" process's Meeting menu items, which is
// the same surface the henrik/Stream-Deck/Alfred community gists have used
// since 2019. All commands no-op safely when Zoom is not running or not in a
// meeting; the helper detects those states without raising errors.
package macosctl

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// IsSupported returns true on macOS where osascript is available.
func IsSupported() bool {
	return runtime.GOOS == "darwin"
}

// ErrUnsupported is returned by commands invoked on non-macOS hosts.
type ErrUnsupported struct{ GOOS string }

func (e *ErrUnsupported) Error() string {
	return fmt.Sprintf("macosctl: osascript not supported on %s — Zoom desktop control is macOS-only", e.GOOS)
}

// guard ensures every entrypoint refuses to run off-platform with a typed
// error the cobra RunE can wrap.
func guard() error {
	if !IsSupported() {
		return &ErrUnsupported{GOOS: runtime.GOOS}
	}
	return nil
}

// Status captures everything the in-meeting probe can answer in a single
// AppleScript call. The probe relies on which menu items the Zoom Meeting
// menu currently exposes — "Mute audio" appears only when the user is
// unmuted; "Unmute audio" only when muted; etc.
type Status struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	InMeeting bool   `json:"in_meeting"`
	Muted     *bool  `json:"muted,omitempty"`    // nil when not in a meeting
	VideoOn   *bool  `json:"video_on,omitempty"` // nil when not in a meeting
	Topic     string `json:"topic,omitempty"`
}

// CheckStatus runs a single AppleScript that combines all probes. The script
// returns a fixed-format string we parse rather than calling osascript four
// times.
func CheckStatus(ctx context.Context) (Status, error) {
	st := Status{}
	if err := guard(); err != nil {
		return st, err
	}

	// installed?
	out, _ := runOsascript(ctx, `tell application "System Events" to return (exists application process "zoom.us") as string`)
	st.Running = strings.TrimSpace(out) == "true"

	// installation independent of process state
	out2, _ := runOsascript(ctx, `try
    tell application "Finder" to return (POSIX path of (file "Applications:zoom.us.app" of (path to applications folder from system domain)) as string)
on error
    return ""
end try`)
	st.Installed = strings.TrimSpace(out2) != ""

	if !st.Running {
		return st, nil
	}

	probe := `tell application "System Events"
    if not (exists process "zoom.us") then
        return "RUNNING=false"
    end if
    tell process "zoom.us"
        if not (exists menu bar item "Meeting" of menu bar 1) then
            return "RUNNING=true|MEETING=false"
        end if
        set meetingMenu to menu 1 of menu bar item "Meeting" of menu bar 1
        set canMute to (exists menu item "Mute audio" of meetingMenu)
        set canUnmute to (exists menu item "Unmute audio" of meetingMenu)
        set canStartVideo to (exists menu item "Start video" of meetingMenu)
        set canStopVideo to (exists menu item "Stop video" of meetingMenu)
        set topicStr to ""
        try
            set topicStr to (name of window 1)
        end try
        return "RUNNING=true|MEETING=true|MUTE=" & (canMute as string) & "|UNMUTE=" & (canUnmute as string) & "|STARTVIDEO=" & (canStartVideo as string) & "|STOPVIDEO=" & (canStopVideo as string) & "|TOPIC=" & topicStr
    end tell
end tell`
	res, err := runOsascript(ctx, probe)
	if err != nil {
		// We can run scripts but the process probe failed — best-effort
		// fall back to "running but unknown meeting state".
		return st, nil
	}
	st.InMeeting = strings.Contains(res, "MEETING=true")
	if st.InMeeting {
		muted := strings.Contains(res, "UNMUTE=true")    // Unmute item present ⇒ currently muted
		video := strings.Contains(res, "STOPVIDEO=true") // Stop video item present ⇒ video currently on
		st.Muted = &muted
		st.VideoOn = &video
		if i := strings.Index(res, "TOPIC="); i >= 0 {
			st.Topic = strings.TrimSpace(res[i+len("TOPIC="):])
		}
	}
	return st, nil
}

// Mute clicks "Mute audio" if the menu item is present; no-op if already muted
// or not in a meeting. Returns whether the click actually fired.
func Mute(ctx context.Context) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	return clickMeetingMenu(ctx, "Mute audio")
}

// Unmute clicks "Unmute audio" if the menu item is present.
func Unmute(ctx context.Context) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	return clickMeetingMenu(ctx, "Unmute audio")
}

// MuteToggle clicks whichever of Mute/Unmute is currently available.
func MuteToggle(ctx context.Context) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	for _, item := range []string{"Unmute audio", "Mute audio"} {
		fired, err := clickMeetingMenu(ctx, item)
		if err == nil && fired {
			return true, nil
		}
	}
	return false, nil
}

// StartVideo turns the camera on (clicks "Start video").
func StartVideo(ctx context.Context) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	return clickMeetingMenu(ctx, "Start video")
}

// StopVideo turns the camera off (clicks "Stop video").
func StopVideo(ctx context.Context) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	return clickMeetingMenu(ctx, "Stop video")
}

// VideoToggle clicks whichever of Start/Stop video is currently available.
func VideoToggle(ctx context.Context) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	for _, item := range []string{"Stop video", "Start video"} {
		fired, err := clickMeetingMenu(ctx, item)
		if err == nil && fired {
			return true, nil
		}
	}
	return false, nil
}

// Leave closes the current meeting window. When endForAll is true and the user
// is the host, click the "End Meeting for All" button on the confirmation
// dialog; otherwise click "Leave Meeting".
func Leave(ctx context.Context, endForAll bool) (bool, error) {
	if err := guard(); err != nil {
		return false, err
	}
	// First try the Meeting → End/Close item directly; fall back to a Cmd+W
	// keystroke which always triggers the leave dialog.
	leave := `tell application "System Events"
    if not (exists process "zoom.us") then return "false"
    tell process "zoom.us"
        if not (exists menu bar item "Meeting" of menu bar 1) then return "false"
        try
            set menuRef to menu 1 of menu bar item "Meeting" of menu bar 1
            if (exists menu item "End Meeting" of menuRef) then
                click menu item "End Meeting" of menuRef
            else if (exists menu item "Leave Meeting" of menuRef) then
                click menu item "Leave Meeting" of menuRef
            else
                keystroke "w" using command down
            end if
        on error
            keystroke "w" using command down
        end try
    end tell
end tell
return "true"`
	out, err := runOsascript(ctx, leave)
	if err != nil {
		return false, err
	}
	fired := strings.Contains(out, "true")

	// Confirmation dialog handling.
	if fired {
		time.Sleep(200 * time.Millisecond)
		btn := "Leave Meeting"
		if endForAll {
			btn = "End Meeting for All"
		}
		confirm := fmt.Sprintf(`tell application "System Events"
    tell process "zoom.us"
        try
            click button %q of window 1
        end try
    end tell
end tell`, btn)
		_, _ = runOsascript(ctx, confirm)
	}
	return fired, nil
}

// AppleScriptFor returns the AppleScript text a given command would run, for
// --dry-run output.
func AppleScriptFor(action string, args ...string) string {
	switch action {
	case "mute":
		return `tell process "zoom.us" → click menu item "Mute audio" of Meeting menu`
	case "unmute":
		return `tell process "zoom.us" → click menu item "Unmute audio" of Meeting menu`
	case "video-on":
		return `tell process "zoom.us" → click menu item "Start video" of Meeting menu`
	case "video-off":
		return `tell process "zoom.us" → click menu item "Stop video" of Meeting menu`
	case "leave":
		return `tell process "zoom.us" → click menu item "Leave Meeting" of Meeting menu (or Cmd+W fallback)`
	case "status":
		return `tell process "zoom.us" → probe Meeting menu for Mute/Unmute, Start/Stop video, window topic`
	}
	return ""
}

// clickMeetingMenu is a small AppleScript that clicks one item of the Zoom
// Meeting menu if present; returns ok=true when the menu item existed and was
// clicked, ok=false when the item was not present (already in that state, or
// not in a meeting), and a non-nil error only on osascript failures.
func clickMeetingMenu(ctx context.Context, item string) (bool, error) {
	script := fmt.Sprintf(`tell application "System Events"
    if not (exists process "zoom.us") then return "false"
    tell process "zoom.us"
        if not (exists menu bar item "Meeting" of menu bar 1) then return "false"
        set menuRef to menu 1 of menu bar item "Meeting" of menu bar 1
        if (exists menu item %q of menuRef) then
            click menu item %q of menuRef
            return "true"
        end if
        return "false"
    end tell
end tell`, item, item)
	out, err := runOsascript(ctx, script)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

func runOsascript(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		// osascript writes useful errors to stderr; preserve them in the
		// wrapped message.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("osascript: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return "", fmt.Errorf("osascript: %w", err)
	}
	return string(out), nil
}
