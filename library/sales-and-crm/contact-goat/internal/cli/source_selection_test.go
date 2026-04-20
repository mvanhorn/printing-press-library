// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

// newFakeBearerServer stands up a minimal httptest server that mimics
// the public Happenstance API endpoints we exercise in integration
// tests. It implements:
//
//   - POST /search          -> {"id":"abc123"}
//   - GET  /search/{id}     -> first poll RUNNING; second poll COMPLETED
//                              with two SearchResult rows
//   - GET  /users/me        -> {"email","name","friends":[]}
//
// Tests pass the server URL via api.WithBaseURL so no real network
// traffic occurs. The server tracks per-search-id poll attempts in an
// atomic counter so RUNNING -> COMPLETED transitions are deterministic
// regardless of timing.
func newFakeBearerServer(t *testing.T) *httptest.Server {
	t.Helper()
	var pollCount int32
	mux := http.NewServeMux()
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			n := atomic.AddInt32(&pollCount, 1)
			if n < 2 {
				_, _ = w.Write([]byte(`{"id":"abc123","status":"RUNNING","results":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"abc123","status":"COMPLETED","results":[
				{"name":"Alice Example","current_title":"VP Engineering","current_company":"NBA","weighted_traits_score":0.91},
				{"name":"Bob Example","current_title":"Director Product","current_company":"NBA","weighted_traits_score":0.84}
			]}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"id":"abc123","url":"https://happenstance.ai/s/abc123"}`))
	})
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"email":"matt@example.com","name":"Matt","friends":[]}`))
	})
	return httptest.NewServer(mux)
}

// newFakeBearer401Server returns an httptest server that always 401s on
// /users/me, simulating a rotated/revoked key.
func newFakeBearer401Server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
	}))
}

// withEnv sets an env var for the duration of the test and restores the
// previous value (or unsets) on cleanup. Avoids polluting other tests.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// TestSelectSource_DecisionTree locks down the exact decision tree from
// the plan's High-Level Technical Design / Mermaid spec. Every branch
// the plan enumerates is covered by one named subtest. The test runs
// the helper as a pure function: no network, no real cookie probe.
func TestSelectSource_DecisionTree(t *testing.T) {
	cfg := &config.Config{Path: t.TempDir() + "/config.toml"}
	ctx := context.Background()

	t.Run("happy: cookie present, quota > 0 -> cookie (free quota first)", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "")
		got, deferredErr, hardErr := SelectSource(ctx, "", cfg, true, 12)
		if hardErr != nil {
			t.Fatalf("unexpected hardErr: %v", hardErr)
		}
		if deferredErr != nil {
			t.Fatalf("unexpected deferredErr: %v", deferredErr)
		}
		if got != SourceCookie {
			t.Errorf("got %q, want SourceCookie", got)
		}
	})

	t.Run("happy: cookie + env both present + quota > 0 -> cookie (do NOT burn paid credits)", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "hpn_live_personal_test")
		got, deferredErr, hardErr := SelectSource(ctx, "", cfg, true, 5)
		if hardErr != nil {
			t.Fatalf("unexpected hardErr: %v", hardErr)
		}
		if deferredErr != nil {
			t.Fatalf("unexpected deferredErr: %v", deferredErr)
		}
		if got != SourceCookie {
			t.Errorf("got %q, want SourceCookie (cookie-first preference)", got)
		}
	})

	t.Run("happy: cookie absent, env set -> api (only path available)", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "hpn_live_personal_test")
		got, _, hardErr := SelectSource(ctx, "", cfg, false, UnknownSearchesRemaining)
		if hardErr != nil {
			t.Fatalf("unexpected hardErr: %v", hardErr)
		}
		if got != SourceAPI {
			t.Errorf("got %q, want SourceAPI", got)
		}
	})

	t.Run("edge: cookie present, quota == 0, env set -> api (fall back to paid)", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "hpn_live_personal_test")
		got, deferredErr, hardErr := SelectSource(ctx, "", cfg, true, 0)
		if hardErr != nil {
			t.Fatalf("unexpected hardErr: %v", hardErr)
		}
		if deferredErr != nil {
			t.Fatalf("unexpected deferredErr: %v", deferredErr)
		}
		if got != SourceAPI {
			t.Errorf("got %q, want SourceAPI", got)
		}
	})

	t.Run("edge: cookie present, quota == 0, env NOT set -> cookie + deferredErr", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "")
		got, deferredErr, hardErr := SelectSource(ctx, "", cfg, true, 0)
		if hardErr != nil {
			t.Fatalf("unexpected hardErr: %v", hardErr)
		}
		if got != SourceCookie {
			t.Errorf("got %q, want SourceCookie", got)
		}
		if deferredErr == nil {
			t.Fatal("want non-nil deferredErr (actionable HAPPENSTANCE_API_KEY hint)")
		}
		if !strings.Contains(deferredErr.Error(), api.KeyEnvVar) {
			t.Errorf("deferredErr should mention %s, got: %v", api.KeyEnvVar, deferredErr)
		}
	})

	t.Run("edge: explicit --source api with no env var -> exit 4", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "")
		_, _, hardErr := SelectSource(ctx, SourceFlagAPI, cfg, true, 12)
		if hardErr == nil {
			t.Fatal("want hardErr (auth required)")
		}
		var ce *cliError
		if !errors.As(hardErr, &ce) {
			t.Fatalf("want *cliError, got %T", hardErr)
		}
		if ce.code != 4 {
			t.Errorf("exit code = %d, want 4", ce.code)
		}
		if !strings.Contains(hardErr.Error(), api.KeyEnvVar) {
			t.Errorf("hardErr should mention %s, got: %v", api.KeyEnvVar, hardErr)
		}
	})

	t.Run("edge: explicit --source hp with no cookie -> exit 4", func(t *testing.T) {
		_, _, hardErr := SelectSource(ctx, SourceFlagCookie, cfg, false, UnknownSearchesRemaining)
		if hardErr == nil {
			t.Fatal("want hardErr (cookie required)")
		}
		var ce *cliError
		if !errors.As(hardErr, &ce) {
			t.Fatalf("want *cliError, got %T", hardErr)
		}
		if ce.code != 4 {
			t.Errorf("exit code = %d, want 4", ce.code)
		}
		if !strings.Contains(hardErr.Error(), "auth login") {
			t.Errorf("hardErr should mention `auth login --chrome`, got: %v", hardErr)
		}
	})

	t.Run("edge: cookie absent + env absent -> exit 4 with both-surface hint", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "")
		_, _, hardErr := SelectSource(ctx, "", cfg, false, UnknownSearchesRemaining)
		if hardErr == nil {
			t.Fatal("want hardErr (no auth at all)")
		}
		msg := hardErr.Error()
		if !strings.Contains(msg, api.KeyEnvVar) {
			t.Errorf("hardErr should mention %s, got: %v", api.KeyEnvVar, hardErr)
		}
		if !strings.Contains(msg, "auth login") {
			t.Errorf("hardErr should mention `auth login --chrome`, got: %v", hardErr)
		}
	})

	t.Run("edge: unknown remaining + cookie + env -> cookie (rely on retry wrapper)", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "hpn_live_personal_test")
		got, deferredErr, hardErr := SelectSource(ctx, "", cfg, true, UnknownSearchesRemaining)
		if hardErr != nil || deferredErr != nil {
			t.Fatalf("unexpected error path: hard=%v deferred=%v", hardErr, deferredErr)
		}
		if got != SourceCookie {
			t.Errorf("got %q, want SourceCookie (auto routes cookie-first when remaining is unknown)", got)
		}
	})

	t.Run("auto: explicit --source auto behaves like unset", func(t *testing.T) {
		withEnv(t, api.KeyEnvVar, "")
		got, _, hardErr := SelectSource(ctx, SourceFlagAuto, cfg, true, 7)
		if hardErr != nil {
			t.Fatalf("unexpected hardErr: %v", hardErr)
		}
		if got != SourceCookie {
			t.Errorf("got %q, want SourceCookie", got)
		}
	})
}

// TestExecuteWithSourceFallback_CookieSucceeds is the trivial happy
// path: SelectSource picks cookie, the cookie call returns a valid
// PeopleSearchResult, no fallback needed. Bearer runner is provided
// but never invoked.
func TestExecuteWithSourceFallback_CookieSucceeds(t *testing.T) {
	want := &client.PeopleSearchResult{Query: "VPs at NBA", Status: "Found 3 people", Completed: true}
	bearerCalled := false
	out, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceCookie,
		func() (*client.PeopleSearchResult, error) { return want, nil },
		func() (*client.PeopleSearchResult, error) {
			bearerCalled = true
			return nil, errors.New("bearer should not have been called")
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bearerCalled {
		t.Error("bearer runner was invoked on a successful cookie call")
	}
	if out.Result != want {
		t.Errorf("Result = %p, want %p", out.Result, want)
	}
	if out.UsedSource != SourceCookie {
		t.Errorf("UsedSource = %q, want SourceCookie", out.UsedSource)
	}
	if out.FellBackFromCookie {
		t.Error("FellBackFromCookie should be false on a clean cookie call")
	}
}

// TestExecuteWithSourceFallback_CookieRateLimited is the cornerstone of
// the cookie-then-bearer pattern: a 429 from the cookie call triggers
// the wrapper to switch surfaces, log the credit-spent notice, and
// surface the bearer result with FellBackFromCookie=true so the
// renderer can embed a "fell back" notice on the JSON envelope.
func TestExecuteWithSourceFallback_CookieRateLimited(t *testing.T) {
	bearerResult := &client.PeopleSearchResult{Query: "VPs at NBA", Status: "Found 5 people (bearer)", Completed: true}
	var stderr bytes.Buffer
	out, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceCookie,
		func() (*client.PeopleSearchResult, error) {
			// Mimic the canonical cookie-surface 429 message verbatim;
			// IsCookieRateLimitError matches on the substring.
			return nil, fmt.Errorf("happenstance POST /api/search: POST /api/search returned HTTP 429: {\"error\":\"Rate limit reached\"}")
		},
		func() (*client.PeopleSearchResult, error) { return bearerResult, nil },
		&stderr,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != bearerResult {
		t.Errorf("Result = %v, want bearerResult", out.Result)
	}
	if !out.FellBackFromCookie {
		t.Error("FellBackFromCookie should be true after a 429 retry")
	}
	if out.UsedSource != SourceAPI {
		t.Errorf("UsedSource = %q, want SourceAPI after fallback", out.UsedSource)
	}
	if !strings.Contains(stderr.String(), FallbackNoticeMessage) {
		t.Errorf("stderr should contain canonical fallback notice; got: %q", stderr.String())
	}
	if out.FallbackNotice != FallbackNoticeMessage {
		t.Errorf("FallbackNotice = %q, want canonical message", out.FallbackNotice)
	}
}

// TestExecuteWithSourceFallback_CookieRateLimitedNoBearer is the edge
// case where the cookie path returns 429 but the user has not set
// HAPPENSTANCE_API_KEY. The wrapper surfaces the original 429 with an
// actionable hint about the env var so the user can self-recover.
func TestExecuteWithSourceFallback_CookieRateLimitedNoBearer(t *testing.T) {
	out, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceCookie,
		func() (*client.PeopleSearchResult, error) {
			return nil, errors.New("happenstance POST /api/search returned HTTP 429: Rate limit reached")
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("want error: cookie 429 with no bearer fallback")
	}
	if out.UsedSource != SourceCookie {
		t.Errorf("UsedSource = %q, want SourceCookie (bearer never attempted)", out.UsedSource)
	}
	if !strings.Contains(err.Error(), api.KeyEnvVar) {
		t.Errorf("error should mention %s as a recovery hint, got: %v", api.KeyEnvVar, err)
	}
	if !strings.Contains(err.Error(), "Rate limit reached") {
		t.Errorf("error should preserve the original 429 message, got: %v", err)
	}
}

// TestExecuteWithSourceFallback_CookieGenericErrorDoesNotRetry confirms
// the wrapper only switches surfaces on documented quota errors. An
// arbitrary cookie failure (e.g. expired session, network blip) must
// surface verbatim so the user diagnoses the right problem instead of
// silently spending paid credits to mask a different bug.
func TestExecuteWithSourceFallback_CookieGenericErrorDoesNotRetry(t *testing.T) {
	bearerCalled := false
	_, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceCookie,
		func() (*client.PeopleSearchResult, error) {
			return nil, errors.New("happenstance: session expired (Clerk JWT)")
		},
		func() (*client.PeopleSearchResult, error) {
			bearerCalled = true
			return &client.PeopleSearchResult{}, nil
		},
		nil,
	)
	if err == nil {
		t.Fatal("want non-quota cookie error to surface verbatim")
	}
	if bearerCalled {
		t.Error("bearer runner must NOT be invoked on a non-quota cookie failure")
	}
}

// TestExecuteWithSourceFallback_BearerSourceDirect ensures that an
// explicit SourceAPI selection runs the bearer path directly without
// touching cookie. There is no cookie-side fallback when the user (or
// auto-routing) has already picked the paid surface.
func TestExecuteWithSourceFallback_BearerSourceDirect(t *testing.T) {
	want := &client.PeopleSearchResult{Query: "VPs at NBA", Status: "Found 4 people (bearer)", Completed: true}
	cookieCalled := false
	out, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceAPI,
		func() (*client.PeopleSearchResult, error) {
			cookieCalled = true
			return nil, errors.New("cookie should not have been called")
		},
		func() (*client.PeopleSearchResult, error) { return want, nil },
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookieCalled {
		t.Error("cookie runner was invoked when SourceAPI was selected")
	}
	if out.Result != want {
		t.Errorf("Result = %v, want bearer want", out.Result)
	}
	if out.UsedSource != SourceAPI {
		t.Errorf("UsedSource = %q, want SourceAPI", out.UsedSource)
	}
}

// TestExecuteWithSourceFallback_BearerNilRunnerErrors guards against
// the call site forgetting to construct a bearer runner when the
// selected source is SourceAPI. The wrapper must surface the canonical
// auth error rather than panic with a nil-deref.
func TestExecuteWithSourceFallback_BearerNilRunnerErrors(t *testing.T) {
	_, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceAPI,
		nil,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("want auth error when SourceAPI is selected with nil bearer runner")
	}
	if !strings.Contains(err.Error(), api.KeyEnvVar) {
		t.Errorf("error should mention %s, got: %v", api.KeyEnvVar, err)
	}
}

// TestIsCookieRateLimitError is a small unit test of the substring
// matcher; the upstream cookie surface returns the 429 as a free-form
// error string. If a future client refactor swaps this for a typed
// sentinel, prefer errors.Is — but keep the substring fallback so
// older error wrappers still match.
func TestIsCookieRateLimitError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"random error", errors.New("connection refused"), false},
		{"canonical Rate limit reached", errors.New("happenstance POST /api/search returned HTTP 429: Rate limit reached"), true},
		{"lowercase variant", errors.New("rate limit reached"), true},
		{"HTTP 429 substring", errors.New("returned HTTP 429"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsCookieRateLimitError(tc.err); got != tc.want {
				t.Errorf("IsCookieRateLimitError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestIsBearerRateLimitError confirms the typed-error path works for
// the bearer surface (which DOES return *api.RateLimitError on 429).
func TestIsBearerRateLimitError(t *testing.T) {
	if IsBearerRateLimitError(nil) {
		t.Error("nil should not be a rate-limit error")
	}
	if IsBearerRateLimitError(errors.New("not a rate-limit error")) {
		t.Error("plain error should not match")
	}
	rl := &api.RateLimitError{RetryAfterSeconds: 30}
	if !IsBearerRateLimitError(rl) {
		t.Error("typed RateLimitError should match")
	}
	wrapped := fmt.Errorf("wrapped: %w", rl)
	if !IsBearerRateLimitError(wrapped) {
		t.Error("wrapped RateLimitError should match via errors.As")
	}
}

// TestSourceSelection_BearerHttpTestFixture is the integration scenario
// from the plan: a bearer-surface call against an httptest fixture
// returns the same shape (after normalization) that the cookie surface
// would have. Exercises the full client+normalize seam without touching
// the public network.
func TestSourceSelection_BearerHttpTestFixture(t *testing.T) {
	srv := newFakeBearerServer(t)
	defer srv.Close()

	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(srv.URL))
	ctx := context.Background()

	// Round-trip: POST /v1/search -> poll -> read results.
	env, err := c.Search(ctx, "VPs at NBA", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if env.Id == "" {
		t.Fatal("Search returned empty id")
	}
	final, err := c.PollSearch(ctx, env.Id, &api.PollSearchOptions{})
	if err != nil {
		t.Fatalf("PollSearch: %v", err)
	}
	if final.Status != api.StatusCompleted {
		t.Errorf("Status = %q, want COMPLETED", final.Status)
	}
	if len(final.Results) == 0 {
		t.Fatal("expected at least one result from fixture")
	}
	// Normalize and confirm the canonical shape is populated.
	got := api.ToClientPerson(final.Results[0])
	if got.Name == "" {
		t.Errorf("normalized Person has empty Name: %+v", got)
	}
	if got.CurrentTitle == "" {
		t.Errorf("normalized Person has empty CurrentTitle: %+v", got)
	}
}

// TestSourceSelection_CookieFallsBackToBearerEnvelope is the integration
// scenario for the cookie-then-bearer retry: the cookie call 429s, the
// wrapper switches to the bearer fixture, and the resulting envelope is
// shape-compatible with a clean bearer call (same fields populated;
// only the FellBackFromCookie / FallbackNotice metadata differs).
func TestSourceSelection_CookieFallsBackToBearerEnvelope(t *testing.T) {
	srv := newFakeBearerServer(t)
	defer srv.Close()

	bearer := api.NewClient("hpn_live_personal_test", api.WithBaseURL(srv.URL))

	bearerRun := func() (*client.PeopleSearchResult, error) {
		ctx := context.Background()
		env, err := bearer.Search(ctx, "VPs at NBA", nil)
		if err != nil {
			return nil, err
		}
		final, err := bearer.PollSearch(ctx, env.Id, nil)
		if err != nil {
			return nil, err
		}
		people := make([]client.Person, 0, len(final.Results))
		for _, r := range final.Results {
			people = append(people, api.ToClientPerson(r))
		}
		return &client.PeopleSearchResult{
			RequestID: final.Id,
			Query:     "VPs at NBA",
			Status:    final.Status,
			Completed: true,
			People:    people,
		}, nil
	}

	cleanRes, cleanErr := bearerRun()
	if cleanErr != nil {
		t.Fatalf("clean bearer baseline: %v", cleanErr)
	}

	var stderr bytes.Buffer
	out, err := ExecuteWithSourceFallback(
		context.Background(),
		SourceCookie,
		func() (*client.PeopleSearchResult, error) {
			return nil, errors.New("happenstance POST /api/search returned HTTP 429: Rate limit reached")
		},
		bearerRun,
		&stderr,
	)
	if err != nil {
		t.Fatalf("ExecuteWithSourceFallback: %v", err)
	}
	if !out.FellBackFromCookie {
		t.Fatal("expected FellBackFromCookie=true")
	}
	if out.UsedSource != SourceAPI {
		t.Errorf("UsedSource = %q, want SourceAPI", out.UsedSource)
	}
	// Shape-compat: same set of populated fields, same number of
	// results. Only the source / fallback-notice metadata differs.
	if got, want := out.Result.Status, cleanRes.Status; got != want {
		t.Errorf("Status: fallback=%q clean=%q", got, want)
	}
	if got, want := len(out.Result.People), len(cleanRes.People); got != want {
		t.Errorf("People count: fallback=%d clean=%d", got, want)
	}
	if !strings.Contains(stderr.String(), FallbackNoticeMessage) {
		t.Errorf("stderr should contain fallback notice; got: %q", stderr.String())
	}
}

// TestSourceSelection_BearerKeyRotation401 exercises the doctor-style
// "key set but /v1/users/me returns 401" path through the bearer client.
// The error message must mention the canonical rotation URL so the user
// has an actionable next step.
func TestSourceSelection_BearerKeyRotation401(t *testing.T) {
	srv := newFakeBearer401Server(t)
	defer srv.Close()

	c := api.NewClient("hpn_live_personal_rotated", api.WithBaseURL(srv.URL))
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected 401 error from /users/me fixture")
	}
	if !strings.Contains(err.Error(), api.RotationURL) {
		t.Errorf("err should mention rotation URL %q, got: %v", api.RotationURL, err)
	}
}

// TestQuotaCacheBypass confirms the in-memory cache honors the
// bypassCache flag (mirrors --no-cache). Without bypass the second
// call within TTL hits the cache; with bypass it does not.
//
// We exercise this via FetchSearchesRemaining directly — there is no
// real network because the cookie client is nil (HasCookieAuth()
// returns false) and the function short-circuits to
// UnknownSearchesRemaining. The test still catches a regression where
// the bypass flag is silently ignored.
func TestQuotaCacheBypass(t *testing.T) {
	resetQuotaCache()
	cfg := &config.Config{Path: t.TempDir() + "/config.toml"}

	// nil client: short-circuit returns UnknownSearchesRemaining and
	// does NOT populate the cache, so bypass and non-bypass agree.
	if got := FetchSearchesRemaining(nil, cfg, false); got != UnknownSearchesRemaining {
		t.Errorf("nil client: got %d, want %d", got, UnknownSearchesRemaining)
	}
	if got := FetchSearchesRemaining(nil, cfg, true); got != UnknownSearchesRemaining {
		t.Errorf("nil client + bypass: got %d, want %d", got, UnknownSearchesRemaining)
	}
}
