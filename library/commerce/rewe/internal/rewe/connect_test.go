package rewe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBrowserCandidates(t *testing.T) {
	env := func(k string) string {
		return map[string]string{"ProgramFiles": `C:\PF`, "LocalAppData": `C:\LA`}[k]
	}
	win := strings.Join(browserCandidates("windows", env), "|")
	for _, want := range []string{"chrome.exe", "msedge.exe", "PF", "LA"} {
		if !strings.Contains(win, want) {
			t.Fatalf("windows candidates miss %q: %s", want, win)
		}
	}
	if mac := strings.Join(browserCandidates("darwin", env), "|"); !strings.Contains(mac, "Google Chrome.app") {
		t.Fatalf("darwin candidates: %s", mac)
	}
	if lin := strings.Join(browserCandidates("linux", env), "|"); !strings.Contains(lin, "chromium") {
		t.Fatalf("linux candidates: %s", lin)
	}
}

func TestFindBrowserOverride(t *testing.T) {
	t.Setenv("REWE_BROWSER", "/opt/custom/chrome")
	got, err := FindBrowser()
	if err != nil || got != "/opt/custom/chrome" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// fakeDevTools serves /json/version and a websocket answering Storage.getCookies;
// the cookie set grows once `loggedIn` is true.
func fakeDevTools(t *testing.T, loggedIn *atomic.Bool) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			fmt.Fprintf(w, `{"webSocketDebuggerUrl":"ws://%s/devtools/browser/x"}`, strings.TrimPrefix(srv.URL, "http://"))
		case "/devtools/browser/x":
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer c.Close()
			var req map[string]any
			if err := c.ReadJSON(&req); err != nil {
				return
			}
			cookies := `[{"name":"__cf_` + `bm","value":"x","domain":".rewe.de","path":"/"}]`
			if loggedIn.Load() {
				cookies = `[{"name":"KEYCLOAK_IDENTITY","value":"x","domain":"account.rewe.de","path":"/realms/sso/","expires":4102444800},{"name":"rstp","value":"y","domain":"www.rewe.de","path":"/"}]`
			}
			_ = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":1,"result":{"cookies":%s}}`, cookies)))
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestWaitForLoginReturnsOnceLoggedIn(t *testing.T) {
	var logged atomic.Bool
	srv := fakeDevTools(t, &logged)
	defer srv.Close()
	go func() { time.Sleep(150 * time.Millisecond); logged.Store(true) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cs, err := WaitForLogin(ctx, srv.URL, nil, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{Cookies: cs}
	if !s.Has("KEYCLOAK_IDENTITY") || !s.Has("rstp") {
		t.Fatalf("cookies %+v", cs)
	}
}

func TestWaitForLoginTimesOut(t *testing.T) {
	var logged atomic.Bool
	srv := fakeDevTools(t, &logged)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := WaitForLogin(ctx, srv.URL, nil, 30*time.Millisecond); !errors.Is(err, ErrLoginTimeout) {
		t.Fatalf("want ErrLoginTimeout, got %v", err)
	}
}

func TestWaitForLoginWindowClosed(t *testing.T) {
	var logged atomic.Bool
	srv := fakeDevTools(t, &logged)
	defer srv.Close()
	closed := make(chan struct{})
	close(closed)
	if _, err := WaitForLogin(context.Background(), srv.URL, closed, 30*time.Millisecond); !errors.Is(err, ErrLoginWindowClosed) {
		t.Fatalf("want ErrLoginWindowClosed, got %v", err)
	}
}

func init() {
	// The test binary doubles as a browser that never opens its DevTools port.
	if os.Getenv("REWE_TEST_FAKE_BROWSER") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func TestStartLoginBrowserCanceledCleansUp(t *testing.T) {
	tmp := t.TempDir()
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(k, tmp)
	}
	t.Setenv("REWE_TEST_FAKE_BROWSER", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	b, err := StartLoginBrowser(ctx, os.Args[0], "about:blank")
	if b != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("StartLoginBrowser = %v, %v; want nil, context.DeadlineExceeded", b, err)
	}
	if d := time.Since(started); d > 15*time.Second {
		t.Fatalf("cancel took %s", d)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "rewe-login-*")); len(left) > 0 {
		t.Fatalf("temporary profile left behind: %v", left)
	}
}
