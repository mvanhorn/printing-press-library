package cli

// Network-error diagnostic capture for the Go-stdlib dial-timeout class
// observed 2026-05-24. When advise (or any catalog-fetch path) hits an error
// whose message matches the dial-class signature, this writes a JSONL row to
// ~/.local/state/ollama-cloud-pp-cli/dial-diag.jsonl with:
//   - timestamp
//   - error message + class (dial-timeout / dns-error / tls-handshake / other)
//   - target host (parsed from error message)
//   - parallel-probe results: same call via Go stdlib (resolver default) AND
//     via curl IPv4-only as subprocess. Observation-only; does NOT alter the
//     dial path. Future repro of the intermittent failure auto-captures the
//     full diagnostic playbook without operator presence.
//
// Per Arc-3 P24 active-now refactor (act-now instrumentation; defer the
// causation claim until repro signal accumulates).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// captureDialDiag inspects an error message; if it matches the dial-class
// signature, writes a diagnostic JSONL row and returns true. Returns false if
// the error isn't dial-class (caller continues normal handling).
//
// Best-effort: never errors back to caller. Writes silently fail-safe.
func captureDialDiag(errMsg string) bool {
	klass := classifyNetError(errMsg)
	if klass == "" {
		return false
	}
	host := parseHostFromError(errMsg)
	row := map[string]any{
		"ts":          time.Now().UTC().Format(time.RFC3339Nano),
		"error":       truncateErr(errMsg, 500),
		"class":       klass,
		"target_host": host,
	}
	// Parallel probe: dig + curl -4 with short timeout. Best-effort.
	if host != "" {
		row["dig_a"] = probeDig(host)
		row["curl_ipv4_status"] = probeCurlIPv4(host)
	}
	row["go_dns_default"] = os.Getenv("GODEBUG")
	row["goarch"] = goarchPlaceholder()
	writeDiagRow(row)
	return true
}

var dialClassPatterns = []struct {
	re    *regexp.Regexp
	klass string
}{
	{regexp.MustCompile(`dial tcp.*i/o timeout`), "dial-timeout"},
	{regexp.MustCompile(`no such host|server misbehaving|getaddrinfo`), "dns-error"},
	{regexp.MustCompile(`tls.*handshake|x509.*certificate`), "tls-handshake"},
	{regexp.MustCompile(`context deadline exceeded`), "context-timeout"},
	{regexp.MustCompile(`connection refused`), "conn-refused"},
}

func classifyNetError(msg string) string {
	for _, p := range dialClassPatterns {
		if p.re.MatchString(msg) {
			return p.klass
		}
	}
	return ""
}

// parseHostFromError extracts the target host from a Go-stdlib dial error like
// `Get "https://ollama.com/api/tags": dial tcp 34.36.133.15:443: i/o timeout`.
// Returns empty string if no host is parseable.
func parseHostFromError(msg string) string {
	// Match either the URL form or the bare IP:port form
	if m := regexp.MustCompile(`https?://([^/"]+)`).FindStringSubmatch(msg); len(m) > 1 {
		// strip :port if present
		h := m[1]
		if idx := strings.Index(h, ":"); idx > 0 {
			h = h[:idx]
		}
		return h
	}
	if m := regexp.MustCompile(`dial tcp ([0-9.]+):`).FindStringSubmatch(msg); len(m) > 1 {
		return m[1]
	}
	return ""
}

func probeDig(host string) string {
	out, err := exec.Command("dig", "+short", "+time=2", host).Output()
	if err != nil {
		return fmt.Sprintf("dig-failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func probeCurlIPv4(host string) string {
	// HEAD request, IPv4-only, 5s timeout. Just the HTTP status line.
	out, err := exec.Command("curl", "-4", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-m", "5", "-I", "https://"+host).Output()
	if err != nil {
		return fmt.Sprintf("curl-failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func goarchPlaceholder() string { return "" } // populated at build time if needed

func truncateErr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}

var diagWriteMu sync.Mutex

func writeDiagRow(row map[string]any) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".local", "state", "ollama-cloud-pp-cli", "dial-diag.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(row)
	if err != nil {
		return
	}
	diagWriteMu.Lock()
	defer diagWriteMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
