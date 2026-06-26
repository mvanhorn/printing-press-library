package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestSnapshotBrowserCDPCapturesProfile(t *testing.T) {
	cdpURL := fakeCDPServer(t, `{"url":"https://www.linkedin.com/in/michael-marcusa-90259176/","title":"Michael Marcusa | LinkedIn","text":"Michael Marcusa\nPrincipal Researcher (UX) @ Meta | Human-Centered AI Evaluation & Alignment\nAbout\nI design AI evaluation systems.\nExperience\nMeta\nPrincipal Researcher"}`)
	db := filepath.Join(t.TempDir(), "pp.db")
	out, err := captureRun(t, "snapshot", "--db", db, "--browser-cdp", "--cdp-url", cdpURL, "--target-url-match", "michael-marcusa", "--expect-name", "Michael Marcusa")
	if err != nil {
		t.Fatal(err)
	}
	var snap map[string]any
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["sections"].(float64) < 4 {
		t.Fatalf("expected structured sections, got %s", out)
	}
	if !strings.Contains(snap["source"].(string), "browser-cdp:https://www.linkedin.com/in/michael-marcusa-90259176/") {
		t.Fatalf("bad source: %s", out)
	}
}

func TestSnapshotBrowserCDPRejectsWrongExpectedName(t *testing.T) {
	cdpURL := fakeCDPServer(t, `{"url":"https://www.linkedin.com/in/other/","title":"Other Person | LinkedIn","text":"Other Person\nAbout\nAI"}`)
	db := filepath.Join(t.TempDir(), "pp.db")
	_, err := captureRun(t, "snapshot", "--db", db, "--browser-cdp", "--cdp-url", cdpURL, "--expect-name", "Michael Marcusa")
	if err == nil || !strings.Contains(err.Error(), "expected profile name") {
		t.Fatalf("expected expected-name failure, got %v", err)
	}
}

func TestSnapshotRequiresFixtureOrBrowser(t *testing.T) {
	_, err := captureRun(t, "snapshot", "--db", filepath.Join(t.TempDir(), "pp.db"))
	if err == nil || !strings.Contains(err.Error(), "--fixture or --browser-cdp") {
		t.Fatalf("expected mode error, got %v", err)
	}
}

func fakeCDPServer(t *testing.T, pagePayload string) string {
	t.Helper()
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/1"
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id":                   "1",
			"type":                 "page",
			"url":                  "https://www.linkedin.com/in/michael-marcusa-90259176/",
			"title":                "Michael Marcusa | LinkedIn",
			"webSocketDebuggerUrl": wsURL,
		}})
	})
	mux.HandleFunc("/devtools/page/1", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var msg struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected CDP method %s", msg.Method)
		}
		_ = conn.WriteJSON(map[string]any{"id": msg.ID, "result": map[string]any{"result": map[string]any{"type": "string", "value": pagePayload}}})
	})
	return server.URL
}
