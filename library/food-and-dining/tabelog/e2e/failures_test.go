package e2e

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestInvalidInputsFailBeforeHTTP(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 500} })
	w := newWorkspace(t, r)
	cases := [][]string{
		{"find"},
		{"find", "--area", tokyoURL, "--limit", "0"},
		{"find", "--area", tokyoURL, "--limit", "51"},
		{"find", "--area", tokyoURL, "--max-pages", "0"},
		{"find", "--area", tokyoURL, "--max-pages", "6"},
		{"find", "--area", tokyoURL, "--meal", "breakfast"},
		{"find", "--area", tokyoURL, "--budget-max", "5000"},
		{"find", "--area", tokyoURL, "--meal", "dinner", "--budget-max", "7500"},
		{"find", "--area", tokyoURL, "--meal", "dinner", "--budget-min", "5000", "--budget-max", "1000"},
		{"show", "https://example.invalid/en/tokyo/A1301/A130103/13294162/"},
		{"show", "http://tabelog.com/en/tokyo/A1301/A130103/13294162/"},
		{"show", "https://tabelog.com/tokyo/A1301/A130103/13294162/"},
		{"show", "99999999"},
		{"lists", "add", "trip", "99999999"},
		{"lists", "alternatives", "trip", "--for", "13005012"},
		{"lists", "alternatives", "trip", "--for", "13005012", "--match", "radius"},
		{"lists", "audit", "trip", "--require", "open_now"},
		{"lists", "refresh", "trip", "--data-source", "local"},
		{"lists", "show", "--data-source", "live"},
	}
	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) { mustFail(t, w.run(t, append(args, "--agent")...)); equal(t, r.count(), 0) })
	}
}

func TestBlocksDriftAndHTTPFailuresCannotBecomeEmptySuccess(t *testing.T) {
	// Synthetic interstitial/structural mutations are deliberately labeled,
	// rather than claimed as live Tabelog responses.
	cases := []struct {
		name   string
		status int
		body   []byte
	}{
		{"interstitial200", 200, []byte("<!doctype html><title>Access denied</title><p>Please verify you are human.</p>")},
		{"unrelated200", 200, []byte("<!doctype html><title>Unrelated page</title><p>No results shown here.</p>")},
		{"missingCards200", 200, []byte("<!doctype html><title>Best Restaurants in Tokyo | Tabelog</title><div class=\"c-page-count\"><strong>1</strong><strong>20</strong><strong>139392</strong></div><div class=\"js-rstlist-info rstlist-info\"></div>")},
		{"forbidden", 403, []byte("Forbidden")},
		{"rateLimited", 429, []byte("Too many requests")},
		{"upstreamError", 503, []byte("Service unavailable")},
		{"oversized", 200, bytes.Repeat([]byte("x"), 4*1024*1024+1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newReplay(t, func(*http.Request) response { return response{status: c.status, body: c.body} })
			w := newWorkspace(t, r)
			res := w.run(t, "find", "--area", tokyoURL, "--agent")
			mustFail(t, res)
			if c.status == 503 {
				if r.count() < 1 || r.count() > 2 {
					t.Fatal("transient retries were not bounded")
				}
			} else {
				equal(t, r.count(), 1)
			}
			if len(res.stdout)+len(res.stderr) > 4096 {
				t.Fatal("failure dumped source body or excessive diagnostics")
			}
		})
	}
}

func TestMismatchedGeographyCannotOverwriteTheCachedSnapshot(t *testing.T) {
	good := readFixture(t, "tokyo-ranked.html")
	r := newReplay(t, func(*http.Request) response { return response{body: good} })
	w := newWorkspace(t, r)
	before := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--agent"))
	wrong := readFixture(t, "ginza-bars.html")
	r.serve(func(*http.Request) response { return response{body: wrong} })
	mustFail(t, w.run(t, "find", "--area", tokyoURL, "--data-source", "live", "--agent"))
	after := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--data-source", "local", "--agent"))
	equal(t, items(t, after), items(t, before))
	equal(t, r.count(), 2)
}

func TestRedirectsCannotEscapeTheApprovedSourceTransport(t *testing.T) {
	other := newReplay(t, func(*http.Request) response { return response{body: []byte("unapproved redirected destination")} })
	r := newReplay(t, func(*http.Request) response { return response{status: 302, body: []byte("redirect")} })
	// A separate loopback listener is an observable off-source destination;
	// the test never risks fetching an unrelated Internet host.
	r.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests = append(r.requests, request{path: req.URL.Path})
		r.mu.Unlock()
		w.Header().Set("Location", other.server.URL+"/en/tokyo/rstLst/")
		w.WriteHeader(302)
	})
	w := newWorkspace(t, r)
	mustFail(t, w.run(t, "find", "--area", tokyoURL, "--agent"))
	equal(t, other.count(), 0)
	equal(t, r.count(), 1)
}

func TestSourceOperationHasABoundedDeadline(t *testing.T) {
	r := newReplay(t, func(req *http.Request) response { <-req.Context().Done(); return response{status: 504} })
	w := newWorkspace(t, r)
	start := time.Now()
	mustFail(t, w.run(t, "find", "--area", tokyoURL, "--agent"))
	if time.Since(start) > 22*time.Second {
		t.Fatal("source operation exceeded the documented20s deadline plus startup allowance")
	}
	equal(t, r.count(), 1)
}
