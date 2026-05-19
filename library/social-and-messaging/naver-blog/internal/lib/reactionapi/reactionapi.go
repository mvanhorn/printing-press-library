// Package reactionapi calls Naver's public reaction-counts endpoint
// to fetch like (공감) counts for one or more Naver Blog posts.
//
// Endpoint: https://apis.naver.com/blogserver/like/v1/search/contents
// Authentication: none (it's the same endpoint that powers the
// like-counter widget on every blog page).
//
// The endpoint accepts a "q" param of shape:
//
//	BLOG[<blog_id>_<log_no>,<blog_id>_<log_no>,...]
//
// and returns one entry per contentsId with the like reaction count
// inline. Posts that don't exist (deleted/private/blog-deleted)
// are absent from the response, NOT zero — callers should treat
// absence as "unknown" (nullable) rather than collapsing it to 0.
package reactionapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Limiter paces outbound requests. Wait blocks until the next slot is
// available; OnRateLimit informs the limiter that the upstream returned
// 429 so the limiter can halve its rate; OnSuccess informs it of a clean
// response so the limiter can ramp back up. Defined as an interface here
// so the lib stays free of cliutil (and any other limiter package);
// cliutil.AdaptiveLimiter satisfies this interface by name. Pass nil to
// disable rate limiting.
type Limiter interface {
	Wait()
	OnSuccess()
	OnRateLimit()
}

// RateLimitError is returned by GetReactions / fetchBatch when the
// reaction API responds with HTTP 429 Too Many Requests. Callers can
// type-assert with errors.As to honor the Retry-After header before
// retrying. The Naver reaction endpoint is unauthenticated and
// rate-limit thresholds aren't documented; in practice the endpoint
// tolerates the polite default in the CLI (--rate-limit 2 rps), but
// busy multi-blog batches occasionally observe 429s.
type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration // 0 when the response omits Retry-After
	Body       string        // truncated response body for diagnostics
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("reaction API rate limited (HTTP %d), retry after %s", e.StatusCode, e.RetryAfter)
	}
	return fmt.Sprintf("reaction API rate limited (HTTP %d)", e.StatusCode)
}

// parseRetryAfter accepts either "<delta-seconds>" or an HTTP-date
// (RFC 1123) per RFC 7231 §7.1.3. Returns 0 when the header is
// missing or unparseable so callers can apply a default backoff.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// MaxBatchSize is the per-request post-key cap. Empirically the
// endpoint accepts up to ~50 entries before truncating; we cap at 20
// to stay well under that limit and to keep individual response
// bodies small. GetReactions chunks larger inputs at this boundary.
const MaxBatchSize = 20

// PostKey identifies a single Naver Blog post for a reaction lookup.
type PostKey struct {
	BlogID string
	LogNo  string
}

// reactionsEndpoint is the public URL. Hoisted as a var so tests can
// override it with an httptest.Server URL.
var reactionsEndpoint = "https://apis.naver.com/blogserver/like/v1/search/contents"

type apiResponse struct {
	Contents []struct {
		ContentsID string `json:"contentsId"`
		Reactions  []struct {
			ReactionType string `json:"reactionType"`
			Count        int    `json:"count"`
		} `json:"reactions"`
	} `json:"contents"`
}

// GetReactions returns a map keyed by "<blog_id>_<log_no>" with the
// like count for each post that the API responded for. Missing keys
// in the map mean the API didn't return data for that post — caller
// MUST distinguish "0 likes" (present, count==0) from "unknown"
// (absent) rather than collapsing both to zero.
//
// Batches over MaxBatchSize are chunked into multiple sequential
// requests and merged. A failure in one chunk aborts the whole call
// (partial successes are confusing for downstream code that relies on
// "absent = unknown" semantics).
//
// httpClient is required — pass http.DefaultClient if you don't have
// a tuned one. ctx is honored on every request. For multi-chunk batches
// against a rate-limited deployment, prefer GetReactionsLimited so the
// caller's cliutil.AdaptiveLimiter can pace the chunks alongside the
// rest of the CLI's HTTP traffic.
func GetReactions(ctx context.Context, httpClient *http.Client, postKeys []PostKey) (map[string]int, error) {
	return GetReactionsLimited(ctx, httpClient, nil, postKeys)
}

// GetReactionsLimited is GetReactions with an explicit limiter. When
// limiter is non-nil, every chunked fetch waits on limiter.Wait() before
// firing, calls limiter.OnSuccess() on a clean response, and calls
// limiter.OnRateLimit() when the upstream returns 429. Passing nil
// retains the legacy behavior (a fixed 200ms inter-chunk pacing).
//
// Callers in this repo wire a *cliutil.AdaptiveLimiter through so the
// reaction endpoint shares the same outbound rate budget as the
// HTML-fetching client.
func GetReactionsLimited(ctx context.Context, httpClient *http.Client, limiter Limiter, postKeys []PostKey) (map[string]int, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("nil http client")
	}
	out := make(map[string]int, len(postKeys))
	if len(postKeys) == 0 {
		return out, nil
	}

	// Inter-chunk pacing fallback when no Limiter is supplied. With a
	// limiter, the limiter's own rate gating supersedes this constant.
	const fallbackPacing = 200 * time.Millisecond
	for start := 0; start < len(postKeys); start += MaxBatchSize {
		end := start + MaxBatchSize
		if end > len(postKeys) {
			end = len(postKeys)
		}
		if start > 0 && limiter == nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(fallbackPacing):
			}
		}
		if limiter != nil {
			limiter.Wait()
		}
		chunk := postKeys[start:end]
		got, err := fetchBatch(ctx, httpClient, chunk)
		if err != nil {
			// Inform the limiter of the upstream rate-limit signal so it
			// can halve its rate before the next call. Other errors don't
			// move the limiter — they're transient or structural.
			if limiter != nil {
				var rle *RateLimitError
				if errors.As(err, &rle) {
					limiter.OnRateLimit()
				}
			}
			return nil, err
		}
		if limiter != nil {
			limiter.OnSuccess()
		}
		for k, v := range got {
			out[k] = v
		}
	}
	return out, nil
}

func fetchBatch(ctx context.Context, httpClient *http.Client, postKeys []PostKey) (map[string]int, error) {
	if len(postKeys) == 0 {
		return map[string]int{}, nil
	}

	ids := make([]string, 0, len(postKeys))
	for _, pk := range postKeys {
		if pk.BlogID == "" || pk.LogNo == "" {
			continue
		}
		ids = append(ids, pk.BlogID+"_"+pk.LogNo)
	}
	if len(ids) == 0 {
		return map[string]int{}, nil
	}

	q := url.Values{}
	q.Set("pool", "blogid")
	// Manually construct the q parameter — url.Values.Set will encode
	// the brackets and commas, which the API still accepts but the
	// canonical request shape we observed uses literal brackets.
	// Using Encode handles the escaping for us; the API tolerates both
	// shapes.
	q.Set("q", buildQParam(ids))
	q.Set("isDuplication", "false")
	reqURL := reactionsEndpoint + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling reaction API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &RateLimitError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			Body:       truncate(string(body), 512),
		}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("reaction API HTTP %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding response: %w (body: %s)", err, truncate(string(body), 512))
	}

	out := make(map[string]int, len(parsed.Contents))
	for _, c := range parsed.Contents {
		for _, r := range c.Reactions {
			if r.ReactionType == "like" {
				out[c.ContentsID] = r.Count
				break
			}
		}
	}
	return out, nil
}

// buildQParam joins ids into "BLOG[id1,id2,...]" without URL-encoding
// the brackets/commas — url.Values.Encode does that for us.
func buildQParam(ids []string) string {
	if len(ids) == 0 {
		return "BLOG[]"
	}
	s := "BLOG["
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += id
	}
	s += "]"
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
