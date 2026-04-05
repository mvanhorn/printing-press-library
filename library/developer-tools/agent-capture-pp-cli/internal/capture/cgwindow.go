package capture

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// listWindowsCGWindow uses a Swift script to call CGWindowListCopyWindowInfo.
// Swift is available on all macOS installs with Xcode Command Line Tools and
// has native access to CoreGraphics without any third-party dependencies.
func listWindowsCGWindow(ctx context.Context) ([]Window, error) {
	script := `
import CoreGraphics
import AppKit
import Foundation

struct WinInfo: Codable {
    let id: UInt32
    let title: String
    let app_name: String
    let pid: Int
    let x: Int
    let y: Int
    let width: Int
    let height: Int
    let on_screen: Bool
    let bundle_id: String
}

guard let windowList = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else {
    print("[]")
    exit(0)
}

var results: [WinInfo] = []
for w in windowList {
    let layer = w[kCGWindowLayer as String] as? Int ?? -1
    guard layer == 0 else { continue }

    let bounds = w[kCGWindowBounds as String] as? [String: Any] ?? [:]
    let width = bounds["Width"] as? Int ?? 0
    let height = bounds["Height"] as? Int ?? 0
    guard width > 0 && height > 0 else { continue }

    let pid = w[kCGWindowOwnerPID as String] as? Int ?? 0
    var bundleID = ""
    if pid > 0, let app = NSRunningApplication(processIdentifier: pid_t(pid)) {
        bundleID = app.bundleIdentifier ?? ""
    }

    let info = WinInfo(
        id: w[kCGWindowNumber as String] as? UInt32 ?? 0,
        title: w[kCGWindowName as String] as? String ?? "",
        app_name: w[kCGWindowOwnerName as String] as? String ?? "",
        pid: pid,
        x: bounds["X"] as? Int ?? 0,
        y: bounds["Y"] as? Int ?? 0,
        width: width,
        height: height,
        on_screen: w[kCGWindowIsOnscreen as String] as? Bool ?? false,
        bundle_id: bundleID
    )
    results.append(info)
}

let encoder = JSONEncoder()
if let data = try? encoder.encode(results), let str = String(data: data, encoding: .utf8) {
    print(str)
} else {
    print("[]")
}
`
	cmd := exec.CommandContext(ctx, "swift", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing windows: %w (is Screen Recording permission granted?)", err)
	}

	var windows []Window
	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("parsing window list: %w", err)
	}
	return windows, nil
}

// listDisplaysCG uses a Swift script to enumerate displays via CoreGraphics.
func listDisplaysCG(ctx context.Context) ([]Display, error) {
	script := `
import CoreGraphics
import AppKit
import Foundation

struct DispInfo: Codable {
    let id: UInt32
    let width: Int
    let height: Int
    let is_primary: Bool
    let scale_factor: Double
}

var displayIDs = [CGDirectDisplayID](repeating: 0, count: 16)
var displayCount: UInt32 = 0
CGGetActiveDisplayList(16, &displayIDs, &displayCount)

let mainID = CGMainDisplayID()
var results: [DispInfo] = []

for i in 0..<Int(displayCount) {
    let did = displayIDs[i]
    let bounds = CGDisplayBounds(did)
    var scale = 1.0
    if let mode = CGDisplayCopyDisplayMode(did) {
        let pw = mode.pixelWidth
        let lw = mode.width
        if lw > 0 { scale = Double(pw) / Double(lw) }
    }
    results.append(DispInfo(
        id: did,
        width: Int(bounds.size.width),
        height: Int(bounds.size.height),
        is_primary: did == mainID,
        scale_factor: scale
    ))
}

let encoder = JSONEncoder()
if let data = try? encoder.encode(results), let str = String(data: data, encoding: .utf8) {
    print(str)
} else {
    print("[]")
}
`
	cmd := exec.CommandContext(ctx, "swift", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing displays: %w", err)
	}

	var displays []Display
	if err := json.Unmarshal(out, &displays); err != nil {
		return nil, fmt.Errorf("parsing display list: %w", err)
	}
	return displays, nil
}

// screenshotWithScreencapture uses macOS built-in screencapture command.
func screenshotWithScreencapture(ctx context.Context, target string, output string, opts ScreenshotOptions) error {
	args := []string{"-x"} // silent (no sound)

	if !opts.ShowShadow {
		args = append(args, "-o") // no shadow
	}

	format := opts.Format
	if format == "" {
		format = "png"
	}
	args = append(args, "-t", format)

	// Parse target: --app "Name", --window-id N, --display N, --region x,y,w,h
	if strings.HasPrefix(target, "window:") {
		wid := strings.TrimPrefix(target, "window:")
		args = append(args, "-l", wid)
	} else if strings.HasPrefix(target, "display:") {
		did := strings.TrimPrefix(target, "display:")
		args = append(args, "-D", did)
	} else if strings.HasPrefix(target, "region:") {
		region := strings.TrimPrefix(target, "region:")
		args = append(args, "-R", region)
	} else if strings.HasPrefix(target, "app:") {
		// Find window ID for app name, then use -l
		appName := strings.TrimPrefix(target, "app:")
		wid, err := findWindowIDByApp(ctx, appName)
		if err != nil {
			return err
		}
		args = append(args, "-l", strconv.Itoa(int(wid)))
	}

	args = append(args, output)

	cmd := exec.CommandContext(ctx, "screencapture", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("screencapture failed: %w\n%s", err, string(out))
	}
	return nil
}

// recordWithScreencapture uses macOS screencapture -v for video recording.
func recordWithScreencapture(ctx context.Context, target string, output string, opts RecordOptions) error {
	args := []string{"-v", "-x"} // video mode, silent

	if opts.ShowCursor {
		args = append(args, "-C")
	}

	if opts.Duration > 0 {
		args = append(args, "-V", strconv.Itoa(opts.Duration))
	}

	// Target selection
	if strings.HasPrefix(target, "window:") {
		wid := strings.TrimPrefix(target, "window:")
		args = append(args, "-l", wid)
	} else if strings.HasPrefix(target, "display:") {
		did := strings.TrimPrefix(target, "display:")
		args = append(args, "-D", did)
	} else if strings.HasPrefix(target, "region:") {
		region := strings.TrimPrefix(target, "region:")
		args = append(args, "-R", region)
	} else if strings.HasPrefix(target, "app:") {
		appName := strings.TrimPrefix(target, "app:")
		wid, err := findWindowIDByApp(ctx, appName)
		if err != nil {
			return err
		}
		args = append(args, "-l", strconv.Itoa(int(wid)))
	}

	args = append(args, output)

	cmd := exec.CommandContext(ctx, "screencapture", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("screencapture recording failed: %w\n%s", err, string(out))
	}
	return nil
}

// findWindowIDByApp finds the frontmost window ID for an app by name (fuzzy match).
func findWindowIDByApp(ctx context.Context, appName string) (uint32, error) {
	windows, err := ListWindows(ctx)
	if err != nil {
		return 0, err
	}

	appLower := strings.ToLower(appName)

	// Exact match first
	for _, w := range windows {
		if strings.ToLower(w.AppName) == appLower {
			return w.ID, nil
		}
	}

	// Prefix match
	for _, w := range windows {
		if strings.HasPrefix(strings.ToLower(w.AppName), appLower) {
			return w.ID, nil
		}
	}

	// Contains match
	for _, w := range windows {
		if strings.Contains(strings.ToLower(w.AppName), appLower) {
			return w.ID, nil
		}
	}

	// Build available apps list for error message
	seen := map[string]bool{}
	var apps []string
	for _, w := range windows {
		if !seen[w.AppName] {
			seen[w.AppName] = true
			apps = append(apps, w.AppName)
		}
	}

	return 0, fmt.Errorf("no window found for app %q. Available: %s", appName, strings.Join(apps, ", "))
}
