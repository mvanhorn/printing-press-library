// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"testing"
	"time"
)

// captchaErr is an error whose message trips isCaptchaRequired.
var captchaErr = errors.New(`HTTP 422: {"error_type": "token_validation_failed"}`)

// fakeClock drives retryOnGate without real sleeping: sleep advances now().
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time { return f.t }
func (f *fakeClock) sleep(_ context.Context, d time.Duration) error {
	f.t = f.t.Add(d)
	return nil
}

func baseGateCfg(fc *fakeClock, enabled bool) gateRetryConfig {
	return gateRetryConfig{
		enabled:        enabled,
		timeout:        30 * time.Minute,
		initialBackoff: 30 * time.Second,
		maxBackoff:     5 * time.Minute,
		now:            fc.now,
		sleep:          fc.sleep,
	}
}

func TestRetryOnGate_DisabledSingleAttempt(t *testing.T) {
	fc := &fakeClock{t: time.Unix(0, 0)}
	calls := 0
	submit := func() (*sunoGenerateResponse, error) {
		calls++
		return nil, captchaErr
	}
	_, err := retryOnGate(context.Background(), baseGateCfg(fc, false), submit)
	if calls != 1 {
		t.Errorf("submit called %d times, want 1 (retry disabled)", calls)
	}
	if !isCaptchaRequired(err) {
		t.Errorf("want captcha error returned, got %v", err)
	}
}

func TestRetryOnGate_EnabledSucceedsOnSecond(t *testing.T) {
	fc := &fakeClock{t: time.Unix(0, 0)}
	calls := 0
	submit := func() (*sunoGenerateResponse, error) {
		calls++
		if calls >= 2 {
			return &sunoGenerateResponse{Status: "ok"}, nil
		}
		return nil, captchaErr
	}
	resp, err := retryOnGate(context.Background(), baseGateCfg(fc, true), submit)
	if err != nil {
		t.Fatalf("want success after retry, got err %v", err)
	}
	if resp == nil || resp.Status != "ok" {
		t.Errorf("resp = %v, want status ok", resp)
	}
	if calls != 2 {
		t.Errorf("submit called %d times, want 2", calls)
	}
}

func TestRetryOnGate_TimesOutOnPersistentGate(t *testing.T) {
	fc := &fakeClock{t: time.Unix(0, 0)}
	calls := 0
	submit := func() (*sunoGenerateResponse, error) {
		calls++
		return nil, captchaErr
	}
	_, err := retryOnGate(context.Background(), baseGateCfg(fc, true), submit)
	if !isCaptchaRequired(err) {
		t.Errorf("want last captcha error after timeout, got %v", err)
	}
	if calls < 2 {
		t.Errorf("want multiple attempts before timeout, got %d", calls)
	}
	if calls > 100 {
		t.Errorf("attempts unbounded (%d) — backoff/deadline not enforced", calls)
	}
	// The fake clock must have advanced at least one full timeout window.
	if fc.t.Before(time.Unix(0, 0).Add(30 * time.Minute)) {
		t.Errorf("clock advanced to %v, expected past the 30m deadline", fc.t)
	}
}

func TestRetryOnGate_NonCaptchaErrorNotRetried(t *testing.T) {
	fc := &fakeClock{t: time.Unix(0, 0)}
	calls := 0
	other := errors.New("HTTP 401: Unauthorized")
	submit := func() (*sunoGenerateResponse, error) {
		calls++
		return nil, other
	}
	_, err := retryOnGate(context.Background(), baseGateCfg(fc, true), submit)
	if calls != 1 {
		t.Errorf("submit called %d times, want 1 (non-captcha error must not retry)", calls)
	}
	if !errors.Is(err, other) {
		t.Errorf("want the original 401 error surfaced, got %v", err)
	}
}

func TestRetryOnGate_ContextCancellationStops(t *testing.T) {
	fc := &fakeClock{t: time.Unix(0, 0)}
	calls := 0
	submit := func() (*sunoGenerateResponse, error) {
		calls++
		return nil, captchaErr
	}
	cfg := baseGateCfg(fc, true)
	cfg.sleep = func(_ context.Context, _ time.Duration) error {
		return context.Canceled
	}
	_, err := retryOnGate(context.Background(), cfg, submit)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Errorf("submit called %d times, want 1 (cancelled during first backoff)", calls)
	}
}
