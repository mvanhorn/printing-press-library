package ytstudio

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/cliutil"
)

const (
	studioOrigin  = "https://studio.youtube.com"
	innertubeBase = "https://studio.youtube.com/youtubei/v1"
)

// Client wraps a Studio session for Innertube XHR calls.
type Client struct {
	Session *SessionFile
	HTTP    *http.Client
	// Limiter paces outbound Innertube XHR calls. Studio rate limits per
	// session are aggressive; a nil limiter is a no-op for tests.
	Limiter *cliutil.AdaptiveLimiter
}

// New constructs a Studio Innertube client.
func New(s *SessionFile) *Client {
	return &Client{
		Session: s,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

// authHeader returns the SAPISIDHASH-formatted Authorization header.
// Format: SAPISIDHASH <timestamp>_<sha1(timestamp + ' ' + SAPISID + ' ' + origin)>
func (c *Client) authHeader() (string, error) {
	if c.Session == nil {
		return "", errors.New("no Studio session loaded")
	}
	sapisid := c.Session.SAPISID()
	if sapisid == "" {
		return "", errors.New("Studio session has no SAPISID cookie; re-run `yt-studio-pp-cli login --studio`")
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := sha1.New()
	_, _ = h.Write([]byte(ts + " " + sapisid + " " + studioOrigin))
	return fmt.Sprintf("SAPISIDHASH %s_%s", ts, hex.EncodeToString(h.Sum(nil))), nil
}

// innertubeContext returns the standard Innertube context blob for Studio.
func (c *Client) innertubeContext() map[string]any {
	return map[string]any{
		"client": map[string]any{
			"clientName":    "WEB_CREATOR",
			"clientVersion": c.Session.EffectiveClientVersion(),
			"hl":            "en",
			"gl":            "US",
		},
	}
}

// CallResponse is the raw JSON response from an Innertube call.
type CallResponse struct {
	StatusCode int
	Body       []byte
}

// Call POSTs a body to a Studio Innertube endpoint and returns the raw response.
// Caller is responsible for decoding the body shape (Innertube responses vary).
func (c *Client) Call(ctx context.Context, path string, body map[string]any) (*CallResponse, error) {
	if c.Session == nil {
		return nil, errors.New("no Studio session loaded; run `yt-studio-pp-cli login`")
	}
	if body == nil {
		body = map[string]any{}
	}
	if _, ok := body["context"]; !ok {
		body["context"] = c.innertubeContext()
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	endpoint := innertubeBase + path
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	auth, err := c.authHeader()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", c.Session.CookieHeader())
	req.Header.Set("Origin", studioOrigin)
	req.Header.Set("Referer", studioOrigin+"/")
	req.Header.Set("X-Origin", studioOrigin)
	req.Header.Set("X-Goog-AuthUser", "0")

	c.Limiter.Wait()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("studio call %s: %w", path, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		c.Limiter.OnRateLimit()
	} else if resp.StatusCode < 400 {
		c.Limiter.OnSuccess()
	}
	return &CallResponse{StatusCode: resp.StatusCode, Body: out}, nil
}

// ErrorKind classifies Studio errors.
type ErrorKind string

const (
	KindAuth        ErrorKind = "auth"
	KindRateLimit   ErrorKind = "rate_limit"
	KindSchemaDrift ErrorKind = "schema_drift"
	KindOther       ErrorKind = "other"
)

// Error is a typed Studio error.
type Error struct {
	StatusCode int
	Kind       ErrorKind
	Body       string
}

func (e *Error) Error() string {
	return fmt.Sprintf("studio innertube error (kind=%s, http=%d)", e.Kind, e.StatusCode)
}

// classify maps an HTTP response to a typed error or nil.
func classify(r *CallResponse) error {
	if r == nil {
		return errors.New("nil response")
	}
	switch {
	case r.StatusCode == 401 || r.StatusCode == 403:
		return &Error{StatusCode: r.StatusCode, Kind: KindAuth, Body: string(r.Body)}
	case r.StatusCode == 429:
		return &Error{StatusCode: r.StatusCode, Kind: KindRateLimit, Body: string(r.Body)}
	case r.StatusCode >= 500:
		return &Error{StatusCode: r.StatusCode, Kind: KindOther, Body: string(r.Body)}
	case r.StatusCode >= 400:
		return &Error{StatusCode: r.StatusCode, Kind: KindOther, Body: string(r.Body)}
	}
	return nil
}

// CheckHealth probes the get_screen endpoint to verify the session is alive.
// Returns nil on 2xx; typed Error otherwise.
func (c *Client) CheckHealth(ctx context.Context) error {
	resp, err := c.Call(ctx, "/analytics/get_screen?alt=json", map[string]any{
		"screenConfig": map[string]any{"timePeriod": "DAYS_28"},
	})
	if err != nil {
		return err
	}
	return classify(resp)
}
