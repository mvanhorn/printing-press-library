// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. See
// .printing-press-patches/unifi-dual-transport.json for the generated-file seams.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
)

// connectorProxySegment is the one line to change after live proof. The Network
// and Protect spec servers say /proxy/<product>/integration, the Site Manager
// narrative says /<product>/integration, and the spec's own path example
// disagrees with its narrative. `reach` reports which shape actually answers.
const connectorProxySegment = "/proxy"

const officialCloudBase = "https://api.ui.com"

// Connector limits from the Site Manager spec.
const (
	connectorRequestsPerWindow = 100
	connectorWindow            = time.Minute
)

// ErrNoConsoleSelected means a console-hosted path has nowhere to go.
var ErrNoConsoleSelected = errors.New("no console selected")

// ErrLocalKeyMissing keeps the cloud key from being offered to a LAN host: the
// two keys are issued separately and a gateway has no business seeing the
// account-wide one.
var ErrLocalKeyMissing = errors.New("local gateway transport needs UNIFI_LOCAL_API_KEY")

// ErrReadOnlyMode is returned for every mutation while UNIFI_READ_ONLY is set.
var ErrReadOnlyMode = errors.New("read-only mode")

// ErrUnsafePath refuses a path that an upstream could resolve somewhere else. The
// connector and the gateway proxies are not under this CLI's control and their
// normalization is unverified, so the decision has to be made before the URL is built.
var ErrUnsafePath = errors.New("unsafe request path")

// ErrPlaceholderBody refuses a body holding a stored placeholder. Local rows hold digests,
// not secrets, and a digest sent as a passphrase would set the WiFi key to a string nobody
// knows, locking every client out. This is the last point that can stop that.
var ErrPlaceholderBody = errors.New("request body holds a stored secret placeholder, not a real value")

// NotSent reports an error raised before any byte left, so nothing can have landed.
// Without this distinction a refusal reads like a timeout: "ambiguous" in the audit log
// and "unresolved" in the journal, sending an operator to look for a write that never was.
func NotSent(err error) bool {
	for _, sentinel := range []error{ErrUnsafePath, ErrPlaceholderBody, ErrNoConsoleSelected, ErrLocalKeyMissing, ErrReadOnlyMode, ErrPlaintextCredential} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

var validEscape = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

// PathHasTraversal reports dot segments, including encoded ones, in a request path. It
// decodes repeatedly because each proxy hop may decode once more, so %252e is a live dot
// to somebody. A path still changing after the last pass is refused rather than assumed
// clean. Only the first pass is strict: after that a bare % is a literal, as in an id of
// "50%25", and must neither fail the id nor stop the search, or it would shield a deeper
// encoded dot. ";" is cut because Tomcat, which serves the classic Network app, reads
// "..;x" as "..". Control characters are refused since a decoded CRLF is header injection
// for any hop that rebuilds the request line.
func PathHasTraversal(path string) bool {
	base, _, _ := strings.Cut(path, "?")
	decoded, err := url.PathUnescape(base)
	if err != nil {
		return true
	}
	stable := false
	for i := 0; i < 4; i++ {
		next := validEscape.ReplaceAllStringFunc(decoded, func(escape string) string {
			value, err := url.PathUnescape(escape)
			if err != nil {
				return escape
			}
			return value
		})
		if next == decoded {
			stable = true
			break
		}
		decoded = next
	}
	if !stable {
		return true
	}
	for _, r := range decoded {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return true
		}
	}
	for _, segment := range strings.Split(decoded, "/") {
		segment, _, _ = strings.Cut(segment, ";")
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

// TransportKind names how one request reaches its destination.
type TransportKind string

const (
	TransportSiteManager TransportKind = "site-manager"
	TransportConnector   TransportKind = "connector"
	TransportLocal       TransportKind = "local"
	TransportPassthrough TransportKind = "passthrough"
)

// TargetOverride carries command-line choices, which outrank env and config.
type TargetOverride struct {
	ConsoleID string
	Gateway   string
	Transport string
}

type requestTarget struct {
	URL     string
	Kind    TransportKind
	Console string
}

type transportState struct {
	mu                sync.Mutex
	override          TargetOverride
	loaded            bool
	console           string
	gateway           string
	localKey          string
	mode              string
	autoSelected      string
	localClient       *http.Client
	localClientErr    error
	onConsoleResolved func(string)
	onMutationSuccess func(method, path string)
}

// SetTargetOverride applies --console, --gateway and --transport. Empty fields
// fall through to env and config, so the MCP binary (no flags) still resolves.
func (c *Client) SetTargetOverride(o TargetOverride) {
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	c.transport.override = o
	c.transport.loaded = false
}

// SetConsoleResolvedHook tells the caller which console this client settled on,
// including one auto-selected from /v1/hosts, so the mirror can key rows by it.
func (c *Client) SetConsoleResolvedHook(fn func(string)) {
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	c.transport.onConsoleResolved = fn
}

// SetMutationSuccessHook lets the CLI react to a successful write without the
// client importing the store. Hanging it here covers generated commands, raw
// and MCP execute alike, which is why mirror cleanup is not wired per command.
func (c *Client) SetMutationSuccessHook(fn func(method, path string)) {
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	c.transport.onMutationSuccess = fn
}

func (c *Client) loadSelection() {
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	c.loadSelectionLocked()
}

func (c *Client) loadSelectionLocked() {
	t := &c.transport
	if t.loaded {
		return
	}
	t.loaded = true
	if c.Config != nil {
		fileAndEnv := c.Config.UniFiTarget()
		t.console, t.gateway, t.localKey, t.mode = fileAndEnv.ConsoleID, fileAndEnv.Gateway, fileAndEnv.LocalAPIKey, fileAndEnv.Transport
	}
	if v := strings.TrimSpace(t.override.ConsoleID); v != "" {
		t.console = v
	}
	if v := strings.TrimSpace(t.override.Gateway); v != "" {
		t.gateway = v
	}
	if v := strings.TrimSpace(t.override.Transport); v != "" {
		t.mode = v
	}
	t.mode = strings.ToLower(strings.TrimSpace(t.mode))
}

// IsConsoleHostedPath reports whether a path lives on a console rather than on
// api.ui.com. /proxy/ is the raw command's home for unofficial controller paths.
func IsConsoleHostedPath(path string) bool {
	for _, prefix := range []string{"/network/integration", "/protect/integration", "/innerspace/integration"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return strings.HasPrefix(path, "/proxy/")
}

func consoleRelativePath(path string) string {
	return strings.TrimPrefix(path, "/proxy")
}

// ConnectorPathFor builds the connector path for a console and console-hosted
// path, with an explicit proxy segment so reach can try both spec shapes.
func ConnectorPathFor(consoleID, path, proxySegment string) string {
	return "/v1/connector/consoles/" + url.PathEscape(consoleID) + proxySegment + consoleRelativePath(path)
}

// NormalizeGateway accepts a bare host, host:port or URL and returns an origin.
func NormalizeGateway(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty gateway")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		// The value is left out: it failed to parse, so a password in it cannot be
		// recognized and removed.
		return "", errors.New("gateway is not a valid host or URL")
	}
	// Userinfo is refused, not dropped: "192.168.1.1@evil.example" reads as a LAN address
	// and dials evil.example, and the value is left out of the message because it may be
	// a real password.
	if u.User != nil {
		return "", errors.New("gateway must not include a user name or password")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("gateway %q must not include a path; /proxy/<product> is added automatically", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && (scheme != "http" || !plaintextLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("gateway %q must use https so the API key is not sent in plaintext", raw)
	}
	return strings.ToLower(u.Scheme) + "://" + u.Host, nil
}

// GatewayMirrorKey is the identity of a gateway however it was spelled, so mirror rows
// and journal entries written through it can be found again. "" when it is not valid.
func GatewayMirrorKey(raw string) string {
	origin, err := NormalizeGateway(raw)
	if err != nil {
		return ""
	}
	return gatewayMirrorKey(origin)
}

func gatewayMirrorKey(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return "gateway:" + origin
	}
	return "gateway:" + u.Host
}

func (c *Client) noteConsoleResolved(key string) {
	c.transport.mu.Lock()
	fn := c.transport.onConsoleResolved
	c.transport.mu.Unlock()
	if fn != nil && key != "" {
		fn(key)
	}
}

// resolveTarget is the choke point every request URL passes through.
func (c *Client) resolveTarget(ctx context.Context, path string) (requestTarget, error) {
	// First, so every route and the explicit connector branch below are covered:
	// /v1/connector/consoles/A/../B must not pass the allowlist as console A.
	if PathHasTraversal(path) {
		return requestTarget{}, fmt.Errorf("%w: %q has a dot segment or encoding that could resolve elsewhere", ErrUnsafePath, path)
	}
	// An explicit connector path (reach's shape probe, the generated connector
	// commands) still spends the per-console budget and must be throttled.
	if rest, ok := strings.CutPrefix(path, "/v1/connector/consoles/"); ok {
		console, _, _ := strings.Cut(rest, "/")
		if decoded, err := url.PathUnescape(console); err == nil {
			console = decoded
		}
		if err := consoleAllowed(console); err != nil {
			return requestTarget{}, err
		}
		return requestTarget{URL: c.BaseURL + path, Kind: TransportConnector, Console: console}, nil
	}
	if !IsConsoleHostedPath(path) {
		return requestTarget{URL: c.BaseURL + path, Kind: TransportSiteManager}, nil
	}
	c.transport.mu.Lock()
	c.loadSelectionLocked()
	console, gateway, mode, auto := c.transport.console, c.transport.gateway, c.transport.mode, c.transport.autoSelected
	c.transport.mu.Unlock()

	switch mode {
	case "", "auto", "cloud", "local":
	default:
		return requestTarget{}, fmt.Errorf("%w: --transport %q must be auto, cloud or local", ErrNoConsoleSelected, mode)
	}
	if mode == "local" && gateway == "" {
		return requestTarget{}, fmt.Errorf("%w: --transport local needs --gateway, UNIFI_GATEWAY or config gateway", ErrNoConsoleSelected)
	}
	if gateway != "" && mode != "cloud" {
		origin, err := NormalizeGateway(gateway)
		if err != nil {
			return requestTarget{}, fmt.Errorf("%w: %v", ErrNoConsoleSelected, err)
		}
		// An operator-configured gateway with no console named stays outside the
		// allowlist, which lists console ids. A named console is still checked, since
		// that name labels mirror rows and journal entries.
		if console != "" {
			if err := consoleAllowed(console); err != nil {
				return requestTarget{}, err
			}
		}
		key := console
		if key == "" {
			// Without a console id the gateway host is the only identity
			// available. Setting both keeps cloud and local rows of one
			// console together.
			key = gatewayMirrorKey(origin)
		}
		c.noteConsoleResolved(key)
		return requestTarget{URL: origin + "/proxy" + consoleRelativePath(path), Kind: TransportLocal, Console: key}, nil
	}
	if console == "" {
		console = auto
	}
	if console == "" {
		// A custom base URL means a mock or an operator-run proxy that already
		// knows its own routing; verify and dogfood mocks rely on this.
		if strings.TrimRight(c.BaseURL, "/") != officialCloudBase {
			// A fence cannot be honored for a console nobody named, so it fails closed.
			if err := consoleAllowed(""); err != nil {
				return requestTarget{}, err
			}
			return requestTarget{URL: c.BaseURL + path, Kind: TransportPassthrough}, nil
		}
		if c.DryRun {
			// A preview must not spend a request on /v1/hosts.
			return requestTarget{URL: c.BaseURL + "/v1/connector/consoles/{consoleId}" + connectorProxySegment + consoleRelativePath(path), Kind: TransportConnector}, nil
		}
		selected, err := c.autoSelectConsole(ctx)
		if err != nil {
			return requestTarget{}, err
		}
		console = selected
	}
	if err := consoleAllowed(console); err != nil {
		return requestTarget{}, err
	}
	// PathEscape leaves ".." alone, and this value becomes a segment of the connector path.
	if PathHasTraversal("/" + console) {
		return requestTarget{}, fmt.Errorf("%w: console id %q", ErrUnsafePath, console)
	}
	c.noteConsoleResolved(console)
	return requestTarget{URL: c.BaseURL + ConnectorPathFor(console, path, connectorProxySegment), Kind: TransportConnector, Console: console}, nil
}

// consoleAllowed backs UNIFI_CONSOLE_ALLOWLIST. An organization key reaches every
// console in the org, so a remote MCP deployment needs a way to fence an agent to
// the consoles it was meant to touch, enforced where the URL is built.
func consoleAllowed(console string) error {
	raw := strings.TrimSpace(cliutil.EnvOverride("UNIFI_CONSOLE_ALLOWLIST"))
	if raw == "" {
		return nil
	}
	for _, allowed := range strings.Split(raw, ",") {
		if strings.TrimSpace(allowed) == console {
			return nil
		}
	}
	return fmt.Errorf("%w: console %q is not in UNIFI_CONSOLE_ALLOWLIST", ErrNoConsoleSelected, console)
}

// ConsoleAllowed lets local-only commands honor the fence, since they build no URL.
func ConsoleAllowed(console string) error { return consoleAllowed(console) }

// ConsoleExplicitlyAllowed differs from ConsoleAllowed on an empty list. No list means no
// fence there, but it is not the operator opting a console in past a pin.
func ConsoleExplicitlyAllowed(console string) bool {
	return strings.TrimSpace(cliutil.EnvOverride("UNIFI_CONSOLE_ALLOWLIST")) != "" && consoleAllowed(console) == nil
}

// ConnectorProxySegment exposes the current path shape to reach and fleet.
func ConnectorProxySegment() string { return connectorProxySegment }

// AlternateConnectorProxySegment is the other shape the docs describe.
func AlternateConnectorProxySegment() string {
	if connectorProxySegment == "" {
		return "/proxy"
	}
	return ""
}

// ResolveRoute settles console and transport before a write, so the journal can
// record them in the intent row that must be durable before anything is sent.
func (c *Client) ResolveRoute(ctx context.Context, path string) (console string, kind TransportKind, err error) {
	target, err := c.resolveTarget(ctx, path)
	if err != nil {
		return "", "", err
	}
	return target.Console, target.Kind, nil
}

// PreviewTarget returns the URL a request would use, without any network call,
// for previews that must describe a request they will not send.
func (c *Client) PreviewTarget(ctx context.Context, path string) (string, TransportKind, error) {
	wasDryRun := c.DryRun
	c.DryRun = true
	defer func() { c.DryRun = wasDryRun }()
	target, err := c.resolveTarget(ctx, path)
	if err != nil {
		return "", "", err
	}
	return target.URL, target.Kind, nil
}

// autoSelectConsole picks the only visible console. More than one is an error,
// never a guess: acting on the wrong console is worse than asking.
func (c *Client) autoSelectConsole(ctx context.Context) (string, error) {
	data, _, err := c.doInternalRaw(ctx, http.MethodGet, "/v1/hosts", nil, nil, nil, true, false)
	if err != nil {
		return "", fmt.Errorf("%w: could not list hosts to auto-select one (%v). Set --console, UNIFI_CONSOLE_ID or config console_id, or use --gateway for local access", ErrNoConsoleSelected, err)
	}
	hosts := ParseHostSummaries(data)
	switch len(hosts) {
	case 0:
		return "", fmt.Errorf("%w: /v1/hosts returned no consoles for this key. Check the key's scope, or use --gateway with UNIFI_LOCAL_API_KEY for local access", ErrNoConsoleSelected)
	case 1:
		c.transport.mu.Lock()
		c.transport.autoSelected = hosts[0].ID
		c.transport.mu.Unlock()
		return hosts[0].ID, nil
	}
	lines := make([]string, 0, len(hosts))
	for i, h := range hosts {
		if i == 10 {
			lines = append(lines, fmt.Sprintf("  ... and %d more", len(hosts)-10))
			break
		}
		// Console names come from the API, so they are escaped before reaching a terminal.
		lines = append(lines, "  "+cliutil.EscapeTerminalControls(h.ID)+"  "+cliutil.EscapeTerminalControls(h.Name))
	}
	return "", fmt.Errorf("%w: %d consoles are visible, choose one with --console, UNIFI_CONSOLE_ID or config console_id:\n%s", ErrNoConsoleSelected, len(hosts), strings.Join(lines, "\n"))
}

// HostSummary is the part of a Site Manager host that identifies a console.
type HostSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ParseHostSummaries reads the /v1/hosts envelope.
func ParseHostSummaries(data json.RawMessage) []HostSummary {
	var envelope struct {
		Data []struct {
			ID            string `json:"id"`
			ReportedState struct {
				Name     string `json:"name"`
				Hostname string `json:"hostname"`
			} `json:"reportedState"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return nil
	}
	out := make([]HostSummary, 0, len(envelope.Data))
	for _, h := range envelope.Data {
		if h.ID == "" {
			continue
		}
		name := h.ReportedState.Name
		if name == "" {
			name = h.ReportedState.Hostname
		}
		out = append(out, HostSummary{ID: h.ID, Name: name})
	}
	return out
}

// authHeaderFor picks the credential that belongs to the destination.
func (c *Client) authHeaderFor(ctx context.Context, target requestTarget) (string, error) {
	if target.Kind != TransportLocal {
		return c.authHeader(ctx)
	}
	c.transport.mu.Lock()
	key := c.transport.localKey
	c.transport.mu.Unlock()
	if key == "" && !c.DryRun {
		return "", fmt.Errorf("%w: cloud keys are not sent to a gateway. Create a key in the console under Settings > Control Plane > Integrations", ErrLocalKeyMissing)
	}
	return key, nil
}

// httpClientFor keeps certificate relaxation away from api.ui.com: a single
// shared client would have to skip verification for both or for neither.
func (c *Client) httpClientFor(target requestTarget) (*http.Client, error) {
	if target.Kind != TransportLocal {
		return c.HTTPClient, nil
	}
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	if c.transport.localClient != nil || c.transport.localClientErr != nil {
		return c.transport.localClient, c.transport.localClientErr
	}
	u, err := url.Parse(target.URL)
	if err != nil {
		return nil, err
	}
	trust := tlsSettingsFromConfig(c.Config)
	trust.Insecure = gatewayTrustSkipsVerify(u.Hostname(), trust.CACert)
	var timeout time.Duration
	if c.HTTPClient != nil {
		timeout = c.HTTPClient.Timeout
	}
	hc, err := newHTTPClient(timeout, nil, trust)
	if err != nil {
		c.transport.localClientErr = err
		return nil, err
	}
	guardHTTPClient(hc, u.Scheme+"://"+u.Host)
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if err := redirectGuard(req, via); err != nil {
			return err
		}
		// net/http copies custom headers across hosts; only Authorization and
		// Cookie are stripped for us, so the local key has to be removed here.
		if redirectLeavesOrigin(req.URL, via) {
			req.Header.Del("X-API-KEY")
			// A 307 or 308 replays the body, which can hold a WiFi passphrase. This
			// client may have skipped certificate checks, so the answer sending it
			// elsewhere is not trustworthy.
			if requestReplaysBody(req) {
				return errors.New("refusing to replay a request body to a redirect that leaves the gateway")
			}
		}
		return redirectPlaintextRefused(req, via, hc.Jar)
	}
	c.transport.localClient = hc
	return hc, nil
}

// gatewaySkipsTLSVerify: UniFi OS gateways serve a self-signed certificate on
// the LAN, so literal private addresses skip verification by default. A DNS name
// or public address could be anyone's, so it verifies unless the operator opts
// in. UNIFI_INSECURE_SKIP_VERIFY wins in both directions.
func gatewaySkipsTLSVerify(host string) bool {
	switch strings.ToLower(strings.TrimSpace(cliutil.EnvOverride("UNIFI_INSECURE_SKIP_VERIFY"))) {
	case "1", "true", "yes", "on":
		if !hostIsLocalNetwork(host) {
			warnTLSOnce("warning: UNIFI_INSECURE_SKIP_VERIFY disables certificate verification for gateway " + host + "\n")
		}
		return true
	case "0", "false", "no", "off":
		return false
	}
	if !hostIsLocalNetwork(host) {
		return false
	}
	// A self-signed gateway is why this default exists, but it also means anyone
	// who can answer at this address on the LAN receives the local key. Loopback
	// never leaves the machine, so only network addresses earn the warning.
	if !hostIsLoopback(host) {
		warnTLSOnce("warning: certificate verification is off for private gateway " + host + "; set UNIFI_CA_CERT to verify it\n")
	}
	return true
}

// gatewayTrustSkipsVerify lets a configured CA turn verification on for a private
// gateway. Without it the address-based default overwrote the CA, so the documented
// fix did nothing. An explicit UNIFI_INSECURE_SKIP_VERIFY still decides.
func gatewayTrustSkipsVerify(host, caCert string) bool {
	if strings.TrimSpace(caCert) != "" && strings.TrimSpace(cliutil.EnvOverride("UNIFI_INSECURE_SKIP_VERIFY")) == "" {
		return false
	}
	return gatewaySkipsTLSVerify(host)
}

func hostIsLoopback(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil && addr.Unmap().IsLoopback()
}

// hostIsLocalNetwork matches loopback, RFC1918, RFC4193 and link-local literals.
func hostIsLocalNetwork(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}

func (c *Client) cacheTargetSuffix() string {
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	c.loadSelectionLocked()
	// Site IDs repeat across consoles, so without this a cached list from one
	// console would answer for another.
	return "|console=" + c.transport.console + "|gateway=" + c.transport.gateway + "|mode=" + c.transport.mode + "|auto=" + c.transport.autoSelected
}

// TransportSummary is what doctor and reach show about the resolved route.
type TransportSummary struct {
	Mode            string `json:"mode"`
	ConsoleID       string `json:"console_id,omitempty"`
	Gateway         string `json:"gateway,omitempty"`
	LocalKeySet     bool   `json:"local_key_set"`
	ConnectorPrefix string `json:"connector_proxy_segment"`
	Note            string `json:"note,omitempty"`
}

// DescribeTransport never touches the network; doctor must work offline.
func (c *Client) DescribeTransport() TransportSummary {
	c.loadSelection()
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	t := &c.transport
	s := TransportSummary{ConsoleID: t.console, Gateway: t.gateway, LocalKeySet: t.localKey != "", ConnectorPrefix: connectorProxySegment}
	// This summary is printed by doctor and reach; a pasted URL may carry a password.
	if strings.Contains(t.gateway, "@") {
		s.Gateway = "[invalid: gateway contains credentials]"
	}
	switch {
	case t.gateway != "" && t.mode != "cloud":
		s.Mode = string(TransportLocal)
	case t.console != "":
		s.Mode = string(TransportConnector)
	default:
		s.Mode = "auto-select"
		s.Note = "console-hosted commands use the only host from /v1/hosts; set --console or --gateway when several are visible"
	}
	return s
}

// slidingWindowThrottle enforces the documented per-console budget. A sliding
// log rather than a token bucket, because a bucket's burst can exceed 100 inside
// a rolling minute and the connector counts per minute. It is per process: a
// parallel shell loop can still exceed the limit and will then get a 429.
type slidingWindowThrottle struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	stamps map[string][]time.Time
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
}

func newConnectorThrottle() *slidingWindowThrottle {
	return &slidingWindowThrottle{limit: connectorRequestsPerWindow, window: connectorWindow, stamps: map[string][]time.Time{}, now: time.Now, sleep: sleepContext}
}

var connectorThrottle = newConnectorThrottle()

func (t *slidingWindowThrottle) wait(ctx context.Context, key string) error {
	for {
		t.mu.Lock()
		now := t.now()
		kept := t.stamps[key][:0]
		for _, stamp := range t.stamps[key] {
			if now.Sub(stamp) < t.window {
				kept = append(kept, stamp)
			}
		}
		if len(kept) < t.limit {
			t.stamps[key] = append(kept, now)
			t.mu.Unlock()
			return nil
		}
		t.stamps[key] = kept
		wait := kept[0].Add(t.window).Sub(now)
		t.mu.Unlock()
		if err := t.sleep(ctx, wait); err != nil {
			return fmt.Errorf("waiting for connector budget (%d requests per minute per console): %w", t.limit, err)
		}
	}
}

// annotateConnectorError turns connector limits into something a caller can act
// on. The 408 text matters most: the console may have applied the write even
// though the connector gave up, so the advice is to read back, never to retry.
func annotateConnectorError(kind TransportKind, status int, body string) string {
	if kind != TransportConnector {
		return body
	}
	lower := strings.ToLower(body)
	switch {
	case status == http.StatusRequestTimeout:
		return body + "\nhint: the connector gave up at its 25 second limit. A write may still have been applied on the console: read the resource back before retrying. For reads, narrow the request with --limit or a filter."
	case status == http.StatusRequestEntityTooLarge || strings.Contains(lower, "too large") || strings.Contains(lower, "10 mb") || strings.Contains(lower, "10mb"):
		return body + "\nhint: the connector caps responses at 10 MB. Page with --limit and --offset, or use --gateway to read locally."
	}
	return body
}

// readOnlyModeActive backs UNIFI_READ_ONLY. Checked at the client choke point so
// it covers generated commands, raw, change apply and MCP execute together.
// ReadOnlyModeActive lets change apply refuse before it journals an intent.
func ReadOnlyModeActive() bool { return readOnlyModeActive() }

// Fails closed: someone who set this to "enabled" or "y" asked for protection, and a
// safety switch must not read an unfamiliar value as permission to write. EnvOverride
// keeps an unresolved MCPB placeholder equivalent to unset.
func readOnlyModeActive() bool {
	switch strings.ToLower(strings.TrimSpace(cliutil.EnvOverride("UNIFI_READ_ONLY"))) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}

// Do is the exported request entry for hand-written commands that take a method
// from the user (raw) and so cannot pick one of the verb helpers. It still runs
// through every gate in doInternal.
func (c *Client) Do(ctx context.Context, method, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	if strings.EqualFold(method, http.MethodGet) {
		return c.doInternal(ctx, http.MethodGet, path, params, nil, nil, true, false)
	}
	return c.doInternal(ctx, strings.ToUpper(method), path, params, body, nil, false, true)
}

// ProbeConnectorShape sends one GET to an explicit connector path, bypassing the
// mapping, so reach can test both documented shapes. It stays GET-only so a probe
// can never mutate.
func (c *Client) ProbeConnectorShape(ctx context.Context, consoleID, path, proxySegment string) (json.RawMessage, int, error) {
	return c.doInternal(ctx, http.MethodGet, ConnectorPathFor(consoleID, path, proxySegment), nil, nil, nil, true, false)
}

// ProbeWrite sends a mutating verb to a path the caller guarantees cannot exist
// (reach uses an all-zero UUID). It reports only status: the body is not trusted
// and nothing is retried.
func (c *Client) ProbeWrite(ctx context.Context, method, path string, body any) (int, error) {
	_, status, err := c.doInternal(ctx, method, path, nil, body, map[string]string{ReplaySafeHeader: "false"}, false, true)
	return status, err
}
