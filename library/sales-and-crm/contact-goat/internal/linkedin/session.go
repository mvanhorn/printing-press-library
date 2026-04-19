// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package linkedin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ProfileDir is the on-disk directory the MCP server uses to persist the
// Selenium-driven Chrome profile (logged-in LinkedIn session). If this
// directory exists and is non-empty, we assume the user has completed the
// one-time login flow.
const ProfileDir = ".linkedin-mcp/profile"

// ProfilePath returns the absolute path to the MCP's profile directory under
// the user's home directory.
func ProfilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	return filepath.Join(home, ProfileDir), nil
}

// IsLoggedIn reports whether the MCP profile directory exists and has at
// least one entry in it (Chrome writes several files/subdirs on first login).
// A missing or empty directory means we need to run the interactive
// `--login` flow.
func IsLoggedIn() (bool, error) {
	p, err := ProfilePath()
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading %s: %w", p, err)
	}
	return len(entries) > 0, nil
}

// LoginHint returns a human-readable string with the exact command a user
// should run to complete the one-time login. This is what the doctor command
// and the `linkedin` parent command surface when IsLoggedIn returns false.
func LoginHint() string {
	return "Run: uvx linkedin-scraper-mcp@latest --login\n" +
		"This opens a Chrome window once so you can sign in to LinkedIn; the\n" +
		"session is cached under ~/.linkedin-mcp/profile and reused afterwards."
}

// PythonAvailable checks whether Python 3.10+ is on PATH. Returns the
// resolved binary (python3 or python) or an empty string if neither is
// present.
func PythonAvailable(ctx context.Context) (string, string, error) {
	for _, name := range []string{"python3", "python"} {
		bin, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
		if err != nil {
			continue
		}
		return bin, string(out), nil
	}
	return "", "", errors.New("no python3 found on PATH")
}

// UVXAvailable reports whether uvx is on PATH.
func UVXAvailable() (string, error) {
	return exec.LookPath("uvx")
}

// GloballyInstalledBinary reports whether `linkedin-scraper-mcp` exists on
// PATH as a standalone script (e.g. installed via pipx). If present, callers
// can skip uvx and spawn it directly for lower startup latency.
func GloballyInstalledBinary() (string, bool) {
	p, err := exec.LookPath("linkedin-scraper-mcp")
	if err != nil {
		return "", false
	}
	return p, true
}

// ResolveSpawnCommand decides how to launch the server. It prefers a global
// install (faster startup), falls back to uvx, and errors if neither is
// available. Returns (command, args, error).
func ResolveSpawnCommand() (string, []string, error) {
	if bin, ok := GloballyInstalledBinary(); ok {
		return bin, nil, nil
	}
	if _, err := UVXAvailable(); err == nil {
		return "uvx", []string{"linkedin-scraper-mcp@latest"}, nil
	}
	return "", nil, errors.New(
		"neither uvx nor linkedin-scraper-mcp is on PATH; install uv from " +
			"https://astral.sh/uv or `pipx install linkedin-scraper-mcp`")
}
