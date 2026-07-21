package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeedbackStdinIsBoundedAndPostHonorsContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NAME_THAT_UI_HOME", home)
	cmd := RootCmd()
	cmd.SetIn(bytes.NewBufferString(strings.Repeat("x", feedbackMaxTextLen+100)))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json", "--no-learn", "feedback", "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Truncated bool          `json:"truncated"`
		Entry     FeedbackEntry `json:"entry"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Truncated || len(response.Entry.Text) != feedbackMaxTextLen {
		t.Fatalf("bounded stdin response = %#v", response)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := postFeedback(ctx, server.URL, FeedbackEntry{}, time.Second); err == nil {
		t.Fatal("cancelled feedback request must fail")
	}
}
