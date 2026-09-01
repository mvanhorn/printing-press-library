package semantic

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDispatchStatusReturnsStableEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"ok":true}}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	got, err := Dispatch(context.Background(), c, "status", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if json.Unmarshal(got, &out) != nil || out["operation"] != "status" || out["ok"] != true {
		t.Fatalf("%s", got)
	}
}

func TestDispatchRejectsUnknownAndWriteWithoutGate(t *testing.T) {
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1"}, 0, 0)
	if _, err := Dispatch(context.Background(), c, "nope", nil); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if _, err := Dispatch(context.Background(), c, "keyboard", map[string]any{"key": "A"}); err == nil {
		t.Fatal("write without gate accepted")
	}
}
