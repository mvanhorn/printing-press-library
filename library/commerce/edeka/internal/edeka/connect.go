package edeka

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// LoginURL is the account page of a market shop, where a person logs in.
func LoginURL(host string) string { return "https://" + host + "/mein-konto/" }

var (
	// ErrLoginTimeout means the person did not finish the login in time.
	ErrLoginTimeout = errors.New("the EDEKA login was not finished in time; run `auth connect` again")
	// ErrLoginWindowClosed means the login window was closed before the login finished.
	ErrLoginWindowClosed = errors.New("the login window was closed before the EDEKA login finished; run `auth connect` again")
)

// LoginBrowser is a visible Chrome, Edge or Chromium window with a fresh,
// temporary profile and a loopback-only DevTools port. A person logs in to
// their EDEKA market shop there themselves; the CLI
// only reads the resulting cookies. Close removes the profile again.
type LoginBrowser struct {
	Endpoint string
	Closed   <-chan struct{}
	cmd      *exec.Cmd
	dir      string
}

// FindBrowser returns a Chrome, Edge or Chromium executable. EDEKA_BROWSER
// overrides the search.
func FindBrowser() (string, error) {
	if p := strings.TrimSpace(os.Getenv("EDEKA_BROWSER")); p != "" {
		return p, nil
	}
	for _, c := range browserCandidates(runtime.GOOS, os.Getenv) {
		if strings.ContainsAny(c, `/\`) {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c, nil
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no Chrome, Edge or Chromium found: install one or set EDEKA_BROWSER to its executable")
}

func browserCandidates(goos string, env func(string) string) []string {
	switch goos {
	case "windows":
		var out []string
		for _, base := range []string{env("ProgramFiles"), env("ProgramFiles(x86)"), env("LocalAppData")} {
			if base == "" {
				continue
			}
			out = append(out,
				base+`\Google\Chrome\Application\chrome.exe`,
				base+`\Microsoft\Edge\Application\msedge.exe`)
		}
		return append(out, "chrome.exe", "msedge.exe")
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		return []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"}
	}
}

// StartLoginBrowser opens exe on startURL with a new temporary profile and a
// DevTools port on 127.0.0.1, and waits until DevTools answers.
func StartLoginBrowser(ctx context.Context, exe, startURL string) (*LoginBrowser, error) {
	dir, err := os.MkdirTemp("", "edeka-login-*")
	if err != nil {
		return nil, err
	}
	port, err := freeLoopbackPort()
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	cmd := exec.Command(exe,
		"--remote-debugging-port="+strconv.Itoa(port),
		"--user-data-dir="+dir,
		"--no-first-run", "--no-default-browser-check", "--new-window", startURL)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	closed := make(chan struct{})
	go func() { _ = cmd.Wait(); close(closed) }()
	b := &LoginBrowser{Endpoint: "http://127.0.0.1:" + strconv.Itoa(port), Closed: closed, cmd: cmd, dir: dir}
	// The startup deadline bounds each probe too, so a DevTools port that
	// accepts connections but never answers still ends in cleanup.
	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		req, _ := http.NewRequestWithContext(startCtx, http.MethodGet, b.Endpoint+"/json/version", nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
			return b, nil
		}
		select {
		case <-closed:
			b.Close()
			return nil, errors.New("the browser exited right after start; close other windows of it that use the same profile, or set EDEKA_BROWSER")
		case <-startCtx.Done():
			b.Close()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, errors.New("the browser's DevTools port did not answer within 20 seconds")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Close ends the browser and deletes its temporary profile.
func (b *LoginBrowser) Close() {
	if b == nil {
		return
	}
	if b.cmd != nil && b.cmd.Process != nil && !b.closeGracefully() {
		_ = b.cmd.Process.Kill()
		select {
		case <-b.Closed:
		case <-time.After(5 * time.Second):
		}
	}
	// The browser may hold profile files open for a moment after it ends.
	for i := 0; i < 30; i++ {
		if err := os.RemoveAll(b.dir); err == nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// closeGracefully asks the browser to quit over DevTools (Browser.close), so it
// shuts down all its processes and releases the profile; false when it did not
// end within a few seconds.
func (b *LoginBrowser) closeGracefully() bool {
	select {
	case <-b.Closed:
		return true
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, b.Endpoint+"/json/version", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	var ver struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(resp.Body).Decode(&ver)
	resp.Body.Close()
	if err != nil || ver.WS == "" {
		return false
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, ver.WS, nil)
	if err != nil {
		return false
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"id": 1, "method": "Browser.close"}); err != nil {
		return false
	}
	select {
	case <-b.Closed:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// WaitForLogin polls the browser's cookies until the shop's one-year
// remember-me cookie (set by a login with "stay logged in") appears on a
// market host, then reads them once more so the fresh shop session is included. It ends with ErrLoginTimeout when ctx ends and
// with ErrLoginWindowClosed when closed is closed first.
func WaitForLogin(ctx context.Context, endpoint string, closed <-chan struct{}, poll time.Duration) ([]Cookie, error) {
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-closed:
			return nil, ErrLoginWindowClosed
		default:
		}
		if cs, err := CookiesFromCDP(ctx, endpoint); err == nil {
			s := &Session{Cookies: FilterEDEKA(cs)}
			if len(s.Markets()) > 0 {
				select {
				case <-time.After(3 * poll):
				case <-ctx.Done():
					return cs, nil
				}
				if more, err := CookiesFromCDP(ctx, endpoint); err == nil && len(more) >= len(cs) {
					return more, nil
				}
				return cs, nil
			}
		}
		select {
		case <-closed:
			return nil, ErrLoginWindowClosed
		case <-ctx.Done():
			return nil, ErrLoginTimeout
		case <-t.C:
		}
	}
}

func freeLoopbackPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
