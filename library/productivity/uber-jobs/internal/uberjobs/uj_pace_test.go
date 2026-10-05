// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ujFakeGate returns a gate on dir whose clock only moves when the test (or
// a pause) moves it, plus the list of pauses it asked for.
func ujFakeGate(dir string, gap time.Duration, now *time.Time) (*Gate, *[]time.Duration) {
	var slept []time.Duration
	g := &Gate{Dir: dir, Gap: gap,
		now: func() time.Time { return *now },
		sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			*now = now.Add(d)
			return nil
		},
	}
	return g, &slept
}

// TestGateWaitsOutTheGap: a second request inside the gap pauses for the
// remainder; one after the gap does not pause at all.
func TestGateWaitsOutTheGap(t *testing.T) {
	now := ujT0
	g, slept := ujFakeGate(t.TempDir(), 3*time.Second, &now)
	ctx := context.Background()

	if err := g.Wait(ctx); err != nil {
		t.Fatalf("first Wait: %v", err)
	}
	if len(*slept) != 0 {
		t.Fatalf("first request paused %v, want no pause", *slept)
	}
	now = now.Add(1200 * time.Millisecond)
	if err := g.Wait(ctx); err != nil {
		t.Fatalf("second Wait: %v", err)
	}
	if len(*slept) != 1 || (*slept)[0] != 1800*time.Millisecond {
		t.Fatalf("second request paused %v, want exactly 1.8s (gap minus elapsed)", *slept)
	}
	last, ok := g.LastRequest()
	if !ok || !last.Equal(ujT0.Add(3*time.Second)) {
		t.Errorf("LastRequest = %v %v, want the post-pause time %v", last, ok, ujT0.Add(3*time.Second))
	}
	now = now.Add(3 * time.Second)
	if err := g.Wait(ctx); err != nil {
		t.Fatalf("third Wait: %v", err)
	}
	if len(*slept) != 1 {
		t.Errorf("request after the gap paused: %v", *slept)
	}
}

// TestGateSharedAcrossProcesses: two gates on the same dir (two CLI
// processes) see each other's last request through the file.
func TestGateSharedAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	now := ujT0
	a, _ := ujFakeGate(dir, 3*time.Second, &now)
	b, slept := ujFakeGate(dir, 3*time.Second, &now)
	if err := a.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(500 * time.Millisecond)
	if err := b.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != 2500*time.Millisecond {
		t.Errorf("second process paused %v, want 2.5s", *slept)
	}
}

// TestGateCancelledWaitDoesNotRecord: a cancelled wait returns the context
// error and does not stamp a request that never happened. This uses the
// real pause, which must return at once on a done context.
func TestGateCancelledWaitDoesNotRecord(t *testing.T) {
	dir := t.TempDir()
	g := &Gate{Dir: dir, Gap: time.Hour}
	if err := g.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, _ := g.LastRequest()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := g.Wait(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("cancelled wait blocked for %v", time.Since(start))
	}
	after, _ := g.LastRequest()
	if !after.Equal(before) {
		t.Errorf("cancelled wait moved the last-request stamp from %v to %v", before, after)
	}
}

// TestGateEmptyDirNeverWaits: no state dir means no gate file and no pause;
// a nil gate is also a no-op.
func TestGateEmptyDirNeverWaits(t *testing.T) {
	g := &Gate{Gap: time.Hour, sleep: func(context.Context, time.Duration) error {
		t.Fatal("gate with no dir paused")
		return nil
	}}
	for i := 0; i < 3; i++ {
		if err := g.Wait(context.Background()); err != nil {
			t.Fatalf("Wait %d: %v", i, err)
		}
	}
	var nilGate *Gate
	if err := nilGate.Wait(context.Background()); err != nil {
		t.Errorf("nil gate Wait: %v", err)
	}
	if _, ok := NewGate("").LastRequest(); ok {
		t.Errorf("empty gate reports a last request")
	}
}

// TestGateCorruptLastFile: an unparseable stamp does not wedge the gate;
// the request proceeds and a valid stamp replaces it.
func TestGateCorruptLastFile(t *testing.T) {
	dir := t.TempDir()
	now := ujT0
	g, slept := ujFakeGate(dir, 3*time.Second, &now)
	if err := os.WriteFile(filepath.Join(dir, "last-request-unixnano"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 0 {
		t.Errorf("paused %v on a corrupt stamp", *slept)
	}
	if last, ok := g.LastRequest(); !ok || !last.Equal(ujT0) {
		t.Errorf("LastRequest = %v %v, want %v", last, ok, ujT0)
	}
}

// TestNewGateMinGapEnv: UBER_JOBS_MIN_GAP may raise the gap but never
// lower it below MinRequestGap, and junk falls back to MinRequestGap.
func TestNewGateMinGapEnv(t *testing.T) {
	cases := []struct {
		env  string
		want time.Duration
	}{
		{"", MinRequestGap},
		{"10s", 10 * time.Second},
		{"1m", time.Minute},
		{"1d", 24 * time.Hour},
		{"1s", MinRequestGap},
		{"3s", MinRequestGap},
		{"0", MinRequestGap},
		{"-5s", MinRequestGap},
		{"fast", MinRequestGap},
	}
	for _, tc := range cases {
		t.Setenv("UBER_JOBS_MIN_GAP", tc.env)
		g := NewGate("/nonexistent")
		if g.Gap != tc.want {
			t.Errorf("UBER_JOBS_MIN_GAP=%q: Gap = %v, want %v", tc.env, g.Gap, tc.want)
		}
		if g.Gap < 3*time.Second {
			t.Errorf("UBER_JOBS_MIN_GAP=%q: Gap = %v is below the owner's 3s floor", tc.env, g.Gap)
		}
		if g.Dir != "/nonexistent" {
			t.Errorf("Dir = %q", g.Dir)
		}
	}
	if MinRequestGap < 3*time.Second {
		t.Errorf("MinRequestGap = %v, below the owner's 3s floor", MinRequestGap)
	}
}

// TestLatchUntil: a latch holds to 00:00 UTC of the next day by default;
// the cooldown env sets a duration with a one-minute floor.
func TestLatchUntil(t *testing.T) {
	at := time.Date(2026, 10, 5, 23, 59, 30, 0, time.UTC)
	midnight := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		env  string
		want time.Time
	}{
		{"", midnight},
		{"10s", at.Add(time.Minute)},
		{"59s", at.Add(time.Minute)},
		{"2h", at.Add(2 * time.Hour)},
		{"1d", at.Add(24 * time.Hour)},
		{"0", midnight},
		{"-5m", midnight},
		{"soon", midnight},
	}
	for _, tc := range cases {
		t.Setenv(RefusalCooldownEnv, tc.env)
		if got := latchUntil(at); !got.Equal(tc.want) {
			t.Errorf("%s=%q: latchUntil = %v, want %v", RefusalCooldownEnv, tc.env, got, tc.want)
		}
	}
	// A non-UTC refusal time still latches to the UTC day boundary.
	t.Setenv(RefusalCooldownEnv, "")
	karachi := time.FixedZone("PKT", 5*3600)
	local := time.Date(2026, 10, 6, 2, 0, 0, 0, karachi) // 2026-10-05T21:00Z
	if got := latchUntil(local); !got.Equal(midnight) {
		t.Errorf("latchUntil(%v) = %v, want %v", local, got, midnight)
	}
}

// TestRecordRefusalWritesLatch: the latch file carries the host, status,
// challenge flag, and an Until set by the cooldown rule.
func TestRecordRefusalWritesLatch(t *testing.T) {
	t.Setenv(RefusalCooldownEnv, "10s")
	dir := t.TempDir()
	r := newRefusal("https://jobs.uber.com/api/jobs/search/?page=1", "jobs.uber.com", 403, true, "<title>Just a moment")
	RecordRefusal(dir, r)

	raw, err := os.ReadFile(filepath.Join(dir, "refused-jobs.uber.com.json"))
	if err != nil {
		t.Fatalf("latch file: %v", err)
	}
	var l refusalLatch
	if err := json.Unmarshal(raw, &l); err != nil {
		t.Fatalf("latch JSON %q: %v", raw, err)
	}
	if l.Host != "jobs.uber.com" || l.Status != 403 || !l.Challenge || !strings.Contains(l.URL, "/api/jobs/search/") {
		t.Errorf("latch = %+v", l)
	}
	at, err1 := time.Parse(time.RFC3339, l.At)
	until, err2 := time.Parse(time.RFC3339, l.Until)
	if err1 != nil || err2 != nil || until.Sub(at) != time.Minute {
		t.Errorf("At=%q Until=%q, want Until one minute after At (10s floored to 1m)", l.At, l.Until)
	}
	log, _ := os.ReadFile(filepath.Join(dir, "refusals.tsv"))
	if !strings.Contains(string(log), "\t403\tchallenge=true") {
		t.Errorf("refusals.tsv = %q", log)
	}

	got := LatchedRefusal(dir, "jobs.uber.com", at)
	if got == nil || !got.Latched || got.Status != 403 || !got.Challenge {
		t.Fatalf("LatchedRefusal inside cooldown = %+v", got)
	}
	if LatchedRefusal(dir, "jobs.uber.com", until) != nil {
		t.Errorf("latch still active at its Until instant")
	}
	if LatchedRefusal(dir, "iaziqy.fa.ocs.oraclecloud.com", at) != nil {
		t.Errorf("latch leaked to another host")
	}
}

// TestRecordRefusalSkipsLatchedAndEmpty: re-recording a latched refusal
// must not extend the latch, and no dir or nil refusal writes nothing.
func TestRecordRefusalSkipsLatchedAndEmpty(t *testing.T) {
	dir := t.TempDir()
	r := newRefusal("u", "jobs.uber.com", 429, false, "")
	r.Latched = true
	RecordRefusal(dir, r)
	if _, err := os.Stat(latchPath(dir, "jobs.uber.com")); !os.IsNotExist(err) {
		t.Errorf("a latched refusal rewrote the latch file")
	}
	RecordRefusal("", newRefusal("u", "h", 403, false, ""))
	RecordRefusal(dir, nil)
	if LatchedRefusal("", "jobs.uber.com", time.Now()) != nil || LatchedRefusal(dir, "", time.Now()) != nil {
		t.Errorf("empty dir or host must never latch")
	}
}

// TestLatchPathSanitizesHost: host:port and odd bytes map to a safe file
// name inside the state dir, never a path traversal.
func TestLatchPathSanitizesHost(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"jobs.uber.com":     "refused-jobs.uber.com.json",
		"127.0.0.1:8080":    "refused-127.0.0.1_8080.json",
		"JOBS.Uber.com":     "refused-jobs.uber.com.json",
		"../../etc/passwd":  "refused-.._.._etc_passwd.json",
		"a/b\\c":            "refused-a_b_c.json",
		"iaziqy.fa.ocs.com": "refused-iaziqy.fa.ocs.com.json",
	}
	for host, want := range cases {
		got := latchPath(dir, host)
		if got != filepath.Join(dir, want) {
			t.Errorf("latchPath(%q) = %q, want %q", host, got, filepath.Join(dir, want))
		}
		if filepath.Dir(got) != dir {
			t.Errorf("latchPath(%q) escapes the state dir: %q", host, got)
		}
	}
}

// TestRefusalErrorMessages pins the two message shapes the CLI prints.
func TestRefusalErrorMessages(t *testing.T) {
	r := newRefusal("https://jobs.uber.com/x", "jobs.uber.com", 429, false, "slow down")
	if got := r.Error(); got != "jobs.uber.com refused the request with HTTP 429; not retrying" {
		t.Errorf("Error() = %q", got)
	}
	c := newRefusal("u", "jobs.uber.com", 200, true, "")
	c.Latched, c.LatchedAt, c.Until, c.LatchFile = true, "2026-10-05T08:00:00Z", "2026-10-06T00:00:00Z", "/state/refused-jobs.uber.com.json"
	want := "jobs.uber.com refused a request at 2026-10-05T08:00:00Z with a bot-protection challenge (HTTP 200); nothing is sent to it until 2026-10-06T00:00:00Z (delete /state/refused-jobs.uber.com.json to allow a request sooner)"
	if got := c.Error(); got != want {
		t.Errorf("latched Error() = %q, want %q", got, want)
	}
}
