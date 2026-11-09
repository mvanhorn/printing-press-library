// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(redirect-and-redaction-policy) + PATCH(retry-policy): the hand-written redaction set (request-aware credential
// masks, maskedError identity, MaskURL / MaskPath / MaskErrorForURL, sensitive path and body fields) and the HTTP/1-only
// mutation client, carried beside the 4.34.2 print's client.go (which absorbed the replay rule, the retry budget, the
// redirect guards and the text/CSV response envelope). See .printing-press-patches.json.

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"
)

// PATCH(redirect-and-redaction-policy): cross-origin redirect header allowlist and origin comparison.
// redirectSafeHeaders are the only request headers forwarded on a cross-origin redirect.
var redirectSafeHeaders = map[string]bool{
	"Accept": true, "Accept-Encoding": true, "Accept-Language": true,
	"Content-Type": true, "Content-Length": true, "User-Agent": true,
	"X-Request-Id": true, "X-Correlation-Id": true, "Cache-Control": true,
}

// sameOrigin reports whether two URLs share scheme, host and port (default
// ports normalised). https to http on the same host is a different origin.
func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	if !strings.EqualFold(a.Scheme, b.Scheme) {
		return false
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if strings.EqualFold(u.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	return strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// PATCH(retry-policy): mutations the CLI will not replay use a separate HTTP/1-only client (own keep-alive pool, ALPN http/1.1 only).
// httpClientFor picks the HTTP client for one call. Reads and keyed calls use
// the shared pooled client. A mutation the CLI will not replay gets its own
// HTTP/1-only client with a separate keep-alive pool. HTTP/2 is off because
// its stack retries streams on its own; HTTP/1 only retransmits a request that
// is "replayable", which a POST/PUT/PATCH/DELETE without a key header never is
// (doInternal strips both key headers from such calls). Keep-alives stay on so
// bulk writes (import) do not pay a handshake per row.
func (c *Client) httpClientFor(replaySafe bool) *http.Client {
	if replaySafe {
		return c.HTTPClient
	}
	c.mutOnce.Do(func() {
		tr, ok := standardTransport(c.HTTPClient.Transport)
		if !ok {
			c.mutClient = c.HTTPClient
			return
		}
		mt := tr.Clone()
		mt.ForceAttemptHTTP2 = false
		mt.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		mt.Protocols = new(http.Protocols)
		mt.Protocols.SetHTTP1(true)
		// Clone copies a TLS config that advertises h2 via ALPN; a server
		// would pick h2 and this HTTP/1-only transport could not speak it.
		cfg := &tls.Config{}
		if mt.TLSClientConfig != nil {
			cfg = mt.TLSClientConfig.Clone()
		}
		cfg.NextProtos = []string{"http/1.1"}
		mt.TLSClientConfig = cfg
		// Go ignores TLSClientConfig when a custom TLS dialer is set, so its
		// ALPN is out of our hands: check what it negotiated and refuse h2
		// rather than speak HTTP/1 on an h2 connection.
		if orig := mt.DialTLSContext; orig != nil {
			mt.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				ctx, release := linkCallCtx(ctx)
				defer release()
				conn, err := orig(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				return rejectH2Conn(ctx, conn)
			}
		}
		if orig := mt.DialTLS; orig != nil && mt.DialTLSContext == nil { //nolint:staticcheck // preserved from the cloned transport
			// The legacy dialer has no context. Run it so the request's
			// context (caller deadline, --timeout, call budget) still ends the
			// wait, and handshake the result under that same context.
			mt.DialTLS = nil //nolint:staticcheck
			mt.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				ctx, release := linkCallCtx(ctx)
				defer release()
				type dialed struct {
					conn net.Conn
					err  error
				}
				result := make(chan dialed)
				done := make(chan struct{})
				defer close(done)
				go func() {
					conn, err := orig(network, addr)
					select {
					case result <- dialed{conn, err}:
					case <-done:
						// Nobody is waiting any more: close a late connection
						// ourselves and exit.
						if conn != nil {
							_ = conn.Close()
						}
					}
				}()
				select {
				case d := <-result:
					if d.err != nil {
						return nil, d.err
					}
					return rejectH2Conn(ctx, d.conn)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
		cp := *c.HTTPClient
		cp.Transport = mt
		c.mutClient = &cp
	})
	return c.mutClient
}

// PATCH(retry-policy) gocli1219 r2: mutationCarriesCallKey reports a mutation sent with a per-call key header (a
// spec-declared --idempotency-key flag). Such a request is sent but never replayed: not by the CLI (it is not
// replay-safe) and not by Go's transport, which replays a keyed request on a dead reused connection.
//
// gocli1219 r4: method-independent. The caller decides whether the call is replay-safe (the intent flag); this only
// reports the key, so a mutating GET carrying a key counts too.
func mutationCarriesCallKey(method string, headerOverrides map[string]string) bool {
	_ = method
	for _, h := range []string{"Idempotency-Key", "X-Idempotency-Key"} {
		for k, v := range headerOverrides {
			if strings.EqualFold(k, h) && strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

// attemptHTTPClient picks the HTTP client for one attempt (PATCH(retry-policy), gocli1219 r3):
//   - replay-safe calls use the shared pooled client;
//   - a mutation the CLI will not replay uses the HTTP/1-only mutation client;
//   - a mutation carrying a per-call key uses the keep-alive-free HTTP/1 client, or is refused when the transport
//     cannot be proven non-reusing (never the ordinary keep-alive client);
//   - a binary (streaming) response wraps the client chosen above, so its transport policy survives.
//
// streamIdle is the idle window of a streaming read (0 when the call does not stream).
func (c *Client) attemptHTTPClient(method string, headerOverrides map[string]string, replaySafe, binaryResponse bool) (*http.Client, time.Duration, error) {
	httpClient := c.httpClientFor(replaySafe)
	// gocli1219 r4: a call that is not replay-safe takes the no-reuse client when it carries a per-call key OR its wire
	// method is one Go's transport resends on its own after a dead reused connection (GET, HEAD, OPTIONS, TRACE): a
	// mutating GET is replayed by the transport with or without a key.
	if !replaySafe && (mutationCarriesCallKey(method, headerOverrides) || goTransportReplaysMethod(method)) {
		keyed, err := c.noReuseMutationClient()
		if err != nil {
			return nil, 0, err
		}
		httpClient = keyed
	}
	// Silence on a Timeout-0 transfer aborts that attempt. A body that keeps arriving may outlive the window. An
	// explicit timeout, including 0, stays on the client deadline and is only size-limited.
	var streamIdle time.Duration
	if binaryResponse && !c.timeoutExplicit {
		streamIdle = c.ConfiguredTimeout()
		httpClient = StreamingHTTPClient(httpClient, streamIdle)
	}
	return httpClient, streamIdle, nil
}

// goTransportReplaysMethod reports the methods net/http treats as idempotent and so may resend on a dead reused
// connection without any key (gocli1219 r4).
func goTransportReplaysMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, "":
		return true
	}
	return false
}

// standardTransport returns the *http.Transport behind rt; a nil transport is net/http's DefaultTransport (callers
// clone it, never mutate it). Any other RoundTripper cannot be inspected and reports false.
func standardTransport(rt http.RoundTripper) (*http.Transport, bool) {
	if rt == nil {
		tr, ok := http.DefaultTransport.(*http.Transport)
		return tr, ok
	}
	tr, ok := rt.(*http.Transport)
	return tr, ok
}

// errKeyedMutationTransport refuses a keyed mutation whose transport cannot be proven non-reusing
// (PATCH(retry-policy) gocli1219 r3).
type errKeyedMutationTransport struct{ transport string }

func (e errKeyedMutationTransport) Error() string {
	return fmt.Sprintf("refusing to send a call that is not safe to replay (a mutation with an Idempotency-Key, or a state-changing GET): the HTTP transport (%s) is not the standard transport, so it cannot be proven not to reuse a connection and silently resend the request; nothing was sent (use the default transport)", e.transport)
}

// noReuseMutationClient is the HTTP/1-only mutation client with keep-alives off: every keyed mutation dials a fresh
// connection, so the transport never holds a reused connection it could silently resend the request on.
// gocli1219 r3 (fail closed): a nil transport starts from a clone of DefaultTransport; a RoundTripper that is not a
// *http.Transport (a wrapper) cannot be inspected or cloned, so the keyed mutation is refused before anything is
// sent. The ordinary keep-alive client is never returned for a keyed mutation.
func (c *Client) noReuseMutationClient() (*http.Client, error) {
	base := c.httpClientFor(false)
	c.keyedOnce.Do(func() {
		tr, ok := standardTransport(base.Transport)
		if !ok {
			// httpClientFor fell back to the shared client: its transport is not one we can clone.
			c.keyedErr = errKeyedMutationTransport{transport: fmt.Sprintf("%T", c.HTTPClient.Transport)}
			return
		}
		nt := tr.Clone()
		nt.DisableKeepAlives = true
		cp := *base
		cp.Transport = nt
		c.keyedClient = &cp
	})
	return c.keyedClient, c.keyedErr
}

type callCtxKey struct{}

// linkCallCtx returns a context that ends when either ctx or the call context
// carried in ctx's values ends (net/http hands dialers a context that is
// detached from the request's cancellation). It mirrors the call's deadline
// (so ctx.Err() is DeadlineExceeded) and its cancellation cause.
func linkCallCtx(ctx context.Context) (context.Context, func()) {
	callCtx, ok := ctx.Value(callCtxKey{}).(context.Context)
	if !ok || callCtx == nil {
		return ctx, func() {}
	}
	linked, cancelCause := context.WithCancelCause(ctx)
	cancelDeadline := func() {}
	if d, has := callCtx.Deadline(); has {
		linked, cancelDeadline = context.WithDeadline(linked, d)
	}
	stop := context.AfterFunc(callCtx, func() {
		// A deadline is mirrored by the timer above; only a real
		// cancellation needs forwarding, with its cause.
		if errors.Is(callCtx.Err(), context.Canceled) {
			cancelCause(context.Cause(callCtx))
		}
	})
	return linked, func() { stop(); cancelDeadline(); cancelCause(nil) }
}

// errH2OnMutationTransport is returned, before any request byte is written,
// when a custom TLS dialer negotiated HTTP/2 for a mutation.
var errH2OnMutationTransport = errors.New("refusing to send a mutation: the custom TLS dialer negotiated HTTP/2 but mutations are HTTP/1.1 only")

// rejectH2Conn closes conn and fails when it is a TLS connection that
// negotiated h2.
func rejectH2Conn(ctx context.Context, conn net.Conn) (net.Conn, error) {
	if tc, ok := conn.(*tls.Conn); ok {
		if err := tc.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if tc.ConnectionState().NegotiatedProtocol == "h2" {
			_ = conn.Close()
			return nil, errH2OnMutationTransport
		}
	}
	return conn, nil
}

// Is matches target against the original chain with the budgeted, cycle-safe
// walk (errors.Is on the original would never return for a self-unwrapping
// error). Same rules as errors.Is: comparable equality, or a node's own Is.
func (e maskedError) Is(target error) bool {
	if e.orig == nil || target == nil {
		return false
	}
	comparable := reflect.TypeOf(target).Comparable()
	return walkChain(e.orig, func(n error) bool {
		if comparable && reflect.TypeOf(n).Comparable() && n == target {
			return true
		}
		if is, ok := n.(interface{ Is(error) bool }); ok && is.Is(target) {
			return true
		}
		return false
	})
}

// chainFind returns the first node of the original chain (budgeted walk) that
// has type T.
func chainFind[T any](orig error) (T, bool) {
	var found T
	ok := walkChain(orig, func(n error) bool {
		if v, is := n.(T); is {
			found = v
			return true
		}
		return false
	})
	return found, ok
}

// chainFindAs is chainFind for pointer-typed targets, and additionally honours
// a node's own shallow As method (the way errors.As does): a node whose As
// fills a T counts as a match. The caller sanitizes the returned value.
func chainFindAs[T any](orig error) (T, bool) {
	var found T
	ok := walkChain(orig, func(n error) bool {
		if v, is := n.(T); is {
			found = v
			return true
		}
		if a, is := n.(interface{ As(any) bool }); is {
			var tmp T
			if a.As(&tmp) && !reflect.ValueOf(tmp).IsNil() {
				found = tmp
				return true
			}
		}
		return false
	})
	return found, ok
}

func (e maskedError) Timeout() bool {
	t, ok := chainFind[interface{ Timeout() bool }](e.orig)
	return ok && t.Timeout()
}

func (e maskedError) Temporary() bool {
	t, ok := chainFind[interface{ Temporary() bool }](e.orig)
	return ok && t.Temporary()
}

// safeSentinels carry no text beyond their fixed message.
var safeSentinels = []error{context.Canceled, context.DeadlineExceeded, io.EOF, io.ErrUnexpectedEOF, os.ErrDeadlineExceeded, net.ErrClosed}

// maxUnwrapNodes is the traversal budget for every walk over the ORIGINAL
// error chain (Unwrap, Is, As, Timeout, Temporary): at most this many errors
// are inspected. It bounds cycles (an error whose Unwrap returns itself) and is
// far above any real chain or errors.Join tree; whatever lies beyond the
// budget is simply not seen.
const maxUnwrapNodes = 4096

// walkChain visits err and everything reachable through Unwrap() error and
// Unwrap() []error, depth first in exactly errors.Is order, at most
// maxUnwrapNodes nodes, and stops (true) as soon as visit returns true.
// Traversal is lazy: a joined error keeps one frame (its children slice plus an
// index) and the next child is taken only when the previous child's subtree is
// exhausted, so the budget bounds visited nodes, never queued children, and
// memory is bounded by depth plus one frame per join.
func walkChain(err error, visit func(error) bool) bool {
	found, _ := walkChainPeak(err, visit)
	return found
}

// chainFrame is the unvisited remainder of one joined error.
type chainFrame struct {
	kids []error
	next int
}

// walkChainPeak is walkChain that also reports the largest frame-stack
// length, so tests can prove the traversal is memory-bounded.
func walkChainPeak(err error, visit func(error) bool) (found bool, peak int) {
	var frames []chainFrame
	n := err
	for seen := 0; seen < maxUnwrapNodes; {
		if n == nil {
			// Subtree finished: pull the next pending child, lazily.
			for len(frames) > 0 {
				top := &frames[len(frames)-1]
				if top.next < len(top.kids) {
					n = top.kids[top.next]
					top.next++
					break
				}
				frames = frames[:len(frames)-1]
			}
			if n == nil {
				if len(frames) == 0 {
					return false, peak
				}
				continue
			}
		}
		cur := n
		n = nil
		seen++
		if visit(cur) {
			return true, peak
		}
		switch u := cur.(type) {
		case interface{ Unwrap() error }:
			n = u.Unwrap()
		case interface{ Unwrap() []error }:
			if kids := u.Unwrap(); len(kids) > 0 {
				frames = append(frames, chainFrame{kids: kids})
				if len(frames) > peak {
					peak = len(frames)
				}
			}
		}
	}
	return false, peak
}

// Unwrap returns the text-free classes found in the original chain (within
// maxUnwrapNodes). Contract: only syscall.Errno values and safeSentinels are
// ever returned; never the original error or any error carrying text.
func (e maskedError) Unwrap() []error {
	if e.orig == nil {
		return nil
	}
	var out []error
	walkChain(e.orig, func(n error) bool {
		if en, ok := n.(syscall.Errno); ok {
			out = append(out, en)
		}
		for _, s := range safeSentinels {
			if n == s {
				out = append(out, s)
			}
		}
		return false
	})
	return out
}

func (e maskedError) wrap(err error) error {
	if err == nil {
		return nil
	}
	return maskedError{msg: e.mask(err.Error()), orig: err, mask: e.mask}
}

// maskedAddr is a net.Addr whose text is masked.
type maskedAddr struct {
	network, text string
}

func (a maskedAddr) Network() string { return a.network }

func (a maskedAddr) String() string { return a.text }

func (e maskedError) addr(a net.Addr) net.Addr {
	if a == nil {
		return nil
	}
	return maskedAddr{network: e.mask(a.Network()), text: e.mask(a.String())}
}

func (e maskedError) As(target any) bool {
	if e.orig == nil {
		return false
	}
	switch t := target.(type) {
	case **url.Error:
		if ue, ok := chainFindAs[*url.Error](e.orig); ok {
			*t = &url.Error{Op: e.mask(ue.Op), URL: e.mask(ue.URL), Err: e.wrap(ue.Err)}
			return true
		}
	case **net.OpError:
		if oe, ok := chainFindAs[*net.OpError](e.orig); ok {
			*t = &net.OpError{Op: e.mask(oe.Op), Net: e.mask(oe.Net), Source: e.addr(oe.Source), Addr: e.addr(oe.Addr), Err: e.wrap(oe.Err)}
			return true
		}
	case **net.DNSError:
		if de, ok := chainFindAs[*net.DNSError](e.orig); ok {
			*t = &net.DNSError{
				Err: e.mask(de.Err), Name: e.mask(de.Name), Server: e.mask(de.Server),
				IsTimeout: de.IsTimeout, IsTemporary: de.IsTemporary, IsNotFound: de.IsNotFound,
				UnwrapErr: e.wrap(de.UnwrapErr),
			}
			return true
		}
	}
	return false
}

type credentialMask struct {
	needle      string
	replacement string
}

// credentialMasks lists every credential form to hide, longest first.
func (c *Client) credentialMasks(extraCredentials ...string) []credentialMask {
	var masks []credentialMask
	seen := map[string]struct{}{}
	addValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		const replacement = "****" // never a fragment of any credential
		addMask := func(needle string) {
			if needle == "" {
				return
			}
			if _, ok := seen[needle]; ok {
				return
			}
			seen[needle] = struct{}{}
			masks = append(masks, credentialMask{needle: needle, replacement: replacement})
		}
		addMask(value)
		// JSON-escaped forms a server may echo the secret in.
		if raw, err := json.Marshal(value); err == nil && len(raw) >= 2 {
			base := string(raw[1 : len(raw)-1])
			addMask(base)
			slashed := strings.ReplaceAll(base, "/", `\/`)
			addMask(slashed)
			addMask(strings.ReplaceAll(base, "-", `\u002d`))
			addMask(strings.ReplaceAll(slashed, "-", `\u002d`))
		}
		if escaped := url.QueryEscape(value); escaped != value {
			addMask(escaped)
		}
		if escaped := url.PathEscape(value); escaped != value {
			addMask(escaped)
		}
	}
	addCredential := func(value string) {
		value = strings.TrimSpace(value)
		addValue(value)
		if _, token, ok := strings.Cut(value, " "); ok {
			addValue(token)
		}
	}
	for _, value := range extraCredentials {
		addCredential(value)
	}
	if c != nil && c.Config != nil {
		addCredential(c.Config.AuthHeaderVal)
		addCredential(c.Config.AuthHeader())
		addCredential(c.Config.AccessToken)
		addCredential(c.Config.RefreshToken)
		addCredential(c.Config.ClientSecret)
		addCredential(c.Config.ConduytCrmBearerAuth)
	}
	sort.SliceStable(masks, func(i, j int) bool {
		return len(masks[i].needle) > len(masks[j].needle)
	})
	return masks
}

// PATCH(redirect-and-redaction-policy): per-attempt credential redaction (mask set, lazy, budgeted).
// maskSet is the credential masks for one request attempt, built once:
// every form of every secret, longest first, behind one single-pass replacer.
type maskSet struct {
	masks   []credentialMask
	repl    *strings.Replacer
	longest int
	// overflow: the request had more secrets than the complexity budget
	// allows us to track, so NOTHING derived from it can be redacted safely:
	// the body copy, the error text and the displayed path are all withheld.
	overflow bool
}

// maskSetBuilds counts mask-set constructions (tests assert that success
// paths build none).
var maskSetBuilds int32

// lazyMasks builds its maskSet on first use.
type lazyMasks struct {
	build func() *maskSet
	ms    *maskSet
}

func (l *lazyMasks) get() *maskSet {
	if l.ms == nil {
		l.ms = l.build()
	}
	return l.ms
}

func (l *lazyMasks) mask(text string) string { return l.get().mask(text) }

func (l *lazyMasks) maskErr(err error) error { return l.get().maskErr(err) }

func (l *lazyMasks) body(b []byte, interrupted bool) string { return l.get().body(b, interrupted) }

func (c *Client) newMaskSet(extraCredentials ...string) *maskSet {
	atomic.AddInt32(&maskSetBuilds, 1)
	masks := c.credentialMasks(extraCredentials...)
	ms := &maskSet{masks: masks}
	pairs := make([]string, 0, 2*len(masks))
	for _, m := range masks {
		pairs = append(pairs, m.needle, m.replacement)
		if len(m.needle) > ms.longest {
			ms.longest = len(m.needle)
		}
	}
	ms.repl = strings.NewReplacer(pairs...)
	return ms
}

// withheldNote replaces any text that cannot be redacted safely.
const withheldNote = "[withheld: this request carried too many credentials to redact it safely]"

func (ms *maskSet) mask(text string) string {
	if text == "" {
		return text
	}
	if ms.overflow {
		return withheldNote
	}
	if len(ms.masks) == 0 {
		return text
	}
	return ms.repl.Replace(text)
}

func (ms *maskSet) maskErr(err error) error {
	if err == nil {
		return nil
	}
	if len(ms.masks) == 0 && !ms.overflow {
		return err // nothing to hide
	}
	// Always wrap: a secret can sit in a typed field that Error() never
	// prints (DNSError.UnwrapErr, Addr.Network()), and errors.As must hand out
	// sanitized clones, never the original.
	return maskedError{msg: ms.mask(err.Error()), orig: err, mask: ms.mask}
}

// minCredentialPrefix is the shortest cut-off credential prefix masked at the
// end of a body. A short coincidental match is an accepted cost.
const minCredentialPrefix = 3

// body returns a display copy of a response body: at most truncateLimit bytes
// (plus "..." when cut), credentials masked. One pass: the input is first cut
// to the limit plus the longest credential (so a token straddling the limit is
// still whole), then masked, then cut to the limit. When the end of the data
// is shown (interrupted read or an input cut that masking shrank under the
// limit), a dangling credential prefix of 3+ bytes becomes a fixed "****".
func (ms *maskSet) body(b []byte, interrupted bool) string {
	if ms.overflow {
		return withheldNote
	}
	in := b
	inputCut := false
	if limit := truncateLimit + ms.longest; len(in) > limit {
		in = in[:limit]
		inputCut = true
	}
	text := ms.mask(string(in))
	needTail := interrupted || inputCut
	truncated := inputCut
	if len(text) > truncateLimit {
		text = text[:truncateLimit]
		truncated = true
		needTail = true // the cut may land in the unmasked residue of an input cut
	}
	if truncated {
		text = strings.ToValidUTF8(text, "")
	}
	if needTail {
		for _, m := range ms.masks {
			for k := len(m.needle) - 1; k >= minCredentialPrefix; k-- {
				if strings.HasSuffix(text, m.needle[:k]) {
					text = strings.TrimSuffix(text, m.needle[:k]) + "****"
					break
				}
			}
		}
	}
	if len(text) > truncateLimit {
		// "****" can be longer than the 3-byte prefix it replaced.
		text = text[:truncateLimit]
		truncated = true
	}
	if truncated {
		text = strings.ToValidUTF8(text, "") + "..."
	}
	return text
}

// PATCH(redirect-and-redaction-policy): word-based secret name rule.
// nameWords splits a header, query or body-field name into lowercase words at
// delimiters, camelCase, acronym and digit boundaries: "X-API-Key" -> x api key,
// "APIToken" -> api token, "apiKEY" -> api key, "SessionID" -> session id,
// "apiKey2" -> api key 2, "keyword" -> keyword.
func nameWords(name string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	rs := []rune(name)
	for i, r := range rs {
		isAlnum := unicode.IsLetter(r) || unicode.IsDigit(r)
		if !isAlnum {
			flush()
			continue
		}
		if len(cur) > 0 {
			prev := cur[len(cur)-1]
			switch {
			case unicode.IsDigit(r) != unicode.IsDigit(prev):
				flush()
			case unicode.IsUpper(r) && unicode.IsLower(prev):
				flush()
			case unicode.IsUpper(r) && unicode.IsUpper(prev) && i+1 < len(rs) && unicode.IsLower(rs[i+1]):
				flush() // end of an acronym: APIToken -> API, Token
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

// isSensitiveName reports whether a header, query or body-field name carries a
// secret. Whole words and deliberate qualifier rules, not substrings, so
// "keyword", "passengers", "author", "fieldKey" and "zipCode" stay readable.
func isSensitiveName(name string) bool {
	words := nameWords(name)
	for _, w := range words {
		if secretWords[w] {
			return true
		}
	}
	qualified := func(target string, qualifiers map[string]bool) bool {
		for i, w := range words {
			if w != target && !(target == "key" && w == "keys") {
				continue
			}
			if len(words) == 1 {
				return w == target // a bare "keys" stays readable
			}
			for j, q := range words {
				if j != i && qualifiers[q] {
					return true
				}
			}
		}
		return false
	}
	// "two factor" / "2 fa" spelled as separate words.
	joined := strings.Join(words, "")
	if strings.Contains(joined, "twofactor") || strings.Contains(joined, "2fa") {
		return true
	}
	return qualified("key", keyQualifiers) || (!(len(words) == 1) && qualified("code", codeQualifiers))
}

// minBodySecretLen skips tiny query/body values ("a", "1"): masking them
// would shred ordinary text. Header values are always masked.
const minBodySecretLen = 4

// Complexity budget for one request's secret set. Past it the error body is
// withheld rather than redacted incompletely.
const (
	maxSecrets     = 256
	maxSecretBytes = 64 << 10
)

// secretSink collects distinct secrets within the budget.
type secretSink struct {
	seen     map[string]struct{}
	list     []string
	bytes    int
	overflow bool
}

func (k *secretSink) add(v string) {
	if v == "" {
		return
	}
	if _, dup := k.seen[v]; dup {
		return
	}
	if len(k.list) >= maxSecrets || k.bytes+len(v) > maxSecretBytes {
		k.overflow = true
		return
	}
	if k.seen == nil {
		k.seen = map[string]struct{}{}
	}
	k.seen[v] = struct{}{}
	k.list = append(k.list, v)
	k.bytes += len(v)
}

// addURL collects what a URL itself can carry: a userinfo password, the
// secret path parameters (sensitivePathParams), the values of sensitive query
// parameters in BOTH raw and decoded form (raw pairs are walked, so lowercase
// percent-escapes and malformed queries are covered), and sensitive fragment
// values (#access_token=...).
func (k *secretSink) addURL(u *url.URL) {
	if u == nil {
		return
	}
	if u.Opaque != "" { // mailto:x?token=y and the like: no structure we can trust
		k.addUnsafe(u.String())
		return
	}
	if u.User != nil {
		if pw, ok := u.User.Password(); ok {
			k.add(pw)
		} else {
			k.add(u.User.Username()) // https://TOKEN@host: the bare userinfo is the credential
		}
	}
	for _, v := range sensitivePathValues(u.EscapedPath()) {
		k.add(v)
	}
	k.addPairs(u.RawQuery)
	k.addPairs(u.EscapedFragment()) // the raw form: lowercase escapes survive
	k.addPairs(u.Fragment)
}

// addUnsafe treats a URL-like value we cannot interpret as wholly secret: the
// whole value, plus every k=v value inside it (split on & ; ? #), so a bare
// echoed value is masked even when the surrounding URL is not.
func (k *secretSink) addUnsafe(raw string) {
	k.add(raw)
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '&' || r == ';' || r == '?' || r == '#' }) {
		if _, val, ok := strings.Cut(part, "="); ok && len(val) >= minBodySecretLen {
			k.add(val)
			if dv, err := url.QueryUnescape(val); err == nil && len(dv) >= minBodySecretLen {
				k.add(dv)
			}
		}
	}
}

// addPairs walks "a=b&c=d" text and collects the values of sensitive names.
func (k *secretSink) addPairs(raw string) {
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == '&' || r == ';' }) {
		name, val, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if dn, err := url.QueryUnescape(name); err == nil {
			name = dn
		}
		if !isSensitiveName(name) {
			continue
		}
		if len(val) >= minBodySecretLen {
			k.add(val)
		}
		if dv, err := url.QueryUnescape(val); err == nil && len(dv) >= minBodySecretLen {
			k.add(dv)
		}
	}
}

var linkTarget = regexp.MustCompile(`<([^>]*)>`)

// refreshURL matches the Refresh header form "0; url=/target".
var refreshURL = regexp.MustCompile(`(?i)^\s*\d+\s*;\s*url\s*=\s*['"]?([^'"]+?)['"]?\s*$`)

// addURLValue collects from one URL-bearing header value (Location, Referer,
// Content-Location, Link). If it cannot be parsed, the WHOLE value is treated
// as a secret: fail closed.
func (k *secretSink) addURLValue(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	candidates := []string{value}
	if m := refreshURL.FindStringSubmatch(value); m != nil {
		candidates = []string{m[1]} // Refresh: 0; url=...
	} else if m := linkTarget.FindAllStringSubmatch(value, -1); len(m) > 0 {
		candidates = candidates[:0]
		for _, mm := range m {
			candidates = append(candidates, mm[1])
		}
	}
	for _, cand := range candidates {
		u, err := url.Parse(cand)
		if err != nil {
			k.addUnsafe(cand)
			continue
		}
		k.addURL(u)
	}
}

// PATCH(redirect-and-redaction-policy): explicit list of secret path parameters.
// sensitivePathParams lists path templates with secret path parameters. The
// public API catalog has no generic marker, so this is explicit; a spec guard
// test fails when a spec path has a sensitive-looking parameter (or "code")
// that is not listed here.
var sensitivePathParams = map[string][]string{
	"/public/screen-share/{code}":       {"code"},  // public screen-share access code
	"/screen-share/session/{code}":      {"code"},  // screen-share session code
	"/public/screen-share/{code}/state": {"code"},  // public screen-share access code (reprint637: new public path)
	"/webhooks/inbound/forms/{token}":   {"token"}, // inbound form webhook token (reprint637: new public path)
}

// PATCH(reprint-637-sensitive-body-fields): body fields secret on one operation only.
// sensitiveBodyFields lists path templates whose top-level JSON body fields carry
// a credential although their names read as ordinary. PUT /chat/agents/{id}/webhook
// (Grok Bot connect): `paste` is the whole pasted block (the webhook URL and its
// crsr_ key), `address` is the webhook URL. Globally sensitive names (`key`, ...)
// are already masked by isSensitiveName; these are masked only on these paths, so
// an email `address` elsewhere stays readable.
var sensitiveBodyFields = map[string][]string{
	"/chat/agents/{id}/webhook": {"paste", "address"},
}

// sensitiveBodyFieldNames returns the operation-scoped secret body fields of a concrete path.
func sensitiveBodyFieldNames(path string) map[string]bool {
	path, _, _ = strings.Cut(path, "?")
	have := strings.Split(strings.Trim(path, "/"), "/")
	out := map[string]bool{}
	for tmpl, fields := range sensitiveBodyFields {
		want := strings.Split(strings.Trim(tmpl, "/"), "/")
		if len(want) != len(have) {
			continue
		}
		match := true
		for i, seg := range want {
			if !isTemplateSegment(seg) && seg != have[i] {
				match = false
				break
			}
		}
		if match {
			for _, f := range fields {
				out[f] = true
			}
		}
	}
	return out
}

// redactBodyFields masks the operation-scoped secret fields of a decoded top-level body.
func redactBodyFields(doc any, path string) any {
	m, ok := doc.(map[string]any)
	if !ok {
		return doc
	}
	names := sensitiveBodyFieldNames(path)
	if len(names) == 0 {
		return doc
	}
	for k, v := range m {
		if names[k] {
			m[k] = redactJSON(v, true)
		}
	}
	return m
}

// addScopedBodySecrets adds the operation-scoped secret field values of a JSON body
// to the sink: the whole value, every whitespace-separated piece of it (a paste
// holds the URL and the key on separate lines, and the server may echo either one
// alone) and what a URL piece carries in its query.
func addScopedBodySecrets(sink *secretSink, path string, bodyBytes []byte) {
	names := sensitiveBodyFieldNames(path)
	if len(names) == 0 || len(bodyBytes) == 0 {
		return
	}
	var m map[string]any
	if json.Unmarshal(bodyBytes, &m) != nil {
		return
	}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if len(t) >= minBodySecretLen {
				sink.add(t)
			}
			for _, piece := range strings.Fields(t) {
				if len(piece) >= minBodySecretLen {
					sink.addUnsafe(piece)
				}
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	for k, v := range m {
		if names[k] {
			walk(v)
		}
	}
}

// sensitivePathValues returns the secret path-parameter values of a concrete path.
func sensitivePathValues(path string) []string {
	path, _, _ = strings.Cut(path, "?")
	have := strings.Split(strings.Trim(path, "/"), "/")
	var out []string
	for tmpl, params := range sensitivePathParams {
		want := strings.Split(strings.Trim(tmpl, "/"), "/")
		if len(want) != len(have) {
			continue
		}
		match := true
		var vals []string
		for i, seg := range want {
			if isTemplateSegment(seg) {
				for _, p := range params {
					if seg == "{"+p+"}" {
						vals = append(vals, have[i])
					}
				}
			} else if seg != have[i] {
				match = false
				break
			}
		}
		if match {
			for _, v := range vals {
				out = append(out, v)
				if un, err := url.PathUnescape(v); err == nil && un != v {
					out = append(out, un)
				}
			}
		}
	}
	return out
}

// sentSecrets collects every secret in the request as actually sent, in the
// redirect targets, and in the response's cookies, within the budget. The
// cheap and most likely sources go first, so the request's own URL, path and
// query secrets are in the set before any flood of header, cookie or body
// secrets can trigger an overflow (which withholds all output, see maskSet).
// The body is read from its typed form (multipart fields) or its
// JSON encoding with numbers kept as written.
func (c *Client) sentSecrets(req *http.Request, resp *http.Response, doErr error, body any, bodyBytes []byte, params map[string]string, path, authHeader string) *secretSink {
	sink := &secretSink{}
	// Order matters under the budget: what names the request itself (auth,
	// path, URL, query params) goes first so a flood of header, cookie or
	// body secrets can never push it out.
	sink.add(authHeader)
	for _, v := range sensitivePathValues(path) {
		sink.add(v)
	}
	sink.addURL(req.URL)
	for k, v := range params {
		if isSensitiveName(k) && len(v) >= minBodySecretLen {
			sink.add(v)
		}
	}
	var ue *url.Error
	if errors.As(doErr, &ue) { // a redirect refused by CheckRedirect names its target
		sink.addURLValue(ue.URL)
	}
	addReq := func(r *http.Request) {
		if r == nil {
			return
		}
		if r != req {
			sink.addURL(r.URL)
		}
		for _, name := range []string{"Referer", "Link", "Location", "Content-Location"} {
			for _, v := range r.Header.Values(name) {
				sink.addURLValue(v)
			}
		}
		for _, v := range sensitiveHeaderValues(r.Header) {
			sink.add(v)
		}
		for _, ck := range r.Cookies() { // a manually supplied Cookie header, value by value
			sink.add(ck.Value)
		}
		if r.URL != nil && c.HTTPClient != nil && c.HTTPClient.Jar != nil {
			for _, ck := range c.HTTPClient.Jar.Cookies(r.URL) {
				sink.add(ck.Value)
			}
		}
	}
	addReq(req)
	addResp := func(rs *http.Response) {
		for _, ck := range rs.Cookies() {
			sink.add(ck.Value)
		}
		for _, name := range []string{"Location", "Content-Location", "Link", "Refresh"} {
			for _, v := range rs.Header.Values(name) {
				sink.addURLValue(v)
			}
		}
	}
	if resp != nil {
		addResp(resp)
		// Walk the WHOLE redirect chain: each hop's request names its URL,
		// and its Referer / Response name the hops before it.
		visited := map[*http.Request]bool{}
		for r := resp.Request; r != nil && !visited[r]; {
			visited[r] = true
			if r != req {
				addReq(r)
			}
			prev := r.Response
			if prev == nil {
				break
			}
			addResp(prev)
			r = prev.Request
		}
	}
	if mp, ok := body.(multipartRequestBody); ok {
		for k, v := range mp.Fields {
			if isSensitiveName(k) && len(v) >= minBodySecretLen {
				sink.add(v)
			}
		}
	} else if len(bodyBytes) > 0 {
		// Walk the tokens, not a decoded map: a duplicate key
		// ({"password":"a","password":"b"}) must yield BOTH values.
		dec := json.NewDecoder(bytes.NewReader(bodyBytes))
		dec.UseNumber()
		collectJSONSecrets(dec, sink, false)
		addScopedBodySecrets(sink, path, bodyBytes) // PATCH(reprint-637-sensitive-body-fields)
	}
	return sink
}

// PATCH(redirect-and-redaction-policy): exported for generated command envelopes.
// redactJSON returns a copy of a decoded JSON document with every value under
// a sensitive name (or inside a sensitive container) replaced by "****".
func redactJSON(v any, inherited bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = redactJSON(val, inherited || isSensitiveName(k))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactJSON(val, inherited)
		}
		return out
	case string, json.Number:
		if inherited {
			return "****"
		}
	}
	return v
}

// MaskURL redacts a URL for display: userinfo, sensitive query values and
// sensitive fragment values become "****"; an unparseable URL is withheld.
func MaskURL(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "[unparseable URL withheld]"
	}
	if u.Opaque != "" {
		return "[opaque URL withheld]"
	}
	if u.User != nil {
		u.User = url.User("****")
	}
	// Query and fragment pairs split on both & and ; (delimiters kept).
	redactPairs := func(in string) string {
		var out strings.Builder
		flush := func(pair string) {
			name, _, ok := strings.Cut(pair, "=")
			if ok {
				if dn, err := url.QueryUnescape(name); err == nil {
					name = dn
				}
				if isSensitiveName(name) {
					n, _, _ := strings.Cut(pair, "=")
					pair = n + "=****"
				}
			}
			out.WriteString(pair)
		}
		start := 0
		for i := 0; i < len(in); i++ {
			if in[i] == '&' || in[i] == ';' {
				flush(in[start:i])
				out.WriteByte(in[i])
				start = i + 1
			}
		}
		flush(in[start:])
		return out.String()
	}
	u.RawQuery = redactPairs(u.RawQuery)
	if frag := u.EscapedFragment(); frag != "" {
		u.RawFragment = redactPairs(frag)
		u.Fragment, _ = url.PathUnescape(u.RawFragment)
	}
	// Path secrets are matched on the ESCAPED path (so SECRET%2Fpart is one
	// segment) and written back with valid escaping.
	if masked := MaskPath(u.EscapedPath()); masked != u.EscapedPath() {
		if plain, err := url.PathUnescape(masked); err == nil {
			u.Path, u.RawPath = plain, masked
		}
	}
	out := u.String()
	if strings.HasSuffix(raw, "#") && !strings.HasSuffix(out, "#") {
		out += "#" // an empty fragment delimiter is preserved
	}
	return out
}

// maskSetFromSink turns a collected sink into a mask set, carrying the
// overflow flag so that an over-budget request is withheld, never returned raw.
func (c *Client) maskSetFromSink(sink *secretSink) *maskSet {
	m := c.newMaskSet(sink.list...)
	m.overflow = sink.overflow
	return m
}

// MaskErrorForURL redacts an error raised while talking to a URL outside the
// API client (webhook delivery, the feedback endpoint): the URL's own
// credentials (userinfo, sensitive query/fragment values, an unparseable URL
// as a whole) are masked in the error text and its typed fields. Over the
// secret budget the text is withheld.
func MaskErrorForURL(err error, rawURL string) error {
	if err == nil {
		return nil
	}
	sink := &secretSink{}
	sink.addURLValue(rawURL)
	var c *Client
	return c.maskSetFromSink(sink).maskErr(err)
}

// MaskPath hides the secret path parameters of a request path (public and
// session screen-share codes) for display, envelopes and logs.
func MaskPath(path string) string {
	for _, v := range sensitivePathValues(path) {
		path = strings.ReplaceAll(path, v, "****")
	}
	return path
}

// collectJSONSecrets walks a JSON token stream. Sensitivity is inherited by
// every descendant: {"tokens":["x"]} and {"credentials":{"value":"x"}} both
// yield "x". Strings and numbers count; booleans and null do not. Every
// occurrence of a duplicate key is visited.
func collectJSONSecrets(dec *json.Decoder, out *secretSink, inherited bool) {
	if out.overflow {
		return
	}
	tok, err := dec.Token()
	if err != nil {
		return
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			for dec.More() && !out.overflow {
				keyTok, err := dec.Token()
				if err != nil {
					return
				}
				key, _ := keyTok.(string)
				collectJSONSecrets(dec, out, inherited || isSensitiveName(key))
			}
			_, _ = dec.Token() // }
		case '[':
			for dec.More() && !out.overflow {
				collectJSONSecrets(dec, out, inherited)
			}
			_, _ = dec.Token() // ]
		}
	case string:
		if inherited && len(t) >= minBodySecretLen {
			out.add(t)
		}
	case json.Number:
		if inherited && len(t) >= minBodySecretLen {
			out.add(t.String())
		}
	}
}

const truncateLimit = 4096

// sensitiveHeaderValues lists the values of credential-bearing headers.
func sensitiveHeaderValues(h http.Header) []string {
	var out []string
	for name, vals := range h {
		if !isSensitiveName(name) {
			continue
		}
		for _, v := range vals {
			if strings.TrimSpace(v) != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

var (
	// secretWords are secret on their own.
	secretWords = map[string]bool{
		"token": true, "tokens": true, "secret": true, "secrets": true,
		"password": true, "passwords": true, "passwd": true, "pass": true, "passphrase": true, "passcode": true, "pwd": true,
		"auth": true, "authorization": true, "cookie": true, "cookies": true,
		"signature": true, "credential": true, "credentials": true,
		"session": true, "sessionid": true, "jwt": true, "bearer": true, "apikey": true,
		"otp": true, "totp": true, "pin": true, "csrf": true, "xsrf": true,
		"verifier": true, "sig": true, "sigs": true, "hmac": true, "signatures": true,
	}
	// keyQualifiers make a bare "key" word a secret ("api key", "conduyt key");
	// "fieldKey", "sortKey" and the like are ordinary.
	keyQualifiers = map[string]bool{
		"api": true, "access": true, "secret": true, "private": true, "client": true,
		"auth": true, "conduyt": true, "license": true, "app": true, "service": true,
		"signing": true, "encryption": true, "master": true, "admin": true, "bearer": true,
	}
	// codeQualifiers make a bare "code" word a secret ("verificationCode");
	// "zipCode", "statusCode" and "countryCode" stay readable.
	codeQualifiers = map[string]bool{
		"verification": true, "recovery": true, "security": true, "backup": true, "reset": true,
		"confirmation": true, "login": true, "mfa": true, "sms": true, "activation": true,
		"auth": true, "access": true, "otp": true, "twofactor": true, "2fa": true, "invite": true,
	}
)
