package cli

import (
	"context"
	"encoding/pem"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/platform"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reviewSyncFlags(t *testing.T) *rootFlags {
	t.Helper()
	testenv.Isolate(t)
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte("api_token = \"fixture\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return &rootFlags{configPath: cfg, timeout: time.Second, timeoutExplicit: true}
}
func TestReviewSyncClientTLS(t *testing.T) {
	f := reviewSyncFlags(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"ok":true}`) }))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	defer srv.Close()
	t.Setenv("READWISE_BASE_URL", srv.URL)
	f.caCert = filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(f.caCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	f.tlsServerName = "example.com"
	c, err := newReadwiseSyncClient(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	paced, ok := c.(*fixedPacedClient)
	if !ok {
		t.Fatalf("unexpected sync client %T", c)
	}
	inner, ok := paced.inner.(*client.Client)
	if !ok {
		t.Fatalf("unexpected inner %T", paced.inner)
	}
	transport, ok := inner.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport %T", inner.HTTPClient.Transport)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.ServerName != "example.com" {
		t.Fatalf("TLS server name ignored: %+v", transport.TLSClientConfig)
	}
	if _, err := c.Get(context.Background(), "/", nil); err != nil {
		t.Fatalf("CA and server name flags ignored: %v", err)
	}
	f.caCert = filepath.Join(t.TempDir(), "missing.pem")
	if _, err := newReadwiseSyncClient(f, 0); err == nil {
		t.Fatal("invalid CA accepted")
	}
}
func TestReviewSyncClientSetup(t *testing.T) {
	f := reviewSyncFlags(t)
	oldHooks := clientHooks
	defer func() { clientHooks = oldHooks }()
	sentinel := errors.New("fixture hook failure")
	clientHooks = []func(*client.Client) error{func(c *client.Client) error {
		if c.HTTPClient.Timeout != f.timeout || !c.NoCache {
			t.Errorf("sync setup missing timeout/cache: %+v", c)
		}
		return sentinel
	}}
	if _, err := newReadwiseSyncClient(f, 3); !errors.Is(err, sentinel) {
		t.Fatalf("sync skipped shared hooks: %v", err)
	}
	clientHooks = nil
	oldSource := registeredPlatformSource
	defer func() { registeredPlatformSource = oldSource }()
	bound := false
	registeredPlatformSource = &platformSourceRegistration{BindClient: func(*client.Client, *platform.Session) error { bound = true; return nil }}
	if _, err := newReadwiseSyncClient(f, 3); err == nil || !strings.Contains(err.Error(), "verified") {
		t.Fatalf("sync bypassed platform gate: %v", err)
	}
	f.platformSession = &platform.Session{Paths: platform.Paths{CacheDir: t.TempDir()}}
	if _, err := newReadwiseSyncClient(f, 3); err == nil {
		t.Fatal("unverified session accepted")
	}
	f.platformSession.GateOutcome = platform.GateVerified
	c, err := newReadwiseSyncClient(f, 3)
	if err != nil || !bound || c.RateLimit() != 3 {
		t.Fatalf("verified binding/pacing: %v bound=%v", err, bound)
	}
}
