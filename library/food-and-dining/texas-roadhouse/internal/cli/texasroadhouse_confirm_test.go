// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/texas-roadhouse/internal/client"
)

func waitlistMutationCases() []struct {
	name string
	args []string
} {
	return []struct {
		name string
		args []string
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
			name: "submit",
			args: []string{
				"texasroadhouse", "submit", "218",
				"--email-address", "guest@example.test",
				"--first-name", "Test",
				"--last-name", "User",
				"--primary-phone-area-code", "555",
				"--primary-phone-number", "555-0100",
				"--party-size", "2",
				"--wait-minutes", "10",
				"--no-learn",
			},
		},
	}
}

func TestWaitlistMutationsRefuseWithoutYes(t *testing.T) {
	withTempLearnHome(t)
	for _, tc := range waitlistMutationCases() {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--agent")
			_, stderr, err := runRootArgs(t, args...)
			if err == nil {
				t.Fatalf("expected refuse without --yes, stderr=%q", stderr)
			}
			if !strings.Contains(err.Error(), "requires --yes") {
				t.Fatalf("error = %v, want requires --yes (stderr=%q)", err, stderr)
			}
		})
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
			_, stderr, err := runRootArgs(t, args...)
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
			_, stderr, err := runRootArgs(t, args...)
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
