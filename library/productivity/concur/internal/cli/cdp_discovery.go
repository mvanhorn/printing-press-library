// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PATCH(amend-2026-10-17: attach to a Chrome whose remote debugging was
// enabled through chrome://inspect) -- in that mode Chrome serves no DevTools
// HTTP discovery endpoints (/json/version answers 404), so the port-only probe
// `agent-browser --cdp <port>` timed out, detection returned "", and every
// browser fallback silently launched an isolated, unauthenticated browser
// ("not logged in"). Chrome records its WebSocket endpoint in
// <user-data-dir>/DevToolsActivePort, and `agent-browser --cdp` accepts that
// ws:// URL directly (confirmed live). See
// .printing-press-patches/cdp-attach-devtools-active-port.json.

const devToolsBrowserPathPrefix = "/devtools/browser/"

// validPort parses s as a TCP port: ASCII digits only, 1-65535.
func validPort(s string) (int, bool) {
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1 && n <= 65535
}

// loopbackWSTarget validates a ws:// browser endpoint and returns its port.
// The id in the path is a session capability for full browser control, so
// only loopback hosts are ever accepted.
func loopbackWSTarget(raw string) (port string, ok bool) {
	if strings.ContainsAny(raw, " \t\r\n") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
	default:
		return "", false
	}
	if _, valid := validPort(u.Port()); !valid {
		return "", false
	}
	if len(u.Path) <= len(devToolsBrowserPathPrefix) || !strings.HasPrefix(u.Path, devToolsBrowserPathPrefix) || strings.Contains(u.Path, "..") {
		return "", false
	}
	return u.Port(), true
}

// parseCandidate accepts a bare port or a loopback ws:// URL and returns the
// TCP port to dial.
func parseCandidate(target string) (port string, ok bool) {
	if _, valid := validPort(target); valid {
		return target, true
	}
	return loopbackWSTarget(target)
}

// describeCDPTarget renders a CDP target for user-facing messages without
// leaking the session id carried by a ws:// URL.
func describeCDPTarget(target string) string {
	if port, ok := parseCandidate(target); ok {
		return "port " + port
	}
	return "the dedicated Concur browser"
}

// cdpSessionIDRE matches the session id of a DevTools browser endpoint, in a
// full URL or a bare path. The delimiter set stops the match at punctuation
// that surrounds a URL in prose or JSON, so the rest of a diagnostic survives.
var cdpSessionIDRE = regexp.MustCompile(`(/devtools/browser/)[^\s"'<>()\[\]{},;]+`)

// redactCDPTarget masks the session id of any DevTools endpoint in text about
// to be shown to the user. agent-browser's argv carries the target and the
// helper may echo it on stderr, and both end up in error messages. target is
// also replaced verbatim so an id containing a delimiter the pattern stops at
// cannot leave a tail behind.
func redactCDPTarget(text, target string) string {
	const redacted = "<redacted>"
	if i := strings.Index(target, devToolsBrowserPathPrefix); i >= 0 {
		text = strings.ReplaceAll(text, target, target[:i+len(devToolsBrowserPathPrefix)]+redacted)
	}
	return cdpSessionIDRE.ReplaceAllString(text, "${1}"+redacted)
}

// parseDevToolsActivePort turns the content of Chrome's DevToolsActivePort
// file (line 1: port, line 2: /devtools/browser/<id>) into a loopback ws://
// URL, rejecting anything malformed.
func parseDevToolsActivePort(content string) (wsURL string, ok bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return "", false
	}
	port, valid := validPort(lines[0])
	if !valid {
		return "", false
	}
	candidate := "ws://127.0.0.1:" + strconv.Itoa(port) + lines[1]
	if _, valid := loopbackWSTarget(candidate); !valid {
		return "", false
	}
	return candidate, true
}

// devToolsActivePortTargets returns the ws:// endpoints recorded in
// DevToolsActivePort files under each candidate Chrome user-data directory.
// The file sits in the user-data-dir root when debugging comes from the
// --remote-debugging-port flag, but in the profile directory (Default,
// Profile N, ...) when it was switched on at chrome://inspect, so both levels
// are read. CONCUR_CHROME_USER_DATA_DIR, when set, replaces the platform
// defaults.
func devToolsActivePortTargets() []string {
	var dirs []string
	if envDir := os.Getenv("CONCUR_CHROME_USER_DATA_DIR"); envDir != "" {
		dirs = []string{envDir}
	} else {
		home, _ := os.UserHomeDir()
		switch runtime.GOOS {
		case "darwin":
			if home != "" {
				dirs = []string{
					filepath.Join(home, "Library/Application Support/Google/Chrome"),
					filepath.Join(home, "Library/Application Support/Chromium"),
				}
			}
		case "linux":
			if home != "" {
				dirs = []string{
					filepath.Join(home, ".config/google-chrome"),
					filepath.Join(home, ".config/chromium"),
				}
			}
		case "windows":
			if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
				dirs = []string{filepath.Join(localAppData, "Google/Chrome/User Data")}
			}
		}
	}

	var targets []string
	for _, dir := range dirs {
		files := []string{filepath.Join(dir, "DevToolsActivePort")}
		if profileFiles, err := filepath.Glob(filepath.Join(dir, "*", "DevToolsActivePort")); err == nil {
			files = append(files, profileFiles...)
		}
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			if wsURL, ok := parseDevToolsActivePort(string(content)); ok {
				targets = append(targets, wsURL)
			}
		}
	}
	return targets
}

// loopbackPortOpen reports whether anything accepts TCP connections on port
// at the IPv4 or IPv6 loopback address. A refused dial fails in milliseconds,
// whereas probing a closed port through agent-browser takes far longer.
func loopbackPortOpen(port string) bool {
	for _, host := range []string{"127.0.0.1", "::1"} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}
