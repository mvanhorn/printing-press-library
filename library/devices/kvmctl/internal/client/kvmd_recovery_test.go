package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/config"
)

func TestKVMDOtgFunctionsPreservesStorageDisabledPolicy(t *testing.T) {
	var got url.Values
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/system/otg_functions" {
			http.NotFound(w, r)
			return
		}
		got = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	c := New(&config.Config{BaseURL: s.URL}, 0, 0)
	if err := c.KVMDOtgFunctions(context.Background(), true, true, true, false, false); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"enable_keyboard": "true", "enable_mouse": "true", "enable_mouse_alt": "true",
		"enable_mtp": "false", "start_cdrom": "false", "start_flash": "false",
	} {
		if got.Get(key) != want {
			t.Fatalf("%s=%q, want %q", key, got.Get(key), want)
		}
	}
}

func TestKVMDNudgeStreamerUsesSetParams(t *testing.T) {
	var got url.Values
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streamer/set_params" {
			http.NotFound(w, r)
			return
		}
		got = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	c := New(&config.Config{BaseURL: s.URL}, 0, 0)
	if err := c.KVMDNudgeStreamer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.Get("desired_fps") != "40" || got.Get("quality") != "80" {
		t.Fatalf("unexpected params: %v", got)
	}
}
