//go:build !windows

// Platform-specific detached launch for the Hermes relay. POSIX path uses
// SysProcAttr.Setsid so the subprocess survives the CLI exiting. See
// tesla_relay.go for the cross-platform glue.

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// launchRelayDetached starts tesla-http-proxy as a detached background process
// (new session, no controlling terminal) so the relay survives the CLI exit.
// stdout + stderr both flow into the log file. Returns the PID.
func launchRelayDetached(spec relayLaunchSpec) (int, error) {
	logFile, err := os.OpenFile(spec.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log %s: %w", spec.LogPath, err)
	}
	// We hand the log fd to the child and close ours after Start; the child
	// keeps its dup.
	defer logFile.Close()

	cmd := exec.Command(spec.Binary, relayLaunchArgs(spec)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	// Setsid: new session, detach from parent's process group + controlling
	// terminal. Without this the relay would be reaped or killed when the
	// CLI exits, defeating the "lives across invocations" requirement.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	// Release the child so we don't accumulate zombies waiting on it.
	if err := cmd.Process.Release(); err != nil {
		return 0, fmt.Errorf("release child process: %w", err)
	}
	return cmd.Process.Pid, nil
}
