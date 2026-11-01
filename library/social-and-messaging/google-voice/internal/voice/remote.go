package voice

import (
	"bytes"
	"context"
	"crypto/sha1" // #nosec G505 -- Google SAPISIDHASH requires SHA-1 for protocol interoperability; not used for password storage.
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Base = "https://clients6.google.com/voice/v1/voiceclient"

var readPaths = map[string]bool{"/voice/v1/voiceclient/account/get": true, "/voice/v1/voiceclient/api2thread/list": true, "/voice/v1/voiceclient/api2thread/get": true}

type HTTPError struct{ Status int }

func (e *HTTPError) Error() string {
	if e.Status == 401 || e.Status == 403 {
		return "Google Voice session rejected; capture a fresh signed-in read HAR and run auth import-har"
	}
	return fmt.Sprintf("Google Voice read returned HTTP %d", e.Status)
}

type Remote struct {
	HTTP            *http.Client
	Session         *Session
	limiter         *cliutil.AdaptiveLimiter
	paceDisabled    bool
	verifiedAccount string
}

func NewRemote(s *Session, rate ...float64) *Remote {
	pace := 1.0
	disabled := false
	if len(rate) > 0 {
		if rate[0] == 0 {
			disabled = true
		} else if rate[0] > 0 {
			pace = rate[0]
		}
	}
	return &Remote{Session: s, limiter: cliutil.NewAdaptiveLimiter(pace), paceDisabled: disabled, HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("Voice read redirects are refused") }}}
}

func signature(sapisid string, now time.Time) string {
	ts := strconv.FormatInt(now.Unix(), 10)
	return fmt.Sprintf("SAPISIDHASH %s_%x", ts, sha1.Sum([]byte(ts+" "+sapisid+" https://voice.google.com"))) // #nosec G401 -- Fixed-origin Google SAPISIDHASH requires this SHA-1 digest; changing algorithms invalidates authentication.
}
func (r *Remote) call(ctx context.Context, path string, body any) ([]byte, error) {
	if !readPaths["/voice/v1/voiceclient"+path] {
		return nil, fmt.Errorf("Voice RPC is outside the three-read allowlist")
	}
	if r.Session == nil || r.Session.Cookies["SAPISID"] == "" {
		return nil, fmt.Errorf("private Voice session is missing")
	}
	if r.limiter == nil {
		r.limiter = cliutil.NewAdaptiveLimiter(1)
	}
	if !r.paceDisabled {
		if err := r.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	u := Base + path + "?" + url.Values{"alt": {"protojson"}, "key": {r.Session.PublicKey}}.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	for k, v := range r.Session.Headers {
		if headerNames[strings.ToLower(k)] {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Origin", "https://clients6.google.com")
	req.Header.Set("X-Origin", "https://voice.google.com")
	req.Header.Set("Content-Type", "application/json+protobuf")
	req.Header.Set("Authorization", signature(r.Session.Cookies["SAPISID"], time.Now()))
	var names []string
	for k := range r.Session.Cookies {
		if cookieNames[k] {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	var cookies []string
	for _, k := range names {
		cookies = append(cookies, k+"="+r.Session.Cookies[k])
	}
	req.Header.Set("Cookie", strings.Join(cookies, "; "))
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Voice network read failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		r.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: Base + path, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, &HTTPError{resp.StatusCode}
	}
	r.limiter.OnSuccess()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20+1))
	if err != nil {
		return nil, fmt.Errorf("reading Voice response failed")
	}
	if len(raw) > 16<<20 {
		return nil, fmt.Errorf("Voice response exceeded 16 MiB safety limit")
	}
	// Keep cookie rotation private; an expired browser-bound session still requires re-import.
	changed := false
	for _, c := range resp.Cookies() {
		if cookieNames[c.Name] && c.Value != "" {
			if r.Session.Cookies[c.Name] != c.Value {
				r.Session.Cookies[c.Name] = c.Value
				changed = true
			}
		}
	}
	if changed && r.Session.AccountID != "" {
		if err := SaveSession(*r.Session); err != nil {
			return nil, err
		}
	}
	return raw, nil
}
func (r *Remote) Account(ctx context.Context) (Account, error) {
	b, e := r.call(ctx, "/account/get", []any{nil, 1})
	if e != nil {
		return Account{}, e
	}
	a, e := DecodeAccount(b)
	if e == nil && r.Session.AccountID != "" && r.Session.AccountID != a.ID {
		return Account{}, fmt.Errorf("Voice account mismatch; use a separate GOOGLE_VOICE_PROFILE before importing another account")
	}
	if e == nil {
		r.verifiedAccount = a.ID
	}
	return a, e
}
func (r *Remote) verifyAccount(ctx context.Context) error {
	if r.Session == nil || r.Session.AccountID == "" {
		return fmt.Errorf("Voice read requires a pinned imported account")
	}
	if r.verifiedAccount == r.Session.AccountID {
		return nil
	}
	_, e := r.Account(ctx)
	return e
}
func (r *Remote) List(ctx context.Context, folder, cursor string) (Page, error) {
	code := 2
	switch folder {
	case "texts":
	case "archived":
		code = 6
	default:
		return Page{}, fmt.Errorf("--folder must be texts or archived")
	}
	if e := r.verifyAccount(ctx); e != nil {
		return Page{}, e
	}
	b, e := r.call(ctx, "/api2thread/list", []any{code, 20, 15, cursor, nil, []any{nil, 1, 1, 1}})
	if e != nil {
		return Page{}, e
	}
	return DecodePage(b, false)
}
func (r *Remote) Get(ctx context.Context, id, cursor string, count int) (Page, error) {
	if id == "" || count < 1 || count > 100 {
		return Page{}, fmt.Errorf("--thread-id is required and --count must be 1 through 100")
	}
	if e := r.verifyAccount(ctx); e != nil {
		return Page{}, e
	}
	b, e := r.call(ctx, "/api2thread/get", []any{id, count, cursor, []any{nil, 1, 1}})
	if e != nil {
		return Page{}, e
	}
	p, e := DecodePage(b, true)
	if e == nil && p.Threads[0].ID != id {
		return Page{}, fmt.Errorf("Voice returned a different thread; refusing page")
	}
	return p, e
}

// Gateway translates logical generated CLI/MCP requests at the last HTTP boundary.
// It denies every remote operation outside the verified three-read surface.
type Gateway struct{ Transport http.RoundTripper }

func (g *Gateway) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "POST" || req.URL.Scheme != "https" || req.URL.Host != "clients6.google.com" || !readPaths[req.URL.Path] {
		return nil, fmt.Errorf("Voice gateway accepts only the three fixed-host read RPCs")
	}
	var input map[string]any
	if req.Body != nil {
		defer req.Body.Close()
		if err := json.NewDecoder(io.LimitReader(req.Body, 1<<20)).Decode(&input); err != nil && err != io.EOF {
			return nil, fmt.Errorf("Voice logical request must be a JSON object")
		}
	}
	// Validate caller-provided limits before opening credentials or doing IO.
	count := 100
	if req.URL.Path == "/voice/v1/voiceclient/api2thread/get" {
		if value, present := input["count"]; present {
			n, ok := value.(float64)
			if !ok || math.Trunc(n) != n || n < 1 || n > 100 {
				return nil, fmt.Errorf("count must be an integer from 1 through 100")
			}
			count = int(n)
		}
	}
	s, err := LoadSession()
	if err != nil {
		return nil, err
	}
	r := NewRemote(&s)
	r.HTTP.Transport = g.Transport
	r.HTTP.Timeout = 0
	var out any
	switch req.URL.Path {
	case "/voice/v1/voiceclient/account/get":
		out, err = r.Account(req.Context())
	case "/voice/v1/voiceclient/api2thread/list":
		folder := str(input["folder"])
		if folder == "" {
			folder = "texts"
		}
		var p Page
		p, err = r.List(req.Context(), folder, str(input["cursor"]))
		if input["unread"] == true {
			ts := []Thread{}
			for _, t := range p.Threads {
				if t.Read != nil && !*t.Read {
					ts = append(ts, t)
				}
			}
			p.Threads = ts
		}
		out = p
	case "/voice/v1/voiceclient/api2thread/get":
		var p Page
		p, err = r.Get(req.Context(), str(input["thread_id"]), str(input["cursor"]), count)
		if input["unread"] == true && err == nil && (p.Threads[0].Read == nil || *p.Threads[0].Read) {
			p.UnknownReadState = p.Threads[0].Read == nil
			p.Threads = []Thread{}
		}
		out = p
	}
	status := 200
	retryAfter := ""
	if err != nil {
		var rate *cliutil.RateLimitError
		if errors.As(err, &rate) {
			status = 429
			out = map[string]string{"error": "Google Voice rate limited this read"}
			if rate.RetryAfter > 0 {
				retryAfter = strconv.FormatInt(int64(math.Ceil(rate.RetryAfter.Seconds())), 10)
			}
		} else if e, ok := err.(*HTTPError); ok {
			status = e.Status
			out = map[string]string{"error": e.Error()}
		} else {
			return nil, err
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	headers := http.Header{"Content-Type": {"application/json"}}
	if retryAfter != "" {
		headers.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: headers, Body: io.NopCloser(bytes.NewReader(b)), ContentLength: int64(len(b)), Request: req}, nil
}
