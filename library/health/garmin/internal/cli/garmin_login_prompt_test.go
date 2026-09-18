// Copyright 2026 Prashant Kamani and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The sign-in-form confirmation is a question to a person, and a person is
// slow. These tests pin what happens while it is outstanding: a sign-in that
// comes back first wins, a no still aborts, and nothing waits past --timeout.
// Nothing here opens a browser or reaches Garmin — the only hosts touched are
// the loopback callback and an httptest token service.

package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// garminBlockingPrompt is a terminal nobody is typing at: the first read
// records that the prompt was asked, runs fire (a test's stand-in for the
// browser finishing the sign-in), and then blocks until the test releases it.
// fire() returns once the callback's success page has been flushed; the
// channel send follows immediately. The test does not depend on that
// ordering — while Read is blocked the answers arm can never fire, so the
// ticket arm is the only way the select can proceed.
type garminBlockingPrompt struct {
	rec     *loginRecorder
	fire    func()
	release <-chan struct{}
	once    sync.Once
}

func (p *garminBlockingPrompt) Read([]byte) (int, error) {
	p.once.Do(func() {
		p.rec.add("prompt")
		if p.fire != nil {
			p.fire()
		}
	})
	<-p.release
	return 0, io.EOF
}

// runLoginWithWatchdog runs a login off the test goroutine so a regression
// that reintroduces the block fails the test instead of hanging it. The timer
// is a failure detector, never a synchroniser: the pass path never reaches it.
func runLoginWithWatchdog(t *testing.T, run func(garminLoginOptions) (error, string, string), opts garminLoginOptions) (error, string) {
	t.Helper()
	type outcome struct {
		err error
		out string
	}
	done := make(chan outcome, 1)
	go func() {
		err, out, _ := run(opts)
		done <- outcome{err: err, out: out}
	}()
	select {
	case got := <-done:
		return got.err, got.out
	case <-time.After(30 * time.Second):
		t.Fatal("the login never returned; it is still blocked on the sign-in form prompt")
		return nil, ""
	}
}

// serviceURLNoFatal is serviceURLFrom for a caller that is not the test
// goroutine: it reports rather than aborts, because t.Fatalf from another
// goroutine does not stop the test it belongs to.
func serviceURLNoFatal(t *testing.T, ssoURL string) string {
	t.Helper()
	parsed, err := url.Parse(ssoURL)
	if err != nil {
		t.Errorf("parse sso url %q: %v", ssoURL, err)
		return ""
	}
	service := parsed.Query().Get("service")
	if service == "" {
		t.Errorf("sign-in URL carries no service parameter: %q", ssoURL)
	}
	return service
}

// The bug this pins (observed 2026-09-12): a person signs in through the
// browser without answering the terminal question, and the login sits on
// stdin until the single-use ticket ages out. The ticket must win.
func TestLoginProceedsWhenTheSignInBeatsTheFormAnswer(t *testing.T) {
	var exchanges int
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		exchanges++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer tokenSrv.Close()
	t.Setenv("GARMIN_TOKEN_URL", tokenSrv.URL)

	rec := &loginRecorder{}
	var signinURL string
	rec.onSignin = func(raw string) { signinURL = raw }

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	prompt := &garminBlockingPrompt{rec: rec, release: release}
	prompt.fire = func() {
		service := serviceURLNoFatal(t, signinURL)
		if service == "" {
			return
		}
		resp, err := http.Get(service + "&ticket=ST-synthetic-not-a-real-ticket") //nolint:noctx // loopback
		if err != nil {
			t.Errorf("callback probe: %v", err)
			return
		}
		_ = resp.Body.Close()
	}

	run := newLoginHarness(t, rec, true, prompt)
	// An hour: the assertion is that the login does not wait for the answer,
	// not that some shorter deadline rescued it.
	err, out := runLoginWithWatchdog(t, run, garminLoginOptions{Email: "placeholder@example.test", Timeout: time.Hour})

	if err == nil {
		t.Fatal("the synthetic exchange was expected to fail")
	}
	if strings.Contains(err.Error(), "sign-in form is not empty") {
		t.Fatalf("the unanswered prompt aborted a completed sign-in: %v", err)
	}
	if strings.Contains(err.Error(), "did not come back within") {
		t.Fatalf("the already-delivered ticket was never picked up: %v", err)
	}
	if exchanges != 1 {
		t.Fatalf("token exchanges = %d, want the early ticket exchanged exactly once", exchanges)
	}
	if !strings.Contains(out, "Sign-in came back before the form check") {
		t.Fatalf("the login did not say why it stopped waiting for the answer: %q", out)
	}
	steps := rec.seen()
	if len(steps) != 3 || steps[0] != "logout" || steps[2] != "prompt" {
		t.Fatalf("login order = %v, want logout then signin then prompt", steps)
	}
}

// The answer still governs when it arrives first: no is the abort that keeps a
// surviving SSO session from costing a real Garmin round trip.
func TestLoginStillAbortsOnNoWhenNoTicketArrives(t *testing.T) {
	rec := &loginRecorder{}
	run := newLoginHarness(t, rec, true, &promptReader{rec: rec, answer: "n\n"})
	err, out := runLoginWithWatchdog(t, run, garminLoginOptions{Email: "placeholder@example.test", Timeout: time.Hour})

	if err == nil {
		t.Fatal("answering no still ran the login")
	}
	if !strings.Contains(err.Error(), "sign-in form is not empty") {
		t.Fatalf("error = %v, want the not-empty-form abort", err)
	}
	if !strings.Contains(err.Error(), "sso.garmin.com/sso/logout") {
		t.Fatalf("the abort does not name the sign-out step: %v", err)
	}
	if strings.Contains(out, "Sign-in came back before the form check") {
		t.Fatalf("a ticket was claimed although none arrived: %q", out)
	}
}

// Neither an answer nor a sign-in: --timeout has to bound the question too, or
// the terminal waits forever.
func TestLoginPromptIsBoundedByTheLoginTimeout(t *testing.T) {
	rec := &loginRecorder{}
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	prompt := &garminBlockingPrompt{rec: rec, release: release}

	run := newLoginHarness(t, rec, true, prompt)
	err, out := runLoginWithWatchdog(t, run, garminLoginOptions{Email: "placeholder@example.test", Timeout: 150 * time.Millisecond})

	if err == nil || !strings.Contains(err.Error(), "did not come back within") {
		t.Fatalf("error = %v, want the login timeout", err)
	}
	if !strings.Contains(out, "EMPTY sign-in form") {
		t.Fatalf("the confirmation was never asked: %q", out)
	}
	steps := rec.seen()
	if len(steps) != 3 || steps[2] != "prompt" {
		t.Fatalf("steps = %v, want logout, signin, prompt", steps)
	}
}
