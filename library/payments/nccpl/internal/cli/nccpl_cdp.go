package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// A minimal Chrome DevTools Protocol client, used only by `capture`.
//
// NCCPL's Cloudflare gate cannot be passed by replaying a clearance cookie -- proven
// across matched TLS fingerprints, both HTTP versions and every cookie combination
// (see proofs/cloudflare-investigation.md). A real headed Chrome, however, solves the
// challenge itself and can then read the API normally. `capture` drives exactly that:
// a throwaway profile, launched on demand, torn down afterwards. It is never the
// transport for ordinary commands, which read only the local store.
//
// Headless works ONLY with the User-Agent pinned. Chrome's headless mode advertises
// `HeadlessChrome/<v>` and the origin's WAF hard-blocks that token outright ("Sorry, you
// have been blocked"). Everything else a headless Chrome sends -- TLS/HTTP fingerprint and
// Sec-CH-UA brands -- matches the headed build, so restoring the normal token restores the
// challenge, which then self-solves without a window (measured 2026-09-05: fresh profile,
// 403 challenge -> Turnstile -> 200 in under 15s; POST var-margins/data 200, 1091 rows).

const nccplDefaultChromeMacOS = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// nccplChromeBinary resolves the browser to drive, overridable for non-default installs.
func nccplChromeBinary() (string, error) {
	if v := strings.TrimSpace(os.Getenv("NCCPL_CHROME_BINARY")); v != "" {
		if _, err := os.Stat(v); err != nil {
			return "", fmt.Errorf("NCCPL_CHROME_BINARY=%s: %w", v, err)
		}
		return v, nil
	}
	for _, c := range []string{
		nccplDefaultChromeMacOS,
		"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("Google Chrome not found; set NCCPL_CHROME_BINARY to its path")
}

// nccplFreePort asks the OS for an unused local port.
func nccplFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

type nccplBrowser struct {
	cmd     *exec.Cmd
	port    int
	profile string
	conn    *websocket.Conn
	nextID  int64
}

// nccplHeadlessUserAgent builds the User-Agent a headless Chrome must present: the same
// reduced UA the headed build sends, with only the major version varying. Chrome prints
// "Google Chrome 149.0.7827.197" for --version; the reduced UA carries "Chrome/149.0.0.0".
func nccplHeadlessUserAgent(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("reading Chrome version: %w", err)
	}
	m := regexp.MustCompile(`(\d+)\.\d+\.\d+\.\d+`).FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("unrecognised Chrome version output %q", strings.TrimSpace(string(out)))
	}
	platform := "Macintosh; Intel Mac OS X 10_15_7"
	switch runtime.GOOS {
	case "linux":
		platform = "X11; Linux x86_64"
	case "windows":
		platform = "Windows NT 10.0; Win64; x64"
	}
	return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", platform, m[1]), nil
}

// nccplLaunchChrome starts a throwaway Chrome and attaches to its page target. With
// headless set, no window is created and the User-Agent is pinned to the normal token.
func nccplLaunchChrome(ctx context.Context, keepProfile string, headless bool) (*nccplBrowser, error) {
	bin, err := nccplChromeBinary()
	if err != nil {
		return nil, err
	}
	var ua string
	if headless {
		if ua, err = nccplHeadlessUserAgent(bin); err != nil {
			return nil, err
		}
	}
	port, err := nccplFreePort()
	if err != nil {
		return nil, fmt.Errorf("allocating debug port: %w", err)
	}
	profile := keepProfile
	if profile == "" {
		profile, err = os.MkdirTemp("", "nccpl-capture-")
		if err != nil {
			return nil, fmt.Errorf("creating throwaway profile: %w", err)
		}
	}
	args := []string{
		"--user-data-dir=" + profile,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run", "--no-default-browser-check",
		"--disable-background-networking", "--disable-sync",
		"--window-size=1200,900",
	}
	if headless {
		args = append(args, "--headless=new", "--user-agent="+ua)
	}
	args = append(args, "about:blank")
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launching Chrome: %w", err)
	}
	b := &nccplBrowser{cmd: cmd, port: port, profile: profile}

	wsURL, err := b.waitForPageTarget(ctx, 30*time.Second)
	if err != nil {
		_ = b.Close(keepProfile != "")
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		_ = b.Close(keepProfile != "")
		return nil, fmt.Errorf("attaching to Chrome: %w", err)
	}
	b.conn = conn
	return b, nil
}

func (b *nccplBrowser) waitForPageTarget(ctx context.Context, limit time.Duration) (string, error) {
	deadline := time.Now().Add(limit)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("http://127.0.0.1:%d/json", b.port), nil)
		resp, err := client.Do(req)
		if err == nil {
			var targets []struct {
				Type string `json:"type"`
				WS   string `json:"webSocketDebuggerUrl"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&targets)
			_ = resp.Body.Close()
			for _, t := range targets {
				if t.Type == "page" && t.WS != "" {
					return t.WS, nil
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return "", fmt.Errorf("Chrome did not expose a debuggable page within %s", limit)
}

// Eval runs an expression in the page and unmarshals its value, awaiting promises.
func (b *nccplBrowser) Eval(ctx context.Context, expr string, out any) error {
	id := atomic.AddInt64(&b.nextID, 1)
	msg := map[string]any{
		"id":     id,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expr,
			"awaitPromise":  true,
			"returnByValue": true,
		},
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = b.conn.SetWriteDeadline(deadline)
		_ = b.conn.SetReadDeadline(deadline)
	} else {
		_ = b.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	}
	if err := b.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("cdp write: %w", err)
	}
	for {
		var frame struct {
			ID     int64 `json:"id"`
			Result struct {
				Result struct {
					Value json.RawMessage `json:"value"`
				} `json:"result"`
				ExceptionDetails *struct {
					Text string `json:"text"`
				} `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := b.conn.ReadJSON(&frame); err != nil {
			return fmt.Errorf("cdp read: %w", err)
		}
		if frame.ID != id {
			continue
		}
		if frame.Result.ExceptionDetails != nil {
			return fmt.Errorf("page error: %s", frame.Result.ExceptionDetails.Text)
		}
		if out == nil {
			return nil
		}
		if len(frame.Result.Result.Value) == 0 {
			return fmt.Errorf("page returned no value")
		}
		return json.Unmarshal(frame.Result.Result.Value, out)
	}
}

// Navigate points the page at a URL and waits for the document to settle.
func (b *nccplBrowser) Navigate(ctx context.Context, url string) error {
	expr := fmt.Sprintf("(()=>{location.href=%q;return 'ok';})()", url)
	var s string
	if err := b.Eval(ctx, expr, &s); err != nil {
		return err
	}
	return nil
}

// Close stops Chrome and removes the throwaway profile unless it was operator-supplied.
func (b *nccplBrowser) Close(keepProfile bool) error {
	if b.conn != nil {
		_ = b.conn.Close()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_, _ = b.cmd.Process.Wait()
	}
	if !keepProfile && b.profile != "" && strings.HasPrefix(filepath.Base(b.profile), "nccpl-capture-") {
		return os.RemoveAll(b.profile)
	}
	return nil
}
