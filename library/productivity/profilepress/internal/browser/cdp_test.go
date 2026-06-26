package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestListTargetsUnavailableReturnsHelpfulError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", 50*time.Millisecond)
	_, err := client.ListTargets(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list Chrome CDP targets") {
		t.Fatalf("expected helpful list error, got %v", err)
	}
}

func TestSelectTargetPrefersURLMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Target{
			{ID: "1", Type: "page", URL: "https://www.linkedin.com/feed/", WebSocketDebuggerURL: "ws://example/feed"},
			{ID: "2", Type: "page", URL: "https://www.linkedin.com/in/michael-marcusa-90259176/", WebSocketDebuggerURL: "ws://example/profile"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL, time.Second)
	target, err := client.SelectTarget(context.Background(), "https://www.linkedin.com/in/michael-marcusa-90259176/", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.ID != "2" {
		t.Fatalf("selected wrong target: %+v", target)
	}
}

func TestCapturePageTextViaFakeCDP(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var wsURL string
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	wsURL = "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/1"
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Target{{ID: "1", Type: "page", URL: "https://www.linkedin.com/in/michael-marcusa-90259176/", Title: "Michael Marcusa | LinkedIn", WebSocketDebuggerURL: wsURL}})
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
			t.Fatalf("unexpected method %s", msg.Method)
		}
		payload := `{"url":"https://www.linkedin.com/in/michael-marcusa-90259176/","title":"Michael Marcusa | LinkedIn","text":"Michael Marcusa\nAbout\nAI evaluation"}`
		_ = conn.WriteJSON(map[string]any{"id": msg.ID, "result": map[string]any{"result": map[string]any{"type": "string", "value": payload}}})
	})
	client := NewClient(server.URL, time.Second)
	page, err := client.CapturePageText(context.Background(), "", "michael-marcusa", 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Michael Marcusa | LinkedIn" || !strings.Contains(page.Text, "AI evaluation") {
		t.Fatalf("bad page: %+v", page)
	}
}
