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
