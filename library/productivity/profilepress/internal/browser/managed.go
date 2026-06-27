package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ManagedOptions configures a ProfilePress-owned browser profile. This is the
// safe UX path for CLI use: it does not touch the user's primary Chrome profile.
type ManagedOptions struct {
	Binary      string
	UserDataDir string
	Port        int
	StartURL    string
	Timeout     time.Duration
}

type ManagedSession struct {
	CDPURL      string
	UserDataDir string
	PID         int
	Started     bool
}

func DefaultManagedUserDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "profilepress-browser-profile")
	}
	return filepath.Join(home, ".local", "share", "profilepress", "browser-profile")
}

func LaunchManaged(ctx context.Context, opts ManagedOptions) (ManagedSession, error) {
	if opts.Port == 0 {
		port, err := freeLocalPort()
		if err != nil {
			return ManagedSession{}, err
		}
		opts.Port = port
	}
	if opts.UserDataDir == "" {
		opts.UserDataDir = DefaultManagedUserDataDir()
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	if err := os.MkdirAll(opts.UserDataDir, 0o700); err != nil {
		return ManagedSession{}, err
	}
	binary, err := resolveBrowserBinary(opts.Binary)
	if err != nil {
		return ManagedSession{}, err
	}
	cdpURL := fmt.Sprintf("http://127.0.0.1:%d", opts.Port)
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", opts.Port),
		"--remote-debugging-address=127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=Translate",
		"--user-data-dir=" + opts.UserDataDir,
	}
	if opts.StartURL != "" {
		args = append(args, opts.StartURL)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return ManagedSession{}, fmt.Errorf("start managed browser %q: %w", binary, err)
	}
	// Let the browser outlive the CLI command; it owns an isolated profile and is
	// safe to reuse for later snapshots.
	_ = cmd.Process.Release()
	session := ManagedSession{CDPURL: cdpURL, UserDataDir: opts.UserDataDir, PID: cmd.Process.Pid, Started: true}
	readyCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := WaitForCDP(readyCtx, cdpURL, time.Second); err != nil {
		return session, err
	}
	return session, nil
}

func WaitForCDP(ctx context.Context, cdpURL string, interval time.Duration) error {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	client := NewClient(cdpURL, interval)
	var last error
	for {
		select {
		case <-ctx.Done():
			if last != nil {
				return fmt.Errorf("wait for managed browser CDP at %s: %w (last error: %v)", cdpURL, ctx.Err(), last)
			}
			return fmt.Errorf("wait for managed browser CDP at %s: %w", cdpURL, ctx.Err())
		default:
		}
		_, last = client.ListTargets(ctx)
		if last == nil {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func resolveBrowserBinary(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"}
	if runtime.GOOS == "darwin" {
		candidates = append([]string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}, candidates...)
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, string(os.PathSeparator)) {
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("could not find Chrome/Chromium; pass --browser-bin")
}

func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %s", listener.Addr())
	}
	return addr.Port, nil
}
