// Package capture wraps ScreenCaptureKit via tfsoares/screencapturekit-go
// for window/display enumeration, screenshots, and recording.
//
// The screencapturekit-go library uses a subprocess bridge pattern:
// it compiles a Swift CLI binary and shells out to it from Go.
// This avoids writing custom cgo+ObjC bridge code while providing
// full ScreenCaptureKit access.
package capture

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// Window represents a macOS window target.
type Window struct {
	ID       uint32 `json:"id"`
	Title    string `json:"title"`
	AppName  string `json:"app_name"`
	BundleID string `json:"bundle_id"`
	PID      int    `json:"pid"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	OnScreen bool   `json:"on_screen"`
}

// Display represents a macOS display target.
type Display struct {
	ID        uint32  `json:"id"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	IsPrimary bool    `json:"is_primary"`
	Scale     float64 `json:"scale_factor"`
}

// RecordOptions configures a recording session.
type RecordOptions struct {
	Duration   int    // seconds
	FPS        int    // frames per second
	Format     string // mp4, mov
	ShowCursor bool
	Codec      string // h264, hevc
	Quality    string // low, medium, high
}

// ScreenshotOptions configures a screenshot capture.
type ScreenshotOptions struct {
	Format     string // png, jpg
	Retina     bool
	ShowShadow bool
}

// CheckPlatform returns an error if not running on macOS.
func CheckPlatform() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("agent-capture requires macOS (ScreenCaptureKit). Current platform: %s", runtime.GOOS)
	}
	return nil
}

// CheckPermissions verifies Screen Recording permission is granted.
func CheckPermissions(ctx context.Context) error {
	if err := CheckPlatform(); err != nil {
		return err
	}
	// Attempt to list shareable content - this will fail without Screen Recording permission
	_, err := ListWindows(ctx)
	if err != nil {
		return fmt.Errorf("screen Recording permission not granted. Go to System Settings > Privacy & Security > Screen Recording and enable your terminal app")
	}
	return nil
}

// CheckFFmpeg returns true if ffmpeg is available on PATH.
func CheckFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// ListWindows returns all visible macOS windows.
func ListWindows(ctx context.Context) ([]Window, error) {
	if err := CheckPlatform(); err != nil {
		return nil, err
	}
	return listWindowsCGWindow(ctx)
}

// ListDisplays returns all connected displays.
func ListDisplays(ctx context.Context) ([]Display, error) {
	if err := CheckPlatform(); err != nil {
		return nil, err
	}
	return listDisplaysCG(ctx)
}

// Screenshot captures a single frame of the specified target.
func Screenshot(ctx context.Context, target string, output string, opts ScreenshotOptions) error {
	if err := CheckPlatform(); err != nil {
		return err
	}
	return screenshotWithScreencapture(ctx, target, output, opts)
}

// Record records video of the specified target for the given duration.
func Record(ctx context.Context, target string, output string, opts RecordOptions) error {
	if err := CheckPlatform(); err != nil {
		return err
	}
	return recordWithScreencapture(ctx, target, output, opts)
}
