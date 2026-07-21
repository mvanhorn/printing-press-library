package cli

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func withDeliverNetworkSeams(t *testing.T, lookup func(context.Context, string) ([]net.IPAddr, error), dial func(context.Context, string, string) (net.Conn, error), tlsConfig func() *tls.Config) {
	t.Helper()
	origLookup, origDial, origTLS := deliverLookupIP, deliverDialContext, deliverTLSConfig
	deliverLookupIP, deliverDialContext, deliverTLSConfig = lookup, dial, tlsConfig
	t.Cleanup(func() { deliverLookupIP, deliverDialContext, deliverTLSConfig = origLookup, origDial, origTLS })
}

func TestDeliverWebhookIgnoresHTTPSProxy(t *testing.T) {
	var proxyHits, webhookHits int
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		http.Error(w, "proxy must not receive webhook output", http.StatusBadGateway)
	}))
	defer proxy.Close()
	webhook := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookHits++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("https_proxy", "")

	listenerAddr := webhook.Listener.Addr().String()
	clientTransport := webhook.Client().Transport.(*http.Transport)
	withDeliverNetworkSeams(t,
		func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host != "public.test" {
				t.Fatalf("resolved host = %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, listenerAddr)
		},
		func() *tls.Config {
			cfg := clientTransport.TLSClientConfig.Clone()
			cfg.ServerName = "example.com"
			return cfg
		},
	)
	if transport := newWebhookHTTPClient(time.Second).Transport.(*http.Transport); transport.Proxy != nil {
		t.Fatal("webhook transport must not use environment proxies")
	}
	if err := deliverWebhookContext(context.Background(), "https://public.test/hook", []byte("private output"), false, time.Second); err != nil {
		t.Fatalf("deliver webhook: %v", err)
	}
	if proxyHits != 0 || webhookHits != 1 {
		t.Fatalf("proxy/webhook hits = %d/%d, want 0/1", proxyHits, webhookHits)
	}
}

func TestDeliverWebhookRejectsCrossHost307And308WithoutForwardingBody(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var forwardedBody []byte
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwardedBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer target.Close()
			origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/received", status)
			}))
			defer origin.Close()

			originAddr := origin.Listener.Addr().String()
			originTransport := origin.Client().Transport.(*http.Transport)
			withDeliverNetworkSeams(t,
				func(_ context.Context, host string) ([]net.IPAddr, error) {
					if host != "origin.test" {
						t.Fatalf("resolved redirected host %q; redirect must be rejected before dialing", host)
					}
					return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
				},
				func(ctx context.Context, network, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, originAddr)
				},
				func() *tls.Config {
					cfg := originTransport.TLSClientConfig.Clone()
					cfg.ServerName = "example.com"
					return cfg
				},
			)
			err := deliverWebhookContext(context.Background(), "https://origin.test/hook", []byte("must-not-forward"), false, time.Second)
			if err == nil || !strings.Contains(err.Error(), "refusing webhook redirect") {
				t.Fatalf("redirect error = %v", err)
			}
			if len(forwardedBody) != 0 {
				t.Fatalf("307/308 forwarded webhook body %q", forwardedBody)
			}
		})
	}
}

func TestDeliverWebhookRejectsPrivateTargetsAndRedirects(t *testing.T) {
	if _, err := ParseDeliverSink("webhook:http://example.com/hook"); err == nil {
		t.Fatal("HTTP webhook must be rejected")
	}
	if err := deliverWebhookContext(context.Background(), "https://127.0.0.1/hook", nil, false, time.Second); err == nil {
		t.Fatal("loopback webhook must be rejected")
	}
	withDeliverNetworkSeams(t, func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host == "private.test" {
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.7")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}, (&net.Dialer{}).DialContext, func() *tls.Config { return nil })
	client := newWebhookHTTPClient(time.Second)
	redirect, _ := url.Parse("https://private.test/metadata")
	if err := client.CheckRedirect(&http.Request{URL: redirect}, []*http.Request{{URL: &url.URL{Scheme: "https", Host: "public.test"}}}); err == nil || !strings.Contains(err.Error(), "refusing webhook redirect") {
		t.Fatalf("redirect to private target error = %v", err)
	}
}

func TestDeliverWebhookUsesPublicResolutionAndCommandContext(t *testing.T) {
	var received string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	listenerAddr := server.Listener.Addr().String()
	clientTransport := server.Client().Transport.(*http.Transport)
	withDeliverNetworkSeams(t,
		func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host != "public.test" {
				t.Fatalf("resolved host = %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, listenerAddr)
		},
		func() *tls.Config {
			cfg := clientTransport.TLSClientConfig.Clone()
			cfg.ServerName = "example.com" // the httptest certificate's DNS name
			return cfg
		},
	)
	if err := deliverWebhookContext(context.Background(), "https://public.test/hook", []byte(`{"ok":true}`), true, time.Second); err != nil {
		t.Fatal(err)
	}
	if received != "application/x-ndjson" {
		t.Fatalf("content type = %q", received)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := deliverWebhookContext(ctx, "https://public.test/hook", nil, false, time.Second); err == nil {
		t.Fatal("cancelled delivery must fail")
	}
}

func TestDeliverFileRejectsSymlinkAndConcurrentWritesAreAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.json")
	if err := os.Symlink(filepath.Join(dir, "other"), target); err != nil {
		t.Fatal(err)
	}
	if err := deliverFile(target, []byte("bad")); err == nil {
		t.Fatal("symlink target must be rejected")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	bodies := [][]byte{[]byte(strings.Repeat("a", 8192)), []byte(strings.Repeat("b", 8192))}
	var wg sync.WaitGroup
	for _, body := range bodies {
		body := body
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := deliverFile(target, body); err != nil {
				t.Errorf("deliverFile: %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bodies[0]) && string(got) != string(bodies[1]) {
		t.Fatalf("concurrent output was partial or corrupt (%d bytes)", len(got))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".name-that-ui-deliver-") {
			t.Fatalf("temporary deliver file left behind: %s", entry.Name())
		}
	}
}
