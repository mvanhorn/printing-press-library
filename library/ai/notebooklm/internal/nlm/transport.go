// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/cliutil"
)

const defaultNLMRequestRate = 2.0
const maxHTTPAttempts = 4

func (s *Session) postForm(ctx context.Context, url, body string, header http.Header) ([]byte, error) {
	payload, resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
		for k, vals := range header {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, truncate(string(payload), 200))
	}
	return payload, nil
}

func (s *Session) get(ctx context.Context, url string, header http.Header) ([]byte, error) {
	payload, resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, vals := range header {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return payload, nil
}

func (s *Session) doWithRetry(ctx context.Context, build func() (*http.Request, error)) ([]byte, *http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < maxHTTPAttempts; attempt++ {
		if s.Limiter != nil {
			s.Limiter.Wait()
		}
		req, err := build()
		if err != nil {
			return nil, nil, err
		}
		resp, err := s.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			if s.Limiter != nil {
				s.Limiter.OnRateLimit()
			}
			wait := cliutil.RetryAfter(resp)
			lastErr = &cliutil.RateLimitError{
				URL:        req.URL.String(),
				RetryAfter: wait,
				Body:       truncate(string(body), 200),
			}
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if s.Limiter != nil {
			s.Limiter.OnSuccess()
			if rem, reset, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
				s.Limiter.ObserveHeaders(rem, reset)
			}
		}
		return body, resp, nil
	}
	if lastErr != nil {
		return nil, nil, lastErr
	}
	return nil, nil, &cliutil.RateLimitError{URL: "notebooklm request"}
}
