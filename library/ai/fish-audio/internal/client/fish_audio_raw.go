// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

// fish_audio_raw.go adds a raw-bytes POST to the generated client. POST
// /v1/tts answers with a chunked audio stream, not JSON, so the generated
// helpers (which unmarshal or base64-wrap the body) cannot carry it. PostRaw
// reuses the same base URL, auth, HTTP client, and adaptive rate limiter as
// every other call and hands back the bytes untouched.

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/cliutil"
)

// maxRawResponseBytes caps a single raw response. A malformed or hostile
// upstream must not be able to exhaust memory through an unbounded audio
// stream.
const maxRawResponseBytes = 512 << 20 // 512 MiB

// PostRaw sends body to path and returns the response bytes verbatim with the
// HTTP status code. Use it for endpoints whose success response is binary or a
// stream. headers may set Content-Type; it defaults to application/json.
//
// Behavior matches the generated transport where it matters:
//   - the verify harness short-circuits before any dial
//   - --dry-run returns an empty body and status 0 without dialing
//   - the adaptive rate limiter gates every attempt
//   - 429 and 5xx are retried with exponential backoff
//   - non-2xx returns an *APIError with a truncated, credential-masked body
func (c *Client) PostRaw(ctx context.Context, path string, body []byte, headers map[string]string) ([]byte, int, error) {
	if cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv() {
		return nil, http.StatusOK, nil
	}
	if err := rejectUnresolvedPathParams(path, nil); err != nil {
		return nil, 0, err
	}
	authHeader, err := c.authHeader(ctx)
	if err != nil {
		return nil, 0, err
	}
	if c.DryRun {
		return nil, 0, nil
	}

	targetURL := c.BaseURL + path
	maxRetries := clientMaxRetries()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
		if reqErr != nil {
			return nil, 0, fmt.Errorf("creating request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		if c.Config != nil {
			for k, v := range c.Config.Headers {
				req.Header.Set(k, v)
			}
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("User-Agent") == "" {
			if ua := os.Getenv("FISH_AUDIO_USER_AGENT"); ua != "" {
				req.Header.Set("User-Agent", ua)
			} else {
				req.Header.Set("User-Agent", "fish-audio-pp-cli/1")
			}
		}

		resp, doErr := c.HTTPClient.Do(req)
		if doErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, 0, ctxErr
			}
			lastErr = fmt.Errorf("POST %s: %w", c.displayURL(path, authHeader), c.maskError(doErr, authHeader))
			if attempt < maxRetries {
				if waitErr := backoffSleep(ctx, attempt); waitErr != nil {
					return nil, 0, waitErr
				}
				continue
			}
			return nil, 0, lastErr
		}

		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxRawResponseBytes+1))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("POST %s: reading response: %w", c.displayURL(path, authHeader), c.maskError(readErr, authHeader))
			if attempt < maxRetries {
				if waitErr := backoffSleep(ctx, attempt); waitErr != nil {
					return nil, 0, waitErr
				}
				continue
			}
			return nil, resp.StatusCode, lastErr
		}
		if int64(len(data)) > maxRawResponseBytes {
			return nil, resp.StatusCode, fmt.Errorf("POST %s: response exceeds the %d byte limit", path, int64(maxRawResponseBytes))
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
		} else if resp.StatusCode < 300 {
			c.limiter.OnSuccess()
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &APIError{
				Method:     http.MethodPost,
				Path:       path,
				StatusCode: resp.StatusCode,
				Body:       c.maskCredentialText(truncateBody(data), authHeader),
			}
			if attempt < maxRetries {
				if waitErr := backoffSleep(ctx, attempt); waitErr != nil {
					return nil, 0, waitErr
				}
				continue
			}
			return nil, resp.StatusCode, lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, resp.StatusCode, &APIError{
				Method:     http.MethodPost,
				Path:       path,
				StatusCode: resp.StatusCode,
				Body:       c.maskCredentialText(truncateBody(data), authHeader),
			}
		}
		return data, resp.StatusCode, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("POST %s: no attempt completed", path)
	}
	return nil, 0, lastErr
}

// backoffSleep waits out the exponential schedule the generated transport
// uses, and reports a cancelled context instead of sleeping through it.
func backoffSleep(ctx context.Context, attempt int) error {
	wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	fmt.Fprintf(os.Stderr, "retrying in %s (attempt %d)\n", wait, attempt+1)
	return sleepContext(ctx, wait)
}

// RawContentTypeJSON and RawContentTypeMsgpack name the two encodings POST
// /v1/tts accepts. MessagePack is required when the request carries inline
// reference audio, because JSON cannot hold raw bytes.
const (
	RawContentTypeJSON    = "application/json"
	RawContentTypeMsgpack = "application/msgpack"
)

// IsAudioContentType reports whether a Content-Type names audio.
func IsAudioContentType(ct string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "audio/")
}
