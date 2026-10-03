// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil"
)

const publicConfigURL = "https://www.hostelworld.com/pwa/s?type=city&id=452"

var publicAppLiteral = regexp.MustCompile(`APIGEE_KEY:\s*"([A-Za-z0-9_-]{8,120})"`)

// AttachHostelworldApplication supplies only the public anonymous application
// identifier observed in first-party config. It never imports a user session.
func AttachHostelworldApplication(c *Client) error {
	if c == nil || c.HTTPClient == nil {
		return fmt.Errorf("Hostelworld client is unavailable")
	}
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.HTTPClient.Transport = &hostelworldApplicationTransport{base: base, limiter: cliutil.NewAdaptiveLimiter(2)}
	c.NoCache = true // dated inventory must be requested on every live invocation
	return nil
}

type hostelworldApplicationTransport struct {
	base        http.RoundTripper
	limiter     *cliutil.AdaptiveLimiter
	mu          sync.Mutex
	application string
}

func (t *hostelworldApplicationTransport) app(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.application != "" {
		return t.application, nil
	}
	if err := t.limiter.Wait(ctx); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicConfigURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	h := &http.Client{Transport: t.base, Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || r.URL.Scheme != "https" || r.URL.Host != "www.hostelworld.com" {
			return fmt.Errorf("public config redirect refused")
		}
		return nil
	}}
	resp, err := h.Do(req)
	if err != nil {
		return "", fmt.Errorf("public application config could not be fetched: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		t.limiter.OnRateLimit()
		return "", &cliutil.RateLimitError{URL: publicConfigURL, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("public application config returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return "", fmt.Errorf("public config body could not be read")
	}
	if len(body) > 4*1024*1024 {
		return "", fmt.Errorf("public application config exceeds 4 MiB")
	}
	matches := publicAppLiteral.FindAllSubmatch(body, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("public application config shape changed; expected one first-party APIGEE_KEY literal; retry later")
	}
	t.application = string(matches[0][1])
	t.limiter.OnSuccess()
	return t.application, nil
}

func (t *hostelworldApplicationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != "prod.apigee.hostelworld.com" {
		return t.base.RoundTrip(req)
	}
	if req.Method != http.MethodGet {
		return nil, fmt.Errorf("Hostelworld integration supports read-only GET requests")
	}
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	needsApp := strings.HasPrefix(req.URL.Path, "/autocomplete-service/") || strings.HasPrefix(req.URL.Path, "/legacy-hwapi-service/2.2/cities/")
	if needsApp {
		key, err := t.app(req.Context())
		if err != nil {
			return nil, err
		}
		cloned.Header.Set("api-key", key)
	}
	cloned.Header.Set("Accept-Encoding", "identity")
	cloned.Header.Del("Cookie")
	cloned.Header.Del("Authorization")
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(cloned)
	if err != nil {
		return nil, err
	}
	var reader io.Reader = resp.Body
	var gz *gzip.Reader
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "", "identity":
	case "gzip":
		gz, err = gzip.NewReader(resp.Body)
		if err != nil {
			_ = resp.Body.Close() // preserve the decoding error from a read-only response
			return nil, fmt.Errorf("Hostelworld gzip response is invalid")
		}
		reader = gz
	default:
		_ = resp.Body.Close() // preserve the unsupported encoding diagnosis
		return nil, fmt.Errorf("Hostelworld response encoding is unsupported")
	}
	body, readErr := io.ReadAll(io.LimitReader(reader, 8*1024*1024+1))
	if gz != nil {
		_ = gz.Close() // gzip.Reader.Close does not close the underlying read-only body
	}
	closeErr := resp.Body.Close()
	if readErr == nil && closeErr != nil {
		return nil, fmt.Errorf("Hostelworld response cleanup failed")
	}
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Set-Cookie")
	resp.Header.Del("api-key")
	if readErr != nil {
		return nil, fmt.Errorf("Hostelworld response body could not be read")
	}
	if len(body) > 8*1024*1024 {
		return nil, fmt.Errorf("Hostelworld response exceeds 8 MiB")
	}
	if resp.StatusCode >= 400 {
		body = []byte(fmt.Sprintf("Hostelworld source returned HTTP %d; response body suppressed", resp.StatusCode))
	} else if key := cloned.Header.Get("api-key"); key != "" {
		body = bytes.ReplaceAll(body, []byte(key), []byte("[redacted-application-identifier]"))
	}
	if resp.StatusCode == 429 {
		t.limiter.OnRateLimit()
	} else if resp.StatusCode < 400 {
		t.limiter.OnSuccess()
	}
	if resp.StatusCode == 200 {
		body, err = hostelworldPublicFacts(req.URL.Path, body)
		if err != nil {
			return nil, err
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Del("Content-Length")
	return resp, nil
}

// Remove editorial descriptions and contributor profiles before generic endpoint
// output, caches, proofs, or MCP can observe the response.
func hostelworldPublicFacts(path string, body []byte) ([]byte, error) {
	if !strings.HasPrefix(path, "/legacy-hwapi-service/2.2/") {
		return body, nil
	}
	var v map[string]any
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, fmt.Errorf("Hostelworld response JSON is invalid")
	}
	if strings.Contains(path, "/properties/") && !strings.HasSuffix(path, "/availability/") && !strings.Contains(path, "/cities/") {
		fields := []string{"id", "name", "transName", "currency", "isActive", "type", "checkIn", "latestCheckOut", "maxNumberOfGuestsPerBooking", "rating", "totalRatings", "facilities", "latitude", "longitude", "address1", "address2", "city", "paymentMethods", "payments", "depositPercentage", "cancellationPolicy", "freeCancellation", "thingsToNote", "policies", "groupInformation", "taxInfo", "feeInfo"}
		out := map[string]any{}
		for _, key := range fields {
			if x, ok := v[key]; ok {
				out[key] = x
			}
		}
		v = out
	} else if strings.Contains(path, "/cities/") {
		if props, ok := v["properties"].([]any); ok {
			for _, p := range props {
				if prop, ok := p.(map[string]any); ok {
					for _, key := range []string{"reviews", "description", "overview", "images", "imagesGallery", "hostelworldSays", "socialCues"} {
						delete(prop, key)
					}
				}
			}
		}
	}
	return json.Marshal(v)
}
