package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/cliutil"
)

const Endpoint = "https://www.pocket-concierge.jp/graphql"
const maxBody = 2 << 20
const maxCacheBytes = 16 << 20

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"http_status,omitempty"`
}

func (e *Error) Error() string        { return e.Code + ": " + e.Message }
func Fail(code, message string) error { return &Error{Code: code, Message: message} }

type Observation struct {
	Language   string    `json:"language"`
	FetchedAt  time.Time `json:"fetched_at"`
	CacheHit   bool      `json:"cache_hit"`
	TTLSeconds int       `json:"ttl_seconds"`
}
type Metrics struct {
	Requests      int           `json:"requests"`
	CacheHits     int           `json:"cache_hits"`
	ResponseBytes int           `json:"response_bytes"`
	Observations  []Observation `json:"observations"`
}
type Client struct {
	HTTP             *http.Client
	CacheDir         string
	Refresh, NoCache bool
	Metrics          Metrics
	limiter          *cliutil.AdaptiveLimiter
}

func New(cacheDir string, refresh, noCache bool) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, CacheDir: cacheDir, Refresh: refresh, NoCache: noCache, limiter: cliutil.NewAdaptiveLimiter(2), Metrics: Metrics{Observations: []Observation{}}}
}

type cacheEntry struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}

func (c *Client) ReadQuery(ctx context.Context, lang, query string, variables any, ttl time.Duration, out any) error {
	if lang != "en" && lang != "ja" {
		return Fail("usage", "language must be en or ja")
	}
	// No caller-supplied GraphQL is exposed. Defense at the transport boundary.
	if !strings.HasPrefix(strings.TrimSpace(query), "query ") || strings.Contains(query, "mutation ") {
		return Fail("usage", "only fixed read queries are supported")
	}
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	sum := sha256.Sum256(append([]byte(lang+":"), payload...))
	key := hex.EncodeToString(sum[:])
	path := filepath.Join(c.CacheDir, "pocket-v1-"+key+".json")
	if ttl > 0 && !c.Refresh && !c.NoCache && c.CacheDir != "" {
		// #nosec G304 -- Operator-selected cache directory plus fixed provider prefix and SHA-256 key; no source-controlled path.
		if f, err := os.Open(path); err == nil {
			b, readErr := io.ReadAll(io.LimitReader(f, maxBody+1024))
			closeErr := f.Close()
			var e cacheEntry
			if readErr == nil && closeErr == nil && json.Unmarshal(b, &e) == nil && !e.FetchedAt.IsZero() && time.Since(e.FetchedAt) >= 0 && time.Since(e.FetchedAt) < ttl && unmarshalFresh(e.Data, out) == nil {
				c.Metrics.CacheHits++
				c.Metrics.Observations = append(c.Metrics.Observations, Observation{lang, e.FetchedAt, true, int(ttl.Seconds())})
				return nil
			}
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err = c.limiter.Wait(ctx); err != nil {
			return Fail("timeout", "command deadline exceeded before source request")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", lang)
		req.Header.Set("X-Auth-Origin", "GUEST_ORIGIN")
		req.Header.Set("User-Agent", "pocket-concierge-pp-cli/0.1 (public read-only discovery)")
		c.Metrics.Requests++
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
				return Fail("timeout", "Pocket Concierge request deadline exceeded")
			}
			return Fail("network", "Pocket Concierge HTTPS request failed; check connectivity")
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		_ = resp.Body.Close() // The full-body read error governs correctness; closing a consumed response is cleanup.
		c.Metrics.ResponseBytes += len(body)
		if resp.StatusCode == 429 {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			if attempt == 0 && wait <= 2*time.Second {
				if wait <= 0 {
					wait = time.Second
				}
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return Fail("timeout", "deadline during rate-limit backoff")
				case <-timer.C:
				}
				continue
			}
			return &cliutil.RateLimitError{URL: Endpoint, RetryAfter: wait}
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			timer := time.NewTimer(300 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return Fail("timeout", "deadline during source retry")
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != 200 {
			code := "source_http"
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				code = "access_denied"
			}
			return &Error{Code: code, Message: "Pocket Concierge returned a non-success response", Status: resp.StatusCode}
		}
		if readErr != nil {
			return Fail("network", "could not read complete source response")
		}
		if len(body) > maxBody {
			return Fail("source_schema", "source response exceeds 2 MiB safety bound")
		}
		var env struct {
			Data   json.RawMessage   `json:"data"`
			Errors []json.RawMessage `json:"errors"`
		}
		if json.Unmarshal(body, &env) != nil || len(env.Data) == 0 || bytes.Equal(env.Data, []byte("null")) {
			return Fail("source_schema", "expected GraphQL data JSON; source may have changed or blocked access")
		}
		if len(env.Errors) > 0 {
			return Fail("source_graphql", "source returned GraphQL errors; partial data withheld")
		}
		if err = unmarshalFresh(env.Data, out); err != nil {
			return Fail("source_schema", "source data has unexpected types")
		}
		now := time.Now().UTC()
		c.Metrics.Observations = append(c.Metrics.Observations, Observation{lang, now, false, int(ttl.Seconds())})
		c.limiter.OnSuccess()
		if ttl > 0 && !c.NoCache && c.CacheDir != "" {
			c.writeCache(path, cacheEntry{now, env.Data})
		}
		return nil
	}
	return Fail("source_http", "source retry exhausted")
}

// Decode into a new value so partial failures cannot contaminate a live fallback,
// and fields omitted by a successful response do not retain earlier values.
func unmarshalFresh(data []byte, out any) error {
	dest := reflect.ValueOf(out)
	if !dest.IsValid() || dest.Kind() != reflect.Pointer || dest.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(out)}
	}
	fresh := reflect.New(dest.Elem().Type())
	if err := json.Unmarshal(data, fresh.Interface()); err != nil {
		return err
	}
	dest.Elem().Set(fresh.Elem())
	return nil
}
func (c *Client) writeCache(path string, e cacheEntry) {
	b, err := json.Marshal(e)
	if err != nil || len(b) > maxBody {
		return
	}
	if err = os.MkdirAll(c.CacheDir, 0700); err != nil {
		return
	}
	entries, err := os.ReadDir(c.CacheDir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		t    time.Time
	}
	files := []file{}
	var total int64
	for _, entry := range entries {
		if entry.Type().IsRegular() && ownedCacheName(entry.Name()) {
			info, err := entry.Info()
			if err == nil {
				files = append(files, file{filepath.Join(c.CacheDir, entry.Name()), info.Size(), info.ModTime()})
				total += info.Size()
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].t.Before(files[j].t) })
	for len(files) >= 128 || total+int64(len(b)) > maxCacheBytes {
		if len(files) == 0 {
			break
		}
		f := files[0]
		if os.Remove(f.path) == nil {
			total -= f.size
		}
		files = files[1:]
	}
	f, err := os.CreateTemp(c.CacheDir, ".entry-*")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		_ = f.Close() // Write already failed; the abandoned temporary file is removed below.
		return
	}
	if f.Close() != nil {
		return
	}
	_ = os.Rename(name, path)
}
func (c *Client) Meta(lang string, elapsed time.Duration) map[string]any {
	var oldest *time.Time
	for _, o := range c.Metrics.Observations {
		if oldest == nil || o.FetchedAt.Before(*oldest) {
			t := o.FetchedAt
			oldest = &t
		}
	}
	return map[string]any{"provider": "Pocket Concierge", "source_url": Endpoint, "language": lang, "timezone": "Asia/Tokyo", "coverage": "public Pocket Concierge listings only; unpublished and account-exclusive inventory excluded", "partial": false, "retrieved_at": time.Now().UTC(), "oldest_fetched_at": oldest, "requests": c.Metrics.Requests, "cache_hits": c.Metrics.CacheHits, "response_bytes": c.Metrics.ResponseBytes, "elapsed_ms": elapsed.Milliseconds(), "observations": c.Metrics.Observations}
}
func ExitCode(err error) int {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return 5
	}
	var e *Error
	if errors.As(err, &e) {
		switch e.Code {
		case "usage":
			return 2
		case "not_found":
			return 3
		case "access_denied":
			return 4
		case "network", "timeout":
			return 6
		default:
			return 7
		}
	}
	return 1
}
func ErrorJSON(err error) any {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return map[string]any{"code": "rate_limited", "message": "Pocket Concierge rate limit exhausted; retry later", "retry_after_seconds": rate.RetryAfter.Seconds()}
	}
	return &Error{Code: "internal", Message: fmt.Sprint(err)}
}

func ownedCacheName(name string) bool {
	if !strings.HasPrefix(name, "pocket-v1-") || !strings.HasSuffix(name, ".json") {
		return false
	}
	key := strings.TrimSuffix(strings.TrimPrefix(name, "pocket-v1-"), ".json")
	if len(key) != 64 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}
