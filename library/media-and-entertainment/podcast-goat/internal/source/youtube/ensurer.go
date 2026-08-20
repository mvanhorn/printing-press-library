// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Lazy yt-dlp ensurer: detects PATH, falls through to a managed sidecar at
// ~/.config/podcast-goat/bin/yt-dlp, falls through to a one-time download
// from yt-dlp's GitHub releases.
//
// This is the "magically works out of the box" path. First YouTube call eats
// a ~35MB one-time download; every later call is instant. Users who run
// `brew install yt-dlp` bypass the sidecar entirely (PATH wins).

package youtube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// SidecarDir is where the managed yt-dlp binary lives. Created on first
// download. Override via PODCAST_GOAT_YTDLP_DIR for tests / multi-tenant runs.
func SidecarDir() string {
	if v := os.Getenv("PODCAST_GOAT_YTDLP_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "podcast-goat", "bin")
	}
	return filepath.Join(home, ".config", "podcast-goat", "bin")
}

// SidecarPath is the expected location of the managed yt-dlp binary.
func SidecarPath() string {
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name = "yt-dlp.exe"
	}
	return filepath.Join(SidecarDir(), name)
}

// SidecarPresent returns true if the managed yt-dlp binary exists and is
// executable. Does not validate that it runs successfully — that's the
// caller's responsibility.
func SidecarPresent() bool {
	st, err := os.Stat(SidecarPath())
	if err != nil {
		return false
	}
	return !st.IsDir() && st.Mode().Perm()&0o111 != 0
}

// PathPresent returns true if `yt-dlp` is available on the user's PATH.
// PATH-installed yt-dlp always wins over the sidecar so brew/pip upgrades
// take effect immediately.
func PathPresent() bool {
	_, err := exec.LookPath("yt-dlp")
	return err == nil
}

// releaseAssetName returns the yt-dlp release asset that matches the
// current OS/arch. Returns "" for unsupported combinations.
func releaseAssetName() string {
	switch runtime.GOOS {
	case "darwin":
		return "yt-dlp_macos"
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "yt-dlp_linux"
		case "arm64":
			return "yt-dlp_linux_aarch64"
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "yt-dlp.exe"
		case "arm64":
			return "yt-dlp_arm64.exe"
		case "386":
			return "yt-dlp_x86.exe"
		}
	}
	return ""
}

// releaseURL builds the redirecting "latest" download URL for the asset.
func releaseURL(asset string) string {
	return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + asset
}

// DownloadSidecar fetches the right yt-dlp binary for this OS/arch into
// SidecarPath(). Replaces any existing sidecar atomically. Emits one
// progress line to progressW (typically os.Stderr) so the user knows why
// the first YouTube call is taking a moment.
//
// Designed to be safe to call concurrently — the final rename is atomic
// and stragglers will see an already-good binary.
func DownloadSidecar(ctx context.Context, progressW io.Writer) error {
	asset := releaseAssetName()
	if asset == "" {
		return fmt.Errorf("yt-dlp auto-download not supported on %s/%s — install manually: `brew install yt-dlp` or `pip install yt-dlp`", runtime.GOOS, runtime.GOARCH)
	}
	url := releaseURL(asset)

	if err := os.MkdirAll(SidecarDir(), 0o755); err != nil {
		return fmt.Errorf("create sidecar dir: %w", err)
	}

	if progressW != nil {
		fmt.Fprintf(progressW, "podcast-goat: downloading yt-dlp (one-time, ~35MB) from %s ...\n", url)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "podcast-goat-pp-cli/0.1 (+lazy yt-dlp ensurer)")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("yt-dlp download GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("yt-dlp download HTTP %d (url=%s)", resp.StatusCode, url)
	}

	// Stage to a tmp file in the same dir so the final rename is atomic on
	// the same filesystem.
	tmp, err := os.CreateTemp(SidecarDir(), ".yt-dlp.partial-*")
	if err != nil {
		return fmt.Errorf("yt-dlp staging file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // best-effort cleanup if we don't rename

	written, err := io.Copy(tmp, resp.Body)
	if cerr := tmp.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("yt-dlp staging copy: %w", err)
	}
	if written < 1<<20 {
		// Smaller than 1MB means we got an HTML error page or a redirect.
		// Real yt-dlp binaries are 18-35MB. Refuse to install garbage.
		return fmt.Errorf("yt-dlp download too small (%d bytes) — likely an HTML error page, not the binary", written)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return fmt.Errorf("chmod yt-dlp: %w", err)
		}
	}

	final := SidecarPath()
	if err := os.Rename(tmpPath, final); err != nil {
		return fmt.Errorf("install yt-dlp to %s: %w", final, err)
	}

	if progressW != nil {
		fmt.Fprintf(progressW, "podcast-goat: yt-dlp installed at %s (%.1f MB)\n", final, float64(written)/1024/1024)
	}
	return nil
}

// EnsureYtDlp returns a runnable yt-dlp binary path. Resolution order:
//  1. user-provided override (Adapter.Bin if not the literal "yt-dlp")
//  2. yt-dlp on PATH
//  3. managed sidecar at SidecarPath()
//  4. one-time auto-download from yt-dlp's GitHub releases
//
// The caller should pass a writer (typically os.Stderr) for the progress line
// the auto-download path emits.
func EnsureYtDlp(ctx context.Context, override string, progressW io.Writer) (string, error) {
	if override != "" && override != "yt-dlp" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		if path, err := exec.LookPath(override); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("yt-dlp override %q not found", override)
	}
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		return path, nil
	}
	if SidecarPresent() {
		return SidecarPath(), nil
	}
	if err := DownloadSidecar(ctx, progressW); err != nil {
		return "", err
	}
	return SidecarPath(), nil
}
