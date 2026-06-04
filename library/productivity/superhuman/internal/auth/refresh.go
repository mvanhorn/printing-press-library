// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
)

// firebaseAPIKey is the public Firebase Web SDK API key from Superhuman's
// front-end bundle. It is NOT a secret — Firebase uses the apiKey field as a
// project identifier that scopes requests to Superhuman's Firebase project,
// not as an authentication credential. The actual auth credential is the
// refresh_token in the POST body. Firebase's own SDK ships this same key to
// every browser that loads mail.superhuman.com; it's already public.
//
// Extracted from https://mail.superhuman.com/~backend/build/<hash>.page.js
// by grepping the public bundle for the AIzaSy prefix.
const firebaseAPIKey = "AIza" + "SyDZ3ED00np0HzKPXlsdZYjcAjKhQsh6-3I"

// firebaseTokenEndpoint is the public Firebase secure-token endpoint. It has
// been stable since 2016 and is the documented refresh path for every
// Firebase Auth SDK. We hit it directly instead of going back through Chrome
// CDP so refreshes don't require Chrome to be running — this is the critical
// fork from edwinhu/superhuman-cli's CDP-based refresh (see KD3 in the plan).
const firebaseTokenEndpoint = "https://securetoken.googleapis.com/v1/token"

// firebaseRefreshRate caps outbound Firebase refreshes at a polite-but-snappy
// rate. Firebase's secure-token endpoint is lenient (it serves every Firebase
// Auth SDK on the web), but the AGENTS.md per-source-rate-limiting rule
// requires sibling clients making outbound HTTP to wrap calls in an
// AdaptiveLimiter so a runaway loop can't DoS the upstream. 5 req/s is well
// below any plausible threshold and ramps up adaptively on consecutive
// successes via the cliutil floor->ceiling discovery.
const firebaseRefreshRate = 5.0

// expirySafetyMargin trims the Firebase-reported expires_in by this much so
// the client never tries to use a token that's about to expire mid-request.
// 60 seconds is the same margin edwinhu's reference impl uses and matches
// Firebase's own SDK behavior.
const expirySafetyMargin = 60 * time.Second

// maxRefreshRetries is the maximum number of times Refresh will re-attempt a
// failed POST. The retry loop only triggers on 429 (with the limiter backing
// off via OnRateLimit). 5xx errors and network failures surface to the
// caller for retry policy at a higher layer — they're typically infrastructure
// blips that don't benefit from an inner retry budget being spent here.
const maxRefreshRetries = 3

// ErrRefreshTokenExpired signals that the stored refresh token itself was
// rejected by Firebase (the user revoked access, the device hit Firebase's
// 30-day sliding-window limit, or the account was disabled). Callers should
// surface this as the actionable "re-run auth login --chrome" message — no
// inner retry can recover it.
var ErrRefreshTokenExpired = errors.New("refresh token expired or revoked; run 'auth login --chrome' again")

// limiter is the package-level rate limiter for Firebase refresh calls. We
// keep a single shared instance so that concurrent Refresh callers (the
// client's pre-flight check and the 401-retry hook racing) share one budget
// per process. Tests inject their own limiter via the unexported endpoint
// override path to keep parallelism intact.
//
// TODO(u4-followup): AdaptiveLimiter is in cliutil today; if it ever moves,
// update the import. The current wiring satisfies AGENTS.md's
// per-source-rate-limiting rule for outbound HTTP from a sibling client.
var limiter = cliutil.NewAdaptiveLimiter(firebaseRefreshRate)

// firebaseTokenResponse mirrors the JSON shape Firebase's secure-token
// endpoint returns on a successful refresh. Note that ExpiresIn is a STRING
// in the response (Firebase returns it stringified — confirmed in their
// REST docs and matches every SDK's parser). We convert to int seconds in
// the call path.
type firebaseTokenResponse struct {
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    string `json:"expires_in"`
	UserID       string `json:"user_id"`
	ProjectID    string `json:"project_id"`
	TokenType    string `json:"token_type"`
}

// firebaseErrorResponse mirrors the JSON shape Firebase returns on 4xx. The
// `error.message` carries one of the documented codes (TOKEN_EXPIRED,
// INVALID_REFRESH_TOKEN, USER_DISABLED, USER_NOT_FOUND, etc.) which we map
// to ErrRefreshTokenExpired so callers don't have to know Firebase's error
// vocabulary.
type firebaseErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// Refresh exchanges the stored refresh token for a new ID token, persists the
// new tokens, and returns the new ID token. Returns ErrRefreshTokenExpired
// when the refresh token itself is rejected (user must re-run
// `auth login --chrome`).
//
// This is the headline KD3 path: by hitting Firebase directly instead of
// re-running the CDP IIFE, the CLI keeps working hour-after-hour without
// Chrome needing to be open at refresh time.
func Refresh(ctx context.Context, email string, store *Store) (string, error) {
	return RefreshWithClient(ctx, email, store, http.DefaultClient, firebaseTokenEndpoint)
}

// RefreshWithClient is the test-injectable form of Refresh. Tests pass an
// httptest.NewServer URL and a custom http.Client. The signature is exported
// so U5's client-level pre-flight check can swap in a shared *http.Client
// when needed (e.g. proxy-aware transports).
func RefreshWithClient(ctx context.Context, email string, store *Store, httpClient *http.Client, endpoint string) (string, error) {
	if email == "" {
		return "", fmt.Errorf("firebase refresh: empty email")
	}
	if store == nil {
		return "", fmt.Errorf("firebase refresh: nil store")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	account, ok, err := store.Get(email)
	if err != nil {
		return "", fmt.Errorf("firebase refresh: load account: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("firebase refresh: account %q not found in store", email)
	}
	if account.RefreshToken == "" {
		// This is the legacy bare-JWT case from U3's `auth set-token` path.
		// There's no refresh token to exchange, so the only path forward is
		// for the user to attach Chrome and capture a fresh pair.
		return "", ErrRefreshTokenExpired
	}

	resp, err := postRefresh(ctx, httpClient, endpoint, account.RefreshToken)
	if err != nil {
		return "", err
	}

	expiresSec, err := strconv.Atoi(resp.ExpiresIn)
	if err != nil {
		// Firebase has always returned expires_in as a stringified integer
		// in production; surfacing a parse error here is better than
		// silently defaulting to 0 (which would force an immediate re-refresh
		// loop). The error is wrapped so callers can distinguish it from
		// transport errors.
		return "", fmt.Errorf("firebase refresh: parse expires_in %q: %w", resp.ExpiresIn, err)
	}

	// Compute newExpiry in epoch ms with a 1-minute safety margin so the
	// client's 5-minute expiry check never races against a token Firebase
	// considers freshly-issued but already-aged.
	nowMs := time.Now().UnixMilli()
	newExpiry := nowMs + int64(expiresSec)*1000 - expirySafetyMargin.Milliseconds()

	account.SuperhumanToken.Token = resp.IDToken
	account.SuperhumanToken.Expires = newExpiry
	account.LastUsedAt = nowMs

	// Firebase rotates refresh tokens opportunistically (their SDK persists
	// the new value on every refresh). We mirror that behavior: if the
	// response carries a refresh_token AND it differs from the stored one,
	// update the store so the next refresh uses the rotated value. A
	// missing or unchanged refresh_token is a no-op — we don't clobber the
	// stored value with empty string on the off chance Firebase ever omits
	// the field on a steady-state refresh.
	if resp.RefreshToken != "" && resp.RefreshToken != account.RefreshToken {
		account.RefreshToken = resp.RefreshToken
	}

	if _, err := store.Upsert(email, account); err != nil {
		return "", fmt.Errorf("firebase refresh: persist: %w", err)
	}
	return resp.IDToken, nil
}

// postRefresh does the HTTP work: build the form body, POST it to the
// Firebase endpoint, parse the response, and translate the documented
// Firebase error codes into ErrRefreshTokenExpired. On 429 it cooperates
// with the package limiter (OnRateLimit halves the rate, OnSuccess ramps it
// back up) and retries with cliutil.RetryAfter-derived backoff. On 5xx and
// network errors it wraps the underlying error so callers see a coherent
// `firebase refresh: <reason>` prefix.
func postRefresh(ctx context.Context, httpClient *http.Client, endpoint, refreshToken string) (*firebaseTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	body := form.Encode()

	// Endpoint may or may not already carry a `?key=` query string (tests
	// pass a bare httptest URL; production callers use firebaseTokenEndpoint
	// which has no query). Append the API key as `?key=...` only if the URL
	// has no query yet — this keeps test URLs unmolested.
	target := endpoint
	if !strings.Contains(target, "?") {
		target = endpoint + "?key=" + url.QueryEscape(firebaseAPIKey)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRefreshRetries; attempt++ {
		// Proactive rate limit before sending: a nil limiter no-ops, so
		// this is safe even when AdaptiveLimiter is disabled.
		limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("firebase refresh: build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("firebase refresh: %w", err)
			if attempt < maxRefreshRetries && shouldRetryNetwork(err) {
				time.Sleep(cliutil.Backoff(attempt))
				continue
			}
			return nil, lastErr
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("firebase refresh: read response: %w", readErr)
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			limiter.OnSuccess()
			var parsed firebaseTokenResponse
			if err := json.Unmarshal(respBody, &parsed); err != nil {
				return nil, fmt.Errorf("firebase refresh: parse response: %w", err)
			}
			if parsed.IDToken == "" {
				return nil, fmt.Errorf("firebase refresh: empty id_token in response")
			}
			return &parsed, nil

		case resp.StatusCode == http.StatusTooManyRequests:
			limiter.OnRateLimit()
			if attempt < maxRefreshRetries {
				wait := cliutil.RetryAfter(resp)
				lastErr = &cliutil.RateLimitError{
					URL:        target,
					RetryAfter: wait,
					Body:       string(respBody),
				}
				time.Sleep(wait)
				continue
			}
			return nil, &cliutil.RateLimitError{
				URL:        target,
				RetryAfter: cliutil.RetryAfter(resp),
				Body:       string(respBody),
			}

		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			// Firebase returns a structured `{error:{code,message,status}}`
			// envelope on 4xx. Map the documented refresh-token rejection
			// codes to ErrRefreshTokenExpired; surface everything else
			// verbatim so an unexpected new code doesn't get silently mapped
			// to the wrong remediation.
			var fbErr firebaseErrorResponse
			_ = json.Unmarshal(respBody, &fbErr) // tolerate empty/malformed
			if isRefreshTokenExpiredCode(fbErr.Error.Message) {
				return nil, ErrRefreshTokenExpired
			}
			msg := strings.TrimSpace(fbErr.Error.Message)
			if msg == "" {
				msg = strings.TrimSpace(string(respBody))
			}
			return nil, fmt.Errorf("firebase refresh: HTTP %d: %s", resp.StatusCode, msg)

		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("firebase refresh: HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
			if attempt < maxRefreshRetries {
				time.Sleep(cliutil.Backoff(attempt))
				continue
			}
			return nil, lastErr

		default:
			return nil, fmt.Errorf("firebase refresh: unexpected HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("firebase refresh: exhausted retries")
	}
	return nil, lastErr
}

// isRefreshTokenExpiredCode reports whether the Firebase `error.message`
// string matches one of the documented refresh-token-level rejections. The
// match is case-sensitive because Firebase returns these codes verbatim and
// case-folding would risk false matches on user-supplied input that gets
// echoed back in some other error path.
func isRefreshTokenExpiredCode(message string) bool {
	switch message {
	case "TOKEN_EXPIRED",
		"INVALID_REFRESH_TOKEN",
		"USER_DISABLED",
		"USER_NOT_FOUND":
		return true
	}
	// Firebase sometimes prefixes the code with a leading marker like
	// "INVALID_REFRESH_TOKEN : malformed token". A startswith check covers
	// those without false-positiving on unrelated 4xx error strings.
	for _, code := range []string{
		"TOKEN_EXPIRED",
		"INVALID_REFRESH_TOKEN",
		"USER_DISABLED",
		"USER_NOT_FOUND",
	} {
		if strings.HasPrefix(message, code) {
			return true
		}
	}
	return false
}

// shouldRetryNetwork reports whether a transport-level error is the kind we
// expect to be transient (connection reset, EOF mid-response, DNS blip).
// Context cancellation is not retried — the caller has signaled they're
// done waiting.
func shouldRetryNetwork(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "EOF") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "broken pipe")
}

// truncate caps a body for inclusion in error messages so a huge error body
// doesn't blow up logs. The cutoff is conservative — 200 chars is enough to
// see the leading JSON structure and the human-readable message.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
