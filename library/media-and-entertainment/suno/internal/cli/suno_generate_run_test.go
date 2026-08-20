package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/captcha"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
	"github.com/spf13/cobra"
)

type stubSolver struct {
	tok string
	err error
}

func (s stubSolver) Solve(_ context.Context, _ captcha.Options) (string, error) {
	return s.tok, s.err
}

func TestCaptchaGateAction(t *testing.T) {
	gateErr := errors.New(`generate returned http 422: {"detail":"token_validation_failed"}`)
	nullToken500 := &client.APIError{Method: "POST", Path: "/api/generate/v2-web/", StatusCode: 500, Body: "server_error"}
	otherErr := errors.New("generate returned http 503: service unavailable")

	cases := []struct {
		name        string
		err         error
		tokenWasNil bool
		noCaptcha   bool
		want        captchaAction
	}{
		{"gate error, solver enabled -> solve", gateErr, false, false, captchaSolve},
		{"gate error, --no-captcha -> suppressed", gateErr, false, true, captchaSuppressed},
		{"null-token 500, solver enabled -> solve", nullToken500, true, false, captchaSolve},
		{"null-token 500, --no-captcha -> suppressed", nullToken500, true, true, captchaSuppressed},
		{"non-gate error -> proceed", otherErr, false, false, captchaProceed},
		{"non-gate error, --no-captcha -> proceed", otherErr, false, true, captchaProceed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := captchaGateAction(tc.err, tc.tokenWasNil, tc.noCaptcha); got != tc.want {
				t.Fatalf("captchaGateAction = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHandleCaptchaGate_Success_ReturnsToken(t *testing.T) {
	prev := defaultSolver
	defaultSolver = stubSolver{tok: "P1_tok"}
	defer func() { defaultSolver = prev }()

	tok, err := handleCaptchaGate(context.Background(), "", true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tok != "P1_tok" {
		t.Fatalf("token=%q", tok)
	}
}

func TestHandleCaptchaGate_InteractiveRequired_PropagatesSentinel(t *testing.T) {
	prev := defaultSolver
	defaultSolver = stubSolver{err: captcha.ErrInteractiveRequired}
	defer func() { defaultSolver = prev }()

	_, err := handleCaptchaGate(context.Background(), "", false)
	if !errors.Is(err, captcha.ErrInteractiveRequired) {
		t.Fatalf("want ErrInteractiveRequired propagated, got %v", err)
	}
}

func TestCaptchaGateFailure_AgentEmitsEnvelopeOnStdout(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	flags := &rootFlags{asJSON: true}

	err := captchaGateFailure(cmd, flags)
	if ExitCode(err) != 2 {
		t.Fatalf("exit code = %d, want 2", ExitCode(err))
	}
	var env map[string]any
	if jerr := json.Unmarshal(stdout.Bytes(), &env); jerr != nil {
		t.Fatalf("stdout not JSON: %q (%v)", stdout.String(), jerr)
	}
	if env["error_type"] != "captcha_required" {
		t.Fatalf("error_type = %v, want captcha_required", env["error_type"])
	}
	if env["retriable"] != true {
		t.Fatalf("retriable = %v, want true", env["retriable"])
	}
}

func TestCaptchaGateFailure_NonJSON_NoEnvelope(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	flags := &rootFlags{asJSON: false}

	_ = captchaGateFailure(cmd, flags)
	if stdout.Len() != 0 {
		t.Fatalf("non-JSON mode must not emit envelope to stdout, got %q", stdout.String())
	}
}

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

// TestIsCaptchaRequired_DistinguishesBodyValidation confirms the gate detector
// fires on token_validation_failed but NOT on a params.title body-validation
// 422 (the U1 bug shape), so the two never get conflated.
func TestIsCaptchaRequired_DistinguishesBodyValidation(t *testing.T) {
	if !isCaptchaRequired(captchaErr) {
		t.Errorf("token_validation_failed should be detected as captcha gate")
	}
	bodyValidation := errors.New(`HTTP 422: [{"loc":["body","params","title"],"msg":"Input should be a valid string"}]`)
	if isCaptchaRequired(bodyValidation) {
		t.Errorf("a params.title body-validation 422 must NOT be treated as the captcha gate")
	}
	if isCaptchaRequired(errors.New("HTTP 401: Unauthorized")) {
		t.Errorf("a 401 must NOT be treated as the captcha gate")
	}
}

// TestCaptchaGateError_AgentModeEmitsStructuredEnvelope verifies the passive
// gate path's JSON envelope on stdout, no envelope in human mode, and exit 2 in
// both modes.
func TestCaptchaGateError_AgentModeEmitsStructuredEnvelope(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := captchaGateError(cmd, &rootFlags{asJSON: true})
	if ExitCode(err) != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", ExitCode(err))
	}
	var env map[string]any
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatalf("agent-mode stdout is not JSON: %v (%q)", jerr, out.String())
	}
	if env["error_type"] != "captcha_required" {
		t.Errorf("error_type = %v, want captcha_required", env["error_type"])
	}
	if env["retriable"] != true {
		t.Errorf("retriable = %v, want true", env["retriable"])
	}

	// Human mode: no JSON envelope on stdout, still exit code 2.
	var hout bytes.Buffer
	hcmd := &cobra.Command{}
	hcmd.SetOut(&hout)
	herr := captchaGateError(hcmd, &rootFlags{asJSON: false})
	if ExitCode(herr) != 2 {
		t.Errorf("human-mode exit code = %d, want 2", ExitCode(herr))
	}
	if hout.Len() != 0 {
		t.Errorf("human mode must not write a JSON envelope to stdout, got %q", hout.String())
	}
}
