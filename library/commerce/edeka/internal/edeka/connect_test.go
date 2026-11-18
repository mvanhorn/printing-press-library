package edeka

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	t.Setenv("EDEKA_BROWSER", "/opt/custom/chrome")
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
			cookies := `[{"name":"sid","value":"x","domain":"m1.edeka.shop","path":"/"}]`
			if loggedIn.Load() {
				cookies = `[{"name":"sid","value":"x","domain":"m1.edeka.shop","path":"/"},{"name":"oxid_1","value":"y","domain":"m1.edeka.shop","path":"/","expires":4102444800}]`
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
	if len(s.Markets()) != 1 || !s.Has("sid", "") {
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
