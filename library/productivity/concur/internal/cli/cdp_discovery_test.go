package cli

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PATCH(amend-2026-10-17: add test suite for CDP discovery and safety)

func TestParseDevToolsActivePort(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantURL string
		wantOk  bool
	}{
		{
			name:    "valid",
			content: "9222\n/devtools/browser/abc",
			wantURL: "ws://127.0.0.1:9222/devtools/browser/abc",
			wantOk:  true,
		},
		{
			name:    "CRLF",
			content: "9222\r\n/devtools/browser/abc\r\n",
			wantURL: "ws://127.0.0.1:9222/devtools/browser/abc",
			wantOk:  true,
		},
		{
			name:    "non-numeric port",
			content: "abc\n/devtools/browser/abc",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "port 0",
			content: "0\n/devtools/browser/abc",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "port 70000",
			content: "70000\n/devtools/browser/abc",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "missing path",
			content: "9222",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "missing path newline",
			content: "9222\n",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "path without prefix",
			content: "9222\n/browser/abc",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "path containing ..",
			content: "9222\n/devtools/browser/../abc",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "whitespace in path",
			content: "9222\n/devtools/browser/abc def",
			wantURL: "",
			wantOk:  false,
		},
		{
			name:    "empty",
			content: "",
			wantURL: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotOk := parseDevToolsActivePort(tt.content)
			if gotOk != tt.wantOk {
				t.Fatalf("got ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotOk && gotURL != tt.wantURL {
				t.Fatalf("got URL = %q, want %q", gotURL, tt.wantURL)
			}
		})
	}
}

func TestDevToolsActivePortTargets(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("CONCUR_CHROME_USER_DATA_DIR", tempDir)

	t.Run("absent", func(t *testing.T) {
		targets := devToolsActivePortTargets()
		if len(targets) != 0 {
			t.Fatalf("expected 0 targets, got %v", targets)
		}
	})

	t.Run("present and valid", func(t *testing.T) {
		content := "9222\n/devtools/browser/valid-id\n"
		err := os.WriteFile(filepath.Join(tempDir, "DevToolsActivePort"), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
		targets := devToolsActivePortTargets()
		if len(targets) != 1 || targets[0] != "ws://127.0.0.1:9222/devtools/browser/valid-id" {
			t.Fatalf("unexpected targets: %v", targets)
		}
	})

	t.Run("profile subdirectory (chrome://inspect mode)", func(t *testing.T) {
		if err := os.Remove(filepath.Join(tempDir, "DevToolsActivePort")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		profileDir := filepath.Join(tempDir, "Default")
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "9222\n/devtools/browser/profile-id\n"
		if err := os.WriteFile(filepath.Join(profileDir, "DevToolsActivePort"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		targets := devToolsActivePortTargets()
		if len(targets) != 1 || targets[0] != "ws://127.0.0.1:9222/devtools/browser/profile-id" {
			t.Fatalf("unexpected targets: %v", targets)
		}
		_ = os.RemoveAll(profileDir)
	})

	t.Run("malformed", func(t *testing.T) {
		content := "abc\n/devtools/browser/valid-id\n"
		err := os.WriteFile(filepath.Join(tempDir, "DevToolsActivePort"), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
		targets := devToolsActivePortTargets()
		if len(targets) != 0 {
			t.Fatalf("expected 0 targets, got %v", targets)
		}
	})
}

func TestDescribeCDPTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "plain port",
			target: "9222",
			want:   "port 9222",
		},
		{
			name:   "ws URL with host, port, path, ID",
			target: "ws://127.0.0.1:9222/devtools/browser/abcdef-1234",
			want:   "port 9222",
		},
		{
			name:   "ws URL IPv6",
			target: "ws://[::1]:9333/devtools/browser/xyz",
			want:   "port 9333",
		},
		{
			name:   "invalid ws URL / anything else",
			target: "invalid-value",
			want:   "the dedicated Concur browser",
		},
		{
			name:   "empty",
			target: "",
			want:   "the dedicated Concur browser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeCDPTarget(tt.target)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			// Verify that the ID never appears in the output
			if strings.Contains(got, "abcdef-1234") || strings.Contains(got, "xyz") {
				t.Fatalf("target description leaked session ID: %q", got)
			}
		})
	}
}

func TestDetectDedicatedConcurBrowser(t *testing.T) {
	// Save and temporarily clear package-level candidates to isolate test from real local ports
	origCandidates := concurCDPPortCandidates
	concurCDPPortCandidates = nil
	t.Cleanup(func() {
		concurCDPPortCandidates = origCandidates
	})

	// Create a temp directory for mock agent-browser
	mockBinDir := t.TempDir()
	mockPath := filepath.Join(mockBinDir, "agent-browser")

	recordFile := filepath.Join(mockBinDir, "args.log")

	// Mock script: appends command line args to recordFile and returns successful JSON for "get url"
	mockScript := fmt.Sprintf(`#!/bin/bash
echo "$@" >> "%s"
if [[ "$*" == *"get url"* ]]; then
	echo '{"success":true,"data":{"url":"https://us2.concursolutions.com/home"}}'
fi
`, recordFile)

	err := os.WriteFile(mockPath, []byte(mockScript), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Put mock bin on PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", mockBinDir+string(filepath.ListSeparator)+origPath)

	// Set up throwaway listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}

	tempUserDir := t.TempDir()
	t.Setenv("CONCUR_CHROME_USER_DATA_DIR", tempUserDir)

	// Write DevToolsActivePort pointing to our listener's port
	dtContent := fmt.Sprintf("%s\n/devtools/browser/mock-session-id-123\n", portStr)
	err = os.WriteFile(filepath.Join(tempUserDir, "DevToolsActivePort"), []byte(dtContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("detection success with open port", func(t *testing.T) {
		// Clear args log
		_ = os.Remove(recordFile)

		gotCDP := detectDedicatedConcurBrowser()
		wantCDP := fmt.Sprintf("ws://127.0.0.1:%s/devtools/browser/mock-session-id-123", portStr)
		if gotCDP != wantCDP {
			t.Fatalf("got CDP %q, want %q", gotCDP, wantCDP)
		}

		// Verify that the mock agent-browser was called with correct cdp argument
		argsData, err := os.ReadFile(recordFile)
		if err != nil {
			t.Fatalf("failed to read recorded args: %v", err)
		}
		argsStr := string(argsData)
		expectedArg := fmt.Sprintf("--cdp ws://127.0.0.1:%s/devtools/browser/mock-session-id-123", portStr)
		if !strings.Contains(argsStr, expectedArg) {
			t.Fatalf("expected args to contain %q, got: %q", expectedArg, argsStr)
		}
	})

	t.Run("detection skipped with closed port", func(t *testing.T) {
		// Close the listener so the port is closed
		listener.Close()

		// Clear args log
		_ = os.Remove(recordFile)

		gotCDP := detectDedicatedConcurBrowser()
		if gotCDP != "" {
			t.Fatalf("expected detection to skip and return empty, got %q", gotCDP)
		}

		// Verify that mock agent-browser was NEVER called (since port is closed and skipped before executing exec)
		if _, err := os.Stat(recordFile); err == nil {
			argsData, _ := os.ReadFile(recordFile)
			t.Fatalf("expected agent-browser to not be spawned, but it recorded args: %s", string(argsData))
		}
	})
}

// PATCH(Greptile review, "Browser errors expose session URLs") -- the ws://
// session id is a browser-control capability and must not reach error text.
func TestRedactCDPTarget(t *testing.T) {
	const (
		secret = "0f3a-secret-id"
		ws     = "ws://127.0.0.1:9222/devtools/browser/" + secret
	)
	tests := []struct {
		name   string
		text   string
		target string
		want   string
	}{
		{
			name:   "target in argv",
			text:   "agent-browser --cdp " + ws + " --session concur-pp-cli open https://x",
			target: ws,
			want:   "agent-browser --cdp ws://127.0.0.1:9222/devtools/browser/<redacted> --session concur-pp-cli open https://x",
		},
		{
			name:   "target echoed on stderr inside quotes",
			text:   `Failed to connect to "` + ws + `": connection refused`,
			target: ws,
			want:   `Failed to connect to "ws://127.0.0.1:9222/devtools/browser/<redacted>": connection refused`,
		},
		{
			name:   "http form derived by the helper",
			text:   "GET http://127.0.0.1:9222/devtools/browser/" + secret + " returned 404",
			target: ws,
			want:   "GET http://127.0.0.1:9222/devtools/browser/<redacted> returned 404",
		},
		{
			name:   "bare path followed by sentence punctuation",
			text:   "unexpected path /devtools/browser/" + secret + ", giving up",
			target: ws,
			want:   "unexpected path /devtools/browser/<redacted>, giving up",
		},
		{
			name:   "id containing a delimiter is still removed",
			text:   "bad endpoint ws://127.0.0.1:9222/devtools/browser/ab(cd-" + secret + ")",
			target: "ws://127.0.0.1:9222/devtools/browser/ab(cd-" + secret + ")",
			want:   "bad endpoint ws://127.0.0.1:9222/devtools/browser/<redacted>",
		},
		{
			name:   "session id found even when it is not the active target",
			text:   "other browser at ws://127.0.0.1:9333/devtools/browser/" + secret,
			target: "9222",
			want:   "other browser at ws://127.0.0.1:9333/devtools/browser/<redacted>",
		},
		{
			name:   "bare port target leaves text alone",
			text:   "agent-browser --cdp 9222 --session s open https://x failed",
			target: "9222",
			want:   "agent-browser --cdp 9222 --session s open https://x failed",
		},
		{
			name:   "no target",
			text:   "agent-browser --session s open https://x failed: boom",
			target: "",
			want:   "agent-browser --session s open https://x failed: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactCDPTarget(tt.text, tt.target)
			if got != tt.want {
				t.Fatalf("got  %q\nwant %q", got, tt.want)
			}
			if strings.Contains(got, secret) {
				t.Fatalf("session id leaked: %q", got)
			}
			if again := redactCDPTarget(got, tt.target); again != got {
				t.Fatalf("not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestRunAgentBrowserErrorRedactsCDPTarget(t *testing.T) {
	const secret = "0f3a-secret-id"
	ws := "ws://127.0.0.1:9222/devtools/browser/" + secret

	mockBinDir := t.TempDir()
	// The helper fails and echoes the endpoint it was given, as a connect
	// failure would.
	mockScript := "#!/bin/bash\n" +
		"echo \"connect failed: $2\" >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(mockBinDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", mockBinDir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	origPort := activeCDPPort
	t.Cleanup(func() { activeCDPPort = origPort })

	t.Run("ws target is redacted from argv and stderr", func(t *testing.T) {
		activeCDPPort = ws
		_, err := runAgentBrowser("open", "https://us2.concursolutions.com/home")
		if err == nil {
			t.Fatal("expected an error from the failing helper")
		}
		msg := err.Error()
		if strings.Contains(msg, secret) {
			t.Fatalf("error leaked the session id: %s", msg)
		}
		for _, want := range []string{
			"--cdp ws://127.0.0.1:9222/devtools/browser/<redacted>",
			"connect failed: ws://127.0.0.1:9222/devtools/browser/<redacted>",
			"open https://us2.concursolutions.com/home",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("error lost useful context %q: %s", want, msg)
			}
		}
	})

	t.Run("port target is left readable", func(t *testing.T) {
		activeCDPPort = "9222"
		_, err := runAgentBrowser("open", "https://us2.concursolutions.com/home")
		if err == nil {
			t.Fatal("expected an error from the failing helper")
		}
		if !strings.Contains(err.Error(), "--cdp 9222") {
			t.Errorf("a bare port is not a secret and should stay in the error: %v", err)
		}
	})
}
