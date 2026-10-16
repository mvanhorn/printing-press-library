// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
)

// TestNovelPortfolioHelpWires smoke-tests that the portfolio command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPortfolioHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"portfolio", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("portfolio --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "portfolio"} {
		if !strings.Contains(help, want) {
			t.Fatalf("portfolio --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSiteEnvSlug(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"example.com", "EXAMPLE_COM"}, {"foo-bar.test:80", "FOO_BAR_TEST_80"}, {"a.com:8080", "A_COM_8080"}, {"", ""}, {"a_b", "A_B"}} {
		t.Run(tc.in, func(t *testing.T) {
			if got := siteEnvSlug(tc.in); got != tc.want {
				t.Fatalf("%q != %q", got, tc.want)
			}
		})
	}
}

func TestPortfolioRejectsCredentialSlugCollision(t *testing.T) {
	testenv.Isolate(t)
	cmd := newNovelPortfolioCmd(&rootFlags{asJSON: true, timeout: 5 * time.Second, noCache: true})
	cmd.SetArgs([]string{"--sites", "foo-bar.test,foo_bar.test"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), `"foo-bar.test" and "foo_bar.test"`) {
		t.Fatalf("expected usage error naming colliding hosts, got %v", err)
	}
}

func TestPortfolioRedirectUsesOnlySiteCredentials(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("RANKMATH_USERNAME", "default-user")
	t.Setenv("RANKMATH_APP_PASSWORD", "default-pass")
	t.Setenv("RANKMATH_SITE", "https://site-a.test")
	t.Setenv("RANKMATH_USERNAME_SITE_A_TEST", "site-user")
	t.Setenv("RANKMATH_APP_PASSWORD_SITE_A_TEST", "site-pass")
	var auths []string
	oldHooks := clientHooks
	t.Cleanup(func() { clientHooks = oldHooks })
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			auths = append(auths, r.Header.Get("Authorization"))
			w := httptest.NewRecorder()
			if r.URL.Path == "/wp-json/rankmath/v1/an/analyticsSummary" {
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(http.StatusMovedPermanently)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"clicks":{"total":1},"impressions":{"total":1},"position":{"total":1},"ctr":{"total":1}}`))
			}
			return w.Result(), nil
		})}
		return nil
	})
	cmd := newNovelPortfolioCmd(&rootFlags{asJSON: true, timeout: 5 * time.Second, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--sites", "site-a.test"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := "Basic " + "c2l0ZS11c2VyOnNpdGUtcGFzcw=="
	if len(auths) != 2 || auths[0] != want || auths[1] != want {
		t.Fatalf("redirect Authorization headers = %#v, want site credential %q on both requests", auths, want)
	}
}
func TestDecodeSummary(t *testing.T) {
	for _, tc := range []struct {
		name, raw, status string
		clicks            float64
		bad               bool
	}{
		{"ok", `{"clicks":{"total":12,"difference":-2},"impressions":{"total":100,"difference":4},"position":{"total":6,"difference":1},"ctr":{"total":12}}`, "ok", 12, false},
		{"empty", `{}`, "not_connected", 0, false},
		{"na", `{"clicks":{"total":"n/a"},"impressions":"n/a","position":"n/a","ctr":"n/a"}`, "not_connected", 0, false},
		{"bad", `{"clicks":{"total":"broken"}}`, "", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeSummary([]byte(tc.raw))
			if (err != nil) != tc.bad {
				t.Fatalf("%v", err)
			}
			if tc.bad {
				return
			}
			if got.Status != tc.status || got.Clicks != tc.clicks {
				t.Fatalf("%+v", got)
			}
			if tc.status == "ok" && (got.ClicksChange != -2 || got.Impressions != 100 || got.Position != 6 || got.CTR != 12) {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestPortfolioTotalsExcludeFailure(t *testing.T) {
	testenv.Isolate(t)
	var gotAuth string
	good := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/rankmath/v1/an/analyticsSummary" {
			t.Errorf("path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"clicks":{"total":12,"difference":2},"impressions":{"total":100},"position":{"total":6},"ctr":{"total":12}}`))
	})
	badRequests := 0
	bad := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badRequests++
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header on unconfigured site")
		}
		w.WriteHeader(401)
	})
	t.Setenv("RANKMATH_USERNAME", "reader")
	t.Setenv("RANKMATH_APP_PASSWORD", "secret")
	t.Setenv("RANKMATH_SITE", "https://site-a.test")
	oldHooks := clientHooks
	t.Cleanup(func() { clientHooks = oldHooks })
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient = &http.Client{Transport: portfolioTestTransport{handlers: map[string]http.Handler{"site-a.test": good, "site-b.test": bad}}}
		return nil
	})
	flags := &rootFlags{asJSON: true, timeout: 5 * time.Second, noCache: true}
	cmd := newNovelPortfolioCmd(flags)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--sites", "site-a.test,site-b.test"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got portfolioResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if got.Totals.Clicks != 12 || got.Totals.Impressions != 100 || got.Totals.SitesCounted != 1 || len(got.FetchFailures) != 0 || got.Sites[1].Status != "not_configured" || badRequests != 0 {
		t.Fatalf("%+v", got)
	}
	if gotAuth != "Basic cmVhZGVyOnNlY3JldA==" {
		t.Fatalf("default auth header = %q", gotAuth)
	}
	if !strings.Contains(stderr.String(), "warning: 0 of 2 sites failed, 1 not configured; totals computed over 1 sites") {
		t.Fatal(stderr.String())
	}
}

func TestPortfolioRefusesAuthOverHTTP(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("RANKMATH_USERNAME", "reader")
	t.Setenv("RANKMATH_APP_PASSWORD", "secret")
	t.Setenv("RANKMATH_SITE", "https://site-a.test")
	requests := 0
	oldHooks := clientHooks
	t.Cleanup(func() { clientHooks = oldHooks })
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests++
			t.Errorf("unexpected request to %s", r.URL)
			return nil, fmt.Errorf("unexpected request")
		})}
		return nil
	})
	cmd := newNovelPortfolioCmd(&rootFlags{asJSON: true, timeout: 5 * time.Second, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--sites", "http://site-a.test"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result portfolioResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Sites) != 1 || result.Sites[0].Status != "error" || !strings.Contains(result.Sites[0].Error, "https") || requests != 0 {
		t.Fatalf("result=%+v requests=%d", result, requests)
	}
}

type portfolioTestTransport struct {
	handlers map[string]http.Handler
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func (tr portfolioTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	handler := tr.handlers[r.URL.Hostname()]
	if handler == nil {
		return nil, fmt.Errorf("unexpected test host %s", r.URL.Host)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Result(), nil
}

// A redirect chain that leaves the site must never carry its credentials, even on a
// second same-host hop at the foreign host, and an https->http downgrade is refused.
func TestPortfolioRedirectGuards(t *testing.T) {
	for _, tc := range []struct {
		name        string
		hops        map[string]string // host+path -> Location
		wantFailure bool
	}{
		{"cross-host second hop", map[string]string{
			"site-a.test/wp-json/rankmath/v1/an/analyticsSummary": "https://other.test/x",
			"other.test/x": "https://other.test/y",
		}, false},
		{"downgrade", map[string]string{
			"site-a.test/wp-json/rankmath/v1/an/analyticsSummary": "http://site-a.test/plain",
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			t.Setenv("RANKMATH_SITE", "https://site-a.test")
			t.Setenv("RANKMATH_USERNAME_SITE_A_TEST", "site-user")
			t.Setenv("RANKMATH_APP_PASSWORD_SITE_A_TEST", "site-pass")
			seen := map[string]string{}
			oldHooks := clientHooks
			t.Cleanup(func() { clientHooks = oldHooks })
			registerClientHook(func(c *client.Client) error {
				c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					key := r.URL.Host + r.URL.Path
					seen[r.URL.Scheme+"://"+key] = r.Header.Get("Authorization")
					w := httptest.NewRecorder()
					if loc, ok := tc.hops[key]; ok {
						w.Header().Set("Location", loc)
						w.WriteHeader(http.StatusFound)
					} else {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte(`{"clicks":{"total":1},"impressions":{"total":1},"position":{"total":1},"ctr":{"total":1}}`))
					}
					return w.Result(), nil
				})}
				return nil
			})
			cmd := newNovelPortfolioCmd(&rootFlags{asJSON: true, timeout: 5 * time.Second, noCache: true})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--sites", "site-a.test"})
			_ = cmd.Execute()
			for url, auth := range seen {
				if !strings.HasPrefix(url, "https://site-a.test/") && auth != "" {
					t.Fatalf("credentials sent to %s: %q", url, auth)
				}
			}
			if _, ok := seen["http://site-a.test/plain"]; ok {
				t.Fatalf("followed https->http downgrade: %v", seen)
			}
			if tc.wantFailure && !strings.Contains(out.String(), `"error"`) {
				t.Fatalf("downgrade should fail the site row, got %s", out.String())
			}
		})
	}
}
