// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/texas-roadhouse/internal/client"
)

func waitlistMutationCases() []struct {
	name  string
	args  []string
	stdin string
} {
	return []struct {
		name  string
		args  []string
		stdin string
	}{
		{
			name: "checkin",
			args: []string{"texasroadhouse", "checkin", "218", "--no-learn"},
		},
		{
			name: "cancel",
			args: []string{"texasroadhouse", "cancel", "--waitlist-request-id", "1", "--site-id", "218", "--no-learn"},
		},
		{
			name:  "submit",
			args:  []string{"texasroadhouse", "submit", "218", "--stdin", "--no-learn"},
			stdin: submitGuestStdinJSON,
		},
	}
}

func TestWaitlistMutationsRefuseWithoutYes(t *testing.T) {
	withTempLearnHome(t)
	for _, tc := range waitlistMutationCases() {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--agent")
			_, stderr, err := runRootArgsWithStdin(t, tc.stdin, args...)
			if err == nil {
				t.Fatalf("expected refuse without --yes, stderr=%q", stderr)
			}
			if !strings.Contains(err.Error(), "requires --yes") {
				t.Fatalf("error = %v, want requires --yes (stderr=%q)", err, stderr)
			}
		})
	}
}

func TestCheckinHelpDescribesHERETextFlow(t *testing.T) {
	stdout, stderr, err := runRootArgs(t, "texasroadhouse", "checkin", "--help")
	if err != nil {
		t.Fatalf("checkin --help: %v (stderr=%q)", err, stderr)
	}
	if strings.Contains(strings.ToLower(stdout), "host stand") {
		t.Fatalf("checkin help must not direct guests to the host stand: %q", stdout)
	}
	if !strings.Contains(stdout, "HERE") {
		t.Fatalf("checkin help must describe the guest HERE text flow: %q", stdout)
	}
}

func TestREADMEStoreTroubleshootingUsesActualFlags(t *testing.T) {
	readmePath := filepath.Join("..", "..", "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}
	text := string(readme)
	if strings.Contains(text, "stores --latitude") || strings.Contains(text, "--longitude <lon>") {
		t.Fatalf("README store troubleshooting uses unsupported coordinate flags")
	}
	if !strings.Contains(text, "stores --lat <lat> --long <lon>") {
		t.Fatalf("README store troubleshooting must use the command's --lat/--long flags")
	}
}

func TestWaitlistMutationsDryRunDoesNotPost(t *testing.T) {
	withTempLearnHome(t)
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(installClientBaseURL(srv.URL))

	for _, tc := range waitlistMutationCases() {
		t.Run(tc.name, func(t *testing.T) {
			posts.Store(0)
			args := append(append([]string{}, tc.args...), "--dry-run", "--agent")
			_, stderr, err := runRootArgsWithStdin(t, tc.stdin, args...)
			if err != nil {
				t.Fatalf("dry-run should succeed: %v (stderr=%q)", err, stderr)
			}
			if posts.Load() != 0 {
				t.Fatalf("dry-run posted %d time(s)", posts.Load())
			}
		})
	}
}

func TestWaitlistMutationsYesPosts(t *testing.T) {
	withTempLearnHome(t)
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(installClientBaseURL(srv.URL))

	for _, tc := range waitlistMutationCases() {
		t.Run(tc.name, func(t *testing.T) {
			posts.Store(0)
			args := append(append([]string{}, tc.args...), "--yes", "--agent")
			_, stderr, err := runRootArgsWithStdin(t, tc.stdin, args...)
			if err != nil {
				t.Fatalf("--yes should POST: %v (stderr=%q)", err, stderr)
			}
			if posts.Load() == 0 {
				t.Fatal("expected a live POST with --yes")
			}
		})
	}
}

func installClientBaseURL(baseURL string) func() {
	orig := clientHooks
	clientHooks = append(append([]func(*client.Client) error{}, orig...), func(c *client.Client) error {
		c.BaseURL = strings.TrimRight(baseURL, "/")
		return nil
	})
	return func() { clientHooks = orig }
}

func TestSubmitPrivateYesPathPosts(t *testing.T) {
	withTempLearnHome(t)
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(installClientBaseURL(srv.URL))
	_, stderr, err := runRootArgs(t,
		"texasroadhouse", "submit", "218",
		"--email-address", "guest@example.test",
		"--first-name", "Test",
		"--last-name", "User",
		"--primary-phone-area-code", "555",
		"--primary-phone-number", "555-0100",
		"--party-size", "2",
		"--wait-minutes", "10",
		"--yes", "--agent", "--no-learn",
	)
	if err != nil {
		t.Fatalf("private --yes path should POST: %v (stderr=%q)", err, stderr)
	}
	if posts.Load() == 0 {
		t.Fatal("expected a live POST with hidden flags + --yes")
	}
}
