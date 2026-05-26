package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// defaultBridgeTimeout caps how long a single bridge subprocess can run before
// being killed. Override at runtime with FRAMER_BRIDGE_TIMEOUT (Go duration
// string, e.g. "5m" or "30s"). Without a timeout a hung Framer WebSocket would
// block the CLI process indefinitely on every bridge-powered command.
const defaultBridgeTimeout = 120 * time.Second

// BridgeClient calls the Node.js framer-bridge.mjs script to communicate
// with the Framer Server API over WebSocket.
type BridgeClient struct {
	BridgePath string // absolute path to framer-bridge.mjs
	NodeBin    string // path to node binary
	ProjectURL string // FRAMER_PROJECT_URL
	APIKey     string // FRAMER_API_KEY
}

// NewBridgeClient creates a bridge client, resolving paths and env vars.
func NewBridgeClient() (*BridgeClient, error) {
	projectURL := os.Getenv("FRAMER_PROJECT_URL")
	if projectURL == "" {
		return nil, fmt.Errorf("FRAMER_PROJECT_URL not set — get your project URL from the Framer editor address bar")
	}
	apiKey := os.Getenv("FRAMER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("FRAMER_API_KEY not set — generate one in Framer > Site Settings > General")
	}

	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node not found on PATH — install Node.js 18+ from https://nodejs.org")
	}

	bridgePath, err := findBridge()
	if err != nil {
		return nil, err
	}

	return &BridgeClient{
		BridgePath: bridgePath,
		NodeBin:    nodeBin,
		ProjectURL: projectURL,
		APIKey:     apiKey,
	}, nil
}

// Call invokes a bridge command and returns the parsed JSON result.
// The subprocess is killed if it does not complete within the bridge timeout
// (default 120s; override with FRAMER_BRIDGE_TIMEOUT, e.g. "5m").
func (bc *BridgeClient) Call(command string, arg ...string) (json.RawMessage, error) {
	args := []string{bc.BridgePath, command}
	args = append(args, arg...)

	timeout := defaultBridgeTimeout
	if raw := os.Getenv("FRAMER_BRIDGE_TIMEOUT"); raw != "" {
		if d, perr := time.ParseDuration(raw); perr == nil && d > 0 {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bc.NodeBin, args...)
	cmd.Env = append(os.Environ(),
		"FRAMER_PROJECT_URL="+bc.ProjectURL,
		"FRAMER_API_KEY="+bc.APIKey,
	)
	// Set working dir to bridge dir so node_modules resolves
	cmd.Dir = filepath.Dir(bc.BridgePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("bridge command %q timed out after %s (override via FRAMER_BRIDGE_TIMEOUT, e.g. FRAMER_BRIDGE_TIMEOUT=5m)", command, timeout)
	}
	if err != nil {
		// Try to parse stderr as JSON error
		var bridgeErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(stderr.Bytes(), &bridgeErr) == nil && bridgeErr.Error != "" {
			return nil, fmt.Errorf("framer API: %s", bridgeErr.Error)
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("bridge error: %s", errMsg)
	}

	raw := json.RawMessage(bytes.TrimSpace(stdout.Bytes()))
	if len(raw) == 0 {
		return nil, fmt.Errorf("bridge returned empty response for %s", command)
	}
	return raw, nil
}

// CallInto invokes a bridge command and unmarshals the result into v.
func (bc *BridgeClient) CallInto(v interface{}, command string, arg ...string) error {
	raw, err := bc.Call(command, arg...)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// findBridge locates framer-bridge.mjs relative to the running binary.
func findBridge() (string, error) {
	// Try relative to binary
	execPath, err := os.Executable()
	if err == nil {
		execPath, _ = filepath.EvalSymlinks(execPath)
		dir := filepath.Dir(execPath)
		candidate := filepath.Join(dir, "bridge", "framer-bridge.mjs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// Also try one level up (when binary is in build/ or cmd/)
		candidate = filepath.Join(dir, "..", "bridge", "framer-bridge.mjs")
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs, nil
			}
		}
	}

	// Try relative to working directory
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "bridge", "framer-bridge.mjs")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Try well-known library path
	home, _ := os.UserHomeDir()
	candidate = filepath.Join(home, "printing-press", "library", "framer", "bridge", "framer-bridge.mjs")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Platform-specific paths
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		candidate = filepath.Join(home, ".local", "share", "framer-pp-cli", "bridge", "framer-bridge.mjs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("framer-bridge.mjs not found — expected at <cli-dir>/bridge/framer-bridge.mjs")
}
