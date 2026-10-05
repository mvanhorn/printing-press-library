// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Preserved hooks that bind the owner's traffic rules to the generated client
// and give doctor a positive content check. They live here, not in generated
// files, so a regen cannot silently drop them.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerClientHook(installUberGuard)
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		wrapDoctorContentCheck(root, flags)
		addNovelCommandIfAbsent(root, newUberSyncCmd(flags))
		guardBareInvocation(root, "careers", "search")
		guardBareInvocation(root, "careers", "lookup")
		describeDryRun(root, flags, "careers", "search")
		describeDryRun(root, flags, "careers", "lookup")
	})
}

// describeDryRun names the planned request in a generated endpoint command's
// --dry-run JSON. The generated read path prints the client's bare
// {"dry_run": true}, which says nothing about what would have been sent (and
// the press's live dogfood fails a dry run without an action).
func describeDryRun(root *cobra.Command, flags *rootFlags, path ...string) {
	c, _, err := root.Find(path)
	if err != nil || c == nil || c.Name() != path[len(path)-1] || c.RunE == nil {
		return
	}
	method, apiPath := c.Annotations["pp:method"], c.Annotations["pp:path"]
	if method == "" || apiPath == "" {
		return
	}
	original := c.RunE
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if !flags.dryRun {
			return original(cmd, args)
		}
		out := cmd.OutOrStdout()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		runErr := original(cmd, args)
		cmd.SetOut(out)
		if _, err := out.Write(withDryRunAction(buf.Bytes(), method+" "+apiPath)); err != nil && runErr == nil {
			return err
		}
		return runErr
	}
}

// withDryRunAction adds an action to a dry-run JSON object that lacks one,
// keeping its compact or indented style; anything else passes through.
func withDryRunAction(raw []byte, action string) []byte {
	var obj map[string]any
	if json.Unmarshal(bytes.TrimSpace(raw), &obj) != nil {
		return raw
	}
	if dry, _ := obj["dry_run"].(bool); !dry {
		return raw
	}
	if a, _ := obj["action"].(string); strings.TrimSpace(a) != "" {
		return raw
	}
	obj["action"] = action + " (dry run; no request sent)"
	var b []byte
	var err error
	if bytes.Contains(raw, []byte("\n ")) {
		b, err = json.MarshalIndent(obj, "", "  ")
	} else {
		b, err = json.Marshal(obj)
	}
	if err != nil {
		return raw
	}
	return append(b, '\n')
}

// guardBareInvocation makes a generated endpoint command print its help when
// run with no arguments and no flags, instead of sending a request with its
// default parameters. Verify's required-flag probe runs every command bare,
// outside its mock server, so a bare request would reach the real site.
func guardBareInvocation(root *cobra.Command, path ...string) {
	c, _, err := root.Find(path)
	if err != nil || c == nil || c.Name() != path[len(path)-1] || c.RunE == nil {
		return
	}
	original := c.RunE
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		return original(cmd, args)
	}
}

// guardedHost reports whether a request goes to a real Uber or Oracle host;
// local mock servers used by verify are not paced.
func guardedHost(host string) bool {
	h := strings.ToLower(host)
	if i := strings.LastIndexByte(h, ':'); i > 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	return h == "uber.com" || strings.HasSuffix(h, ".uber.com") || strings.HasSuffix(h, ".oraclecloud.com")
}

// uberGuard wraps the generated client's transport: one machine-wide paced
// stream, and after a refusal from a host nothing more is sent to it, so the
// generated retry loop can never put a second request on the wire.
type uberGuard struct {
	base     http.RoundTripper
	gate     *uberjobs.Gate
	stateDir string
	logPath  string

	mu      sync.Mutex
	refused map[string]*uberjobs.RefusalError
}

// lastGuardRefusal records the most recent refusal any guard saw in this
// process, so callers can classify an error the generated retry loop wrapped.
var (
	lastGuardRefusalMu sync.Mutex
	lastGuardRefusal   *uberjobs.RefusalError
)

func guardRefusal() *uberjobs.RefusalError {
	lastGuardRefusalMu.Lock()
	defer lastGuardRefusalMu.Unlock()
	return lastGuardRefusal
}

func setGuardRefusal(r *uberjobs.RefusalError) {
	lastGuardRefusalMu.Lock()
	lastGuardRefusal = r
	lastGuardRefusalMu.Unlock()
}

// resetGuardRefusal forgets refusals from earlier work in this process, so a
// long-lived process (the MCP server) judges each command on its own requests.
func resetGuardRefusal() { setGuardRefusal(nil) }

func installUberGuard(c *client.Client) error {
	if c == nil || c.HTTPClient == nil {
		return nil
	}
	if _, already := c.HTTPClient.Transport.(*uberGuard); already {
		return nil
	}
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	dir := uberStateDir()
	c.HTTPClient.Transport = &uberGuard{
		base:     base,
		gate:     uberjobs.NewGate(dir),
		stateDir: dir,
		logPath:  strings.TrimSpace(os.Getenv("UBER_JOBS_REQUEST_LOG")),
	}
	return nil
}

// RoundTrip paces and latches real Uber and Oracle hosts. A refusal from any
// host (403, 429, or a challenge page) reaches the generated client as a
// terminal 429 (see refusalResponse), and every later request to that host in
// this process is answered the same way without touching the wire.
func (g *uberGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if r := g.refusalFor(host); r != nil {
		return refusalResponse(req, r), nil
	}
	if !guardedHost(host) {
		return g.exchange(req, false)
	}
	var resp *http.Response
	var latched *uberjobs.RefusalError
	err := g.gate.Do(req.Context(), func() error {
		// Checked again under the gate lock: another process may have
		// latched the host while this one queued.
		if latched = g.refusalFor(host); latched != nil {
			return latched
		}
		return nil
	}, func() error {
		var err error
		resp, err = g.exchange(req, true)
		return err
	})
	if latched != nil {
		return refusalResponse(req, latched), nil
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// refusalFor returns this guard's refusal for host, or, for a guarded host,
// the latch any process recorded.
func (g *uberGuard) refusalFor(host string) *uberjobs.RefusalError {
	g.mu.Lock()
	prior := g.refused[host]
	g.mu.Unlock()
	if prior != nil {
		return prior
	}
	if !guardedHost(host) {
		return nil
	}
	if r := uberjobs.LatchedRefusal(g.stateDir, host, time.Now()); r != nil {
		g.remember(host, r)
		return r
	}
	return nil
}

// exchange sends req once, buffers the reply, and turns a refusal into the
// terminal response. For guarded hosts it runs under the gate lock, so the
// refusal latch is written before the next process may send.
func (g *uberGuard) exchange(req *http.Request, guarded bool) (*http.Response, error) {
	host := req.URL.Host
	started := time.Now().UTC()
	resp, err := g.base.RoundTrip(req)
	if err != nil {
		if guarded {
			g.log(started, req, 0, 0, err)
		}
		return nil, err
	}
	body, rerr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	_ = resp.Body.Close()
	if guarded {
		g.log(started, req, resp.StatusCode, len(body), rerr)
	}
	// A refusal is classified before the read error: a 403 whose body was
	// cut short is still a refusal, and passing it up as a network error
	// would let the generated client retry the host that refused.
	challenge := uberjobs.IsChallenge(resp.Header, body)
	if challenge || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		snippet := string(body)
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		r := uberjobs.NewRefusal(req.URL.String(), host, resp.StatusCode, challenge, snippet)
		g.remember(host, r)
		if guarded {
			uberjobs.RecordRefusal(g.stateDir, r)
		}
		return refusalResponse(req, r), nil
	}
	if rerr != nil {
		// A truncated body is a transport failure, never a short reply.
		return nil, rerr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// remember latches a refusal for host in this guard and for doctor.
func (g *uberGuard) remember(host string, r *uberjobs.RefusalError) {
	g.mu.Lock()
	if g.refused == nil {
		g.refused = map[string]*uberjobs.RefusalError{}
	}
	g.refused[host] = r
	g.mu.Unlock()
	setGuardRefusal(r)
}

// refusalResponse is what the generated client sees for a refused request: a
// 429 whose Retry-After outlasts the client's whole retry budget, so its
// retry loop stops at once with a typed rate-limit error (exit 7) instead of
// retrying, and instead of mapping a 403 to the auth exit code. The body
// names the real refusal.
func refusalResponse(req *http.Request, r *uberjobs.RefusalError) *http.Response {
	body, _ := json.Marshal(map[string]any{"error": r.Error(), "refused": true, "status": r.Status, "challenge": r.Challenge})
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Retry-After", "86400")
	return &http.Response{
		Status:        "429 Too Many Requests",
		StatusCode:    http.StatusTooManyRequests,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        h,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

func (g *uberGuard) log(t time.Time, req *http.Request, status, n int, err error) {
	if g.logPath == "" {
		return
	}
	f, ferr := os.OpenFile(g.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if ferr != nil {
		return
	}
	defer f.Close()
	st := fmt.Sprint(status)
	if err != nil {
		st = "ERR:" + strings.Join(strings.Fields(err.Error()), " ")
	}
	fmt.Fprintf(f, "%s\tuber-jobs-pp-cli(generated client)\t%s\t%s\t%s\t%d\n", t.Format("2006-01-02T15:04:05Z"), req.Method, req.URL.String(), st, n)
}

const doctorHealthPath = "/api/jobs/search/?page=1&pagesize=1"

// wrapDoctorContentCheck makes doctor print "API: reachable" only after a
// 200 whose body is a real search envelope. A challenge reports blocked
// (exit 7) and a DNS or transport failure reports unreachable (exit 6).
func wrapDoctorContentCheck(root *cobra.Command, flags *rootFlags) {
	doctor, _, err := root.Find([]string{"doctor"})
	if err != nil || doctor == nil || doctor.Name() != "doctor" || doctor.RunE == nil {
		return
	}
	original := doctor.RunE
	doctor.RunE = func(cmd *cobra.Command, args []string) error {
		if flags.dryRun || cmd.Flags().Changed("help") {
			return original(cmd, args)
		}
		verdict, verr := doctorContentCheck(cmd, flags)
		if verr == nil || errors.Is(verr, errDoctorNotChecked) {
			// Config, profile, and tenant-binding failures belong to the
			// generated doctor, which reports them in its typed format.
			return original(cmd, args)
		}
		if flags.asJSON || flags.agent {
			_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"api": verdict, "ok": false})
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "API: %s\n", verdict)
		}
		return verr
	}
}

// errDoctorNotChecked means the content check could not build a client, so
// no request was sent and the generated doctor should report the cause.
var errDoctorNotChecked = errors.New("content check not run: no client")

func doctorContentCheck(cmd *cobra.Command, flags *rootFlags) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "not checked (config error)", fmt.Errorf("%w: %v", errDoctorNotChecked, err)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	// Judge this health check on its own request: forget refusals from
	// earlier work in the process, and skip the response cache read (the
	// fresh reply is still cached, so the generated doctor's identical probe
	// that follows costs no second request).
	resetGuardRefusal()
	body, err := c.GetWithHeadersNoCache(ctx, doctorHealthPath, nil, map[string]string{client.HTMLResponseHeader: "true"})
	if err != nil {
		var apiE *client.APIError
		var refusal *uberjobs.RefusalError
		var limited *platform.RateLimitedError
		var dnsErr *net.DNSError
		r := guardRefusal()
		if r == nil && errors.As(err, &refusal) {
			r = refusal
		}
		switch {
		case r != nil && r.Latched:
			return fmt.Sprintf("blocked (refused at %s; no request is sent until %s)", r.LatchedAt, r.Until), rateLimitErr(err)
		case r != nil, errors.As(err, &limited), errors.As(err, &apiE) && (apiE.StatusCode == 403 || apiE.StatusCode == 429):
			return "blocked (the careers site refused the health check; not retried)", rateLimitErr(err)
		case errors.As(err, &dnsErr):
			return "unreachable (DNS lookup failed)", transportErr(err)
		case errors.As(err, &apiE):
			return fmt.Sprintf("error (HTTP %d from the health check)", apiE.StatusCode), apiErr(err)
		default:
			return "unreachable (" + strings.Join(strings.Fields(err.Error()), " ") + ")", transportErr(err)
		}
	}
	if uberjobs.IsChallenge(http.Header{}, body) {
		return "blocked (challenge page instead of data)", rateLimitErr(fmt.Errorf("challenge page at %s", doctorHealthPath))
	}
	var env struct {
		Jobs      []json.RawMessage `json:"jobs"`
		TotalJobs *int              `json:"totalJobs"`
	}
	if json.Unmarshal(body, &env) != nil || env.Jobs == nil || env.TotalJobs == nil {
		return "error (the health check returned no search envelope)", apiErr(fmt.Errorf("content check failed at %s", doctorHealthPath))
	}
	return "reachable", nil
}
