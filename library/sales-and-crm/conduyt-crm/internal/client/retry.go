// PATCH(retry-policy): hand-written file, not generated; see .printing-press-patches.json.

package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	mrand "math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxRetryDelay caps any wait the server asks for, jitter included.
const maxRetryDelay = 30 * time.Second

// Test seams.
var (
	retryJitter = mrand.Float64
	retryNow    = time.Now
	retrySleep  = sleepContext
)

// DefaultCallBudget bounds one whole call (attempts, waits, limiter pacing).
const DefaultCallBudget = 120 * time.Second

// idempotentEndpoints lists "METHOD /path/template" operations the API
// catalog documents as honouring an Idempotency-Key (true), plus explicit
// literal routes that must NOT be keyed (false) because a listed template
// would otherwise shadow them (for example "PATCH /users/me" next to
// "PATCH /users/{id}"). The public spec documents none today, so no mutation
// is ever replayed. Add an entry only with the API contract behind it; the CLI
// then generates the key itself. "{x}" matches any one path segment. The most
// specific match (fewest wildcards) decides; TestAllowlistTemplatesDoNotShadowSpecLiterals
// fails the build if a template shadows an unlisted literal route in spec.yaml.
var idempotentEndpoints = map[string]bool{}

func isReadMethod(method string) bool {
	m := strings.ToUpper(method)
	return m == http.MethodGet || m == http.MethodHead
}

func isTemplateSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

func endpointHonoursIdempotencyKey(method, path string) bool {
	if len(idempotentEndpoints) == 0 {
		return false
	}
	path, _, _ = strings.Cut(path, "?")
	have := strings.Split(strings.Trim(path, "/"), "/")
	bestWild, bestVal, found := 0, false, false
	for key, val := range idempotentEndpoints {
		m, tmpl, ok := strings.Cut(key, " ")
		if !ok || !strings.EqualFold(m, method) {
			continue
		}
		want := strings.Split(strings.Trim(tmpl, "/"), "/")
		if len(want) != len(have) {
			continue
		}
		wild, match := 0, true
		for i := range want {
			if isTemplateSegment(want[i]) {
				wild++
			} else if want[i] != have[i] {
				match = false
				break
			}
		}
		if match && (!found || wild < bestWild) {
			bestWild, bestVal, found = wild, val, true
		}
	}
	return found && bestVal
}

func newIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "conduyt-cli-" + hex.EncodeToString(b)
}

type replaySafeCtxKey struct{}

func withReplaySafe(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, replaySafeCtxKey{}, v)
}

func replaySafeFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(replaySafeCtxKey{}).(bool)
	return v
}

// retryAfterNumber is digits with an optional single dot that has digits on
// both sides ("5", "1.5"; not ".5", "1.", "+5", "1e3").
var retryAfterNumber = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

// parseRetryAfterHeader reads seconds or an HTTP-date. ok is false when the
// header is missing, malformed, negative, or already in the past.
func parseRetryAfterHeader(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if retryAfterNumber.MatchString(v) {
		secs, err := strconv.ParseFloat(v, 64)
		if err == nil || errors.Is(err, strconv.ErrRange) {
			return secondsToDelay(secs)
		}
		return 0, false
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(retryNow())
		if d < 0 {
			d = 0
		}
		return clampDelay(d), true
	}
	return 0, false
}

func secondsToDelay(secs float64) (time.Duration, bool) {
	if math.IsNaN(secs) || secs < 0 {
		return 0, false
	}
	if math.IsInf(secs, 1) {
		return maxRetryDelay, true
	}
	if secs >= maxRetryDelay.Seconds() {
		return maxRetryDelay, true
	}
	return time.Duration(secs * float64(time.Second)), true
}

func clampDelay(d time.Duration) time.Duration {
	if d > maxRetryDelay {
		return maxRetryDelay
	}
	return d
}

// busyBody is the shape of the API's gate_busy / rate-limit bodies. Both the
// top level and an "error" object are accepted.
type busyBody struct {
	Code       string          `json:"code"`
	RetryAfter json.RawMessage `json:"retryAfter"`
	Error      json.RawMessage `json:"error"`
}

// nonNull treats a JSON null like a missing field.
func nonNull(raw json.RawMessage) json.RawMessage {
	if strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	return raw
}

func parseBusyBody(body []byte) (code string, retryAfter time.Duration, ok bool) {
	var b busyBody
	if json.Unmarshal(body, &b) != nil {
		return "", 0, false
	}
	code = b.Code
	ra := nonNull(b.RetryAfter)
	if len(b.Error) > 0 {
		var inner busyBody
		if json.Unmarshal(b.Error, &inner) == nil {
			if code == "" {
				code = inner.Code
			}
			if len(ra) == 0 {
				ra = nonNull(inner.RetryAfter)
			}
		}
	}
	if len(ra) > 0 {
		var secs float64
		if json.Unmarshal(ra, &secs) == nil {
			retryAfter, ok = secondsToDelay(secs)
		}
	}
	return code, retryAfter, ok
}

// serverRetryDelay returns the wait the server asked for: the Retry-After
// header wins, then the body's retryAfter. Up to 50% jitter is added, and the
// result never exceeds maxRetryDelay. ok is false when the server gave none.
func serverRetryDelay(h http.Header, body []byte) (time.Duration, bool) {
	base, ok := parseRetryAfterHeader(h.Get("Retry-After"))
	if !ok {
		_, base, ok = parseBusyBody(body)
	}
	if !ok {
		return 0, false
	}
	return clampDelay(base + time.Duration(retryJitter()*0.5*float64(base))), true
}

func isGateBusy(body []byte) bool {
	code, _, _ := parseBusyBody(body)
	return code == "gate_busy"
}

func secondsLabel(d time.Duration) int {
	return int(math.Ceil(d.Seconds()))
}
