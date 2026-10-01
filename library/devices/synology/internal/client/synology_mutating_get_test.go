// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/synology/internal/config"
)

const dsmCopyStartPath = "/webapi/entry.cgi?api=SYNO.FileStation.CopyMove&method=start&version=3"

func TestDSMMutatingGetClassification(t *testing.T) {
	for _, path := range []string{
		dsmCopyStartPath,
		"/webapi/entry.cgi?method=stop&api=SYNO.FileStation.Delete",
		"/webapi/entry.cgi?api=SYNO.FileStation.CreateFolder&method=create",
		"/webapi/entry.cgi?api=SYNO.FileStation.Rename&method=rename",
		"/webapi/entry.cgi?api=SYNO.FileStation.Search&method=start",
		"/webapi/entry.cgi?api=SYNO.API.Auth&method=login",
		"/webapi/entry.cgi?api=SYNO.API.Auth&method=logout",
	} {
		if !isDSMMutatingGet(path) {
			t.Errorf("missed state-changing DSM GET: %s", path)
		}
	}
	for _, path := range []string{
		"/webapi/entry.cgi?api=SYNO.FileStation.CopyMove&method=status",
		"/webapi/entry.cgi?api=SYNO.Core.System&method=info",
		"/webapi/other.cgi?api=SYNO.FileStation.Delete&method=start",
	} {
		if isDSMMutatingGet(path) {
			t.Errorf("read incorrectly classified as a write: %s", path)
		}
	}
}

func TestDSMMutatingGetBypassesCacheAndInvalidatesReads(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	var reads, writes atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("api") == "SYNO.FileStation.CopyMove" {
			writes.Add(1)
			_, _ = w.Write([]byte(`{"success":true,"data":{"taskid":"synthetic"}}`))
			return
		}
		reads.Add(1)
		_, _ = w.Write([]byte(`{"success":true,"data":{"state":"synthetic"}}`))
	}))
	defer fixture.Close()
	c := New(&config.Config{BaseURL: fixture.URL}, time.Second, 0)
	c.HTTPClient = fixture.Client()
	c.Session = nil
	c.cacheDir = t.TempDir()
	readPath := "/webapi/entry.cgi?api=SYNO.Core.System&method=info&version=1"
	for i := 0; i < 2; i++ {
		if _, err := c.Get(context.Background(), readPath, nil); err != nil {
			t.Fatal(err)
		}
	}
	if reads.Load() != 1 {
		t.Fatalf("read cache did not warm: requests=%d", reads.Load())
	}
	for i := 0; i < 2; i++ {
		if _, err := c.Get(context.Background(), dsmCopyStartPath, map[string]string{"path": `["/fixture"]`}); err != nil {
			t.Fatal(err)
		}
	}
	if writes.Load() != 2 {
		t.Fatalf("GET-shaped writes were cached: requests=%d", writes.Load())
	}
	if _, err := c.Get(context.Background(), readPath, nil); err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 2 {
		t.Fatalf("mutation left stale read cache: requests=%d", reads.Load())
	}
}

func TestDSMMutatingGetDoesNotRetryServerFailure(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	var calls atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"synthetic"}`))
	}))
	defer fixture.Close()
	c := New(&config.Config{BaseURL: fixture.URL}, time.Second, 0)
	c.HTTPClient = fixture.Client()
	c.Session = nil
	if _, err := c.Get(context.Background(), dsmCopyStartPath, nil); err == nil {
		t.Fatal("expected server error")
	}
	if calls.Load() != 1 {
		t.Fatalf("ambiguous write response was retried %d times", calls.Load())
	}
}

func TestDSMMutatingGetInvalidatesExpiredSessionWithoutReplaying(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("SYNOLOGY_DATA_DIR", t.TempDir())
	t.Setenv("SYNOLOGY_ACCOUNT", "")
	t.Setenv("SYNOLOGY_PASSWORD", "")
	var writes, logins atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("api") == "SYNO.API.Auth" {
			logins.Add(1)
			_, _ = w.Write([]byte(`{"success":true,"data":{"sid":"renewed-synthetic","synotoken":"renewed-synthetic"}}`))
			return
		}
		writes.Add(1)
		if r.URL.Query().Get("_sid") == "expired-synthetic" {
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":119}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"taskid":"synthetic"}}`))
	}))
	defer fixture.Close()
	cfg := &config.Config{BaseURL: fixture.URL}
	c := New(cfg, time.Second, 0)
	c.Session.SetSession(dsmLoginResult{SID: "expired-synthetic", SynoToken: "expired-synthetic"})
	if _, err := c.Get(context.Background(), dsmCopyStartPath, nil); err == nil {
		t.Fatal("expired session should fail the first write without replay")
	}
	if writes.Load() != 1 || c.Session.Token() != "" || c.Session.SynoToken() != "" {
		t.Fatalf("expired write was replayed or session kept: writes=%d", writes.Load())
	}
	if reloaded := New(cfg, time.Second, 0); reloaded.Session.Token() != "" {
		t.Fatal("expired session remained on disk for a new CLI process")
	}
	t.Setenv("SYNOLOGY_ACCOUNT", "synthetic-user")
	t.Setenv("SYNOLOGY_PASSWORD", "synthetic-password")
	if _, err := c.Get(context.Background(), dsmCopyStartPath, nil); err != nil {
		t.Fatalf("next explicit call did not renew session: %v", err)
	}
	if writes.Load() != 2 || logins.Load() != 1 {
		t.Fatalf("next call did not renew exactly once: writes=%d logins=%d", writes.Load(), logins.Load())
	}
}

func TestDSMMutatingGetPermissionDenialKeepsSession(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("SYNOLOGY_DATA_DIR", t.TempDir())
	t.Setenv("SYNOLOGY_ACCOUNT", "")
	t.Setenv("SYNOLOGY_PASSWORD", "")
	var writes atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("_sid") != "valid-synthetic" {
			t.Errorf("request lost its existing session")
		}
		if writes.Load() == 1 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":403}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"taskid":"synthetic"}}`))
	}))
	defer fixture.Close()
	c := New(&config.Config{BaseURL: fixture.URL}, time.Second, 0)
	c.Session.SetSession(dsmLoginResult{SID: "valid-synthetic", SynoToken: "valid-synthetic"})
	if _, err := c.Get(context.Background(), dsmCopyStartPath, nil); err == nil {
		t.Fatal("expected permission denial")
	}
	if c.Session.Token() != "valid-synthetic" {
		t.Fatal("permission denial invalidated a valid session")
	}
	if _, err := c.Get(context.Background(), dsmCopyStartPath, nil); err != nil {
		t.Fatalf("subsequent call with the valid session failed: %v", err)
	}
	if writes.Load() != 2 {
		t.Fatalf("permission denial triggered a replay: writes=%d", writes.Load())
	}
}

func TestDSMMutatingGetVerifyModeNeverDials(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	c, recorder := newClientWithRecorder(t)
	data, err := c.Get(context.Background(), dsmCopyStartPath, nil)
	if err != nil || recorder.calls != 0 {
		t.Fatalf("verify-mode write dialed: calls=%d err=%v", recorder.calls, err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil || response["__pp_verify_synthetic__"] != true {
		t.Fatalf("expected verify no-op envelope: response=%v err=%v", response, err)
	}
}
