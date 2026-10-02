// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/cliutil"
)

const Origin = "https://www.limousinebus.co.jp"
const MaxBodyBytes = 2 * 1024 * 1024

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)
var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type Provider struct {
	HTTP       *http.Client
	Base       string
	Limiter    *cliutil.AdaptiveLimiter
	Requests   int
	ObservedAt string
}

func New(timeout time.Duration, rate float64) *Provider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if rate < 0 {
		rate = 2
	}
	jar, _ := cookiejar.New(nil)
	c := client.ProviderHTTPClient(timeout, jar)
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Observed data URLs return 200 directly. Refuse redirects so every wire
		// request is counted and paced by Fetch, including when the source changes.
		return fmt.Errorf("Airport Limousine redirect refused; recheck the canonical provider page")
	}

	return &Provider{HTTP: c, Base: Origin, Limiter: cliutil.NewAdaptiveLimiter(rate)}
}

func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid provider ID %q: use a stable ID from routes or stops find", id)
	}
	return nil
}

func (p *Provider) Fetch(ctx context.Context, method, path, body string) ([]byte, error) {
	if p.Requests >= 8 {
		return nil, fmt.Errorf("provider request cap (8) reached")
	}
	if (!strings.HasPrefix(path, "/en/") && path != "/ja/guide/terms/baggage/" && path != "/ja/guide/terms/baggage/__data.json") || strings.Contains(path, "..") {
		return nil, fmt.Errorf("unsupported provider path")
	}
	if err := p.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, p.Base+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.8")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", Origin)
		req.Header.Set("Referer", Origin+"/en/busstop/")
	}
	p.Requests++
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Airport Limousine request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		p.Limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: req.URL.String(), RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Airport Limousine HTTP %d for %s; check the canonical provider page", resp.StatusCode, path)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxBodyBytes {
		return nil, fmt.Errorf("provider response exceeds %d bytes", MaxBodyBytes)
	}
	p.Limiter.OnSuccess()
	p.ObservedAt = time.Now().UTC().Format(time.RFC3339)
	return b, nil
}

func (p *Provider) Data(ctx context.Context, path string) (map[string]any, error) {
	b, err := p.Fetch(ctx, http.MethodGet, path, "")
	if err != nil {
		return nil, err
	}
	return DecodeSvelte(b)
}

func (p *Provider) StopSearch(ctx context.Context, query string) (map[string]any, error) {
	if len([]rune(strings.TrimSpace(query))) == 0 || len([]rune(query)) > 80 {
		return nil, fmt.Errorf("--query requires 1–80 characters")
	}
	// Public search changes only two ephemeral keyword cookies in this command's private jar.
	// No account/session cookies are imported or persisted.
	b, err := p.Fetch(ctx, http.MethodPost, "/en/busstop/list/?/keyword", url.Values{"keyword": {query}}.Encode())
	if err != nil {
		return nil, err
	}
	var action struct {
		Type   string `json:"type"`
		Status int    `json:"status"`
	}
	if err = json.Unmarshal(b, &action); err != nil || action.Type != "success" || action.Status != 200 {
		return nil, fmt.Errorf("provider stop search did not confirm success")
	}
	return p.Data(ctx, "/en/busstop/list/__data.json")
}

// DecodeSvelte reads the first data record; optional streamed timetable chunks on guide pages
// are unrelated to their already-resolved content. Unknown reducers and invalid references fail closed.
func DecodeSvelte(body []byte) (map[string]any, error) {
	var payload struct {
		Type  string            `json:"type"`
		Nodes []json.RawMessage `json:"nodes"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("provider returned invalid Svelte data: %w", err)
	}
	if payload.Type != "data" {
		return nil, fmt.Errorf("provider returned %q instead of structured data", payload.Type)
	}
	var result map[string]any
	for _, raw := range payload.Nodes {
		if string(raw) == "null" {
			continue
		}
		var node struct {
			Type  string `json:"type"`
			Data  []any  `json:"data"`
			Error any    `json:"error"`
		}
		if err := json.Unmarshal(raw, &node); err != nil {
			return nil, err
		}
		if node.Type == "error" {
			return nil, fmt.Errorf("provider data error; retry the canonical page")
		}
		if node.Type != "data" || len(node.Data) == 0 {
			continue
		}
		visits := 0
		var decode func(int, int, map[int]bool) (any, error)
		decode = func(index, depth int, stack map[int]bool) (any, error) {
			visits++
			if visits > 80000 || depth > 64 {
				return nil, fmt.Errorf("provider data expansion limit exceeded")
			}
			if index < 0 {
				if index >= -6 {
					return nil, nil
				}
				return nil, fmt.Errorf("unknown negative data reference %d", index)
			}
			if index >= len(node.Data) {
				return nil, fmt.Errorf("invalid data reference %d", index)
			}
			if stack[index] {
				return nil, fmt.Errorf("cyclic provider data")
			}
			stack[index] = true
			defer delete(stack, index)
			ref := func(v any) (any, error) {
				f, ok := v.(float64)
				if !ok || f != float64(int(f)) {
					return nil, fmt.Errorf("noninteger data reference")
				}
				return decode(int(f), depth+1, stack)
			}
			switch v := node.Data[index].(type) {
			case map[string]any:
				obj := make(map[string]any, len(v))
				for k, x := range v {
					value, err := ref(x)
					if err != nil {
						return nil, err
					}
					obj[k] = value
				}
				return obj, nil
			case []any:
				if len(v) > 0 {
					if tag, ok := v[0].(string); ok {
						switch tag {
						case "Promise":
							return nil, nil
						case "Date":
							if len(v) == 2 {
								return v[1], nil
							}
						case "Object":
							if len(v) == 2 {
								return ref(v[1])
							}
						}
						return nil, fmt.Errorf("unsupported provider data reducer %q", tag)
					}
				}
				arr := make([]any, 0, len(v))
				for _, x := range v {
					value, err := ref(x)
					if err != nil {
						return nil, err
					}
					arr = append(arr, value)
				}
				return arr, nil
			default:
				return v, nil
			}
		}
		v, err := decode(0, 0, map[int]bool{})
		if err != nil {
			return nil, err
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("provider data node is not an object")
		}
		if _, props := obj["_props"]; !props {
			result = obj
		}
	}
	if result == nil {
		return nil, fmt.Errorf("provider response has no domain data node")
	}
	return result, nil
}

func M(v any) map[string]any { m, _ := v.(map[string]any); return m }
func A(v any) []any          { a, _ := v.([]any); return a }
func S(v any) string         { s, _ := v.(string); return s }
func B(v any) bool           { b, _ := v.(bool); return b }
func Num(v any) *int {
	f, ok := v.(float64)
	if !ok || f < 0 || f != float64(int(f)) {
		return nil
	}
	n := int(f)
	return &n
}
func Float(v any) *float64 {
	f, ok := v.(float64)
	if !ok {
		return nil
	}
	return &f
}

func DataPath(page string, query url.Values) string {
	path := strings.TrimRight(page, "/") + "/__data.json"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return path
}
func PageURL(page string, query url.Values) string {
	u := Origin + strings.TrimRight(page, "/") + "/"
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}
