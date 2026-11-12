package diggmcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
)

const structuredReply = `{"result":{"content":[{"type":"text","text":"{\"posts\":[{\"id\":\"from-text\"}]}"}],"structuredContent":{"posts":[{"id":"from-structured"}]}},"jsonrpc":"2.0","id":1}`

func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{Endpoint: srv.URL, Timeout: 5 * time.Second, UserAgent: "digg-pp-cli/test"}
}

func firstPostID(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var out struct {
		Posts []struct {
			ID string `json:"id"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Posts) != 1 {
		t.Fatalf("result %s: err=%v, want one post", raw, err)
	}
	return out.Posts[0].ID
}

func TestCallSendsOneStatelessToolsCall(t *testing.T) {
	var got struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	var header http.Header
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, structuredReply)
	})
	if _, err := c.Call(context.Background(), "digg_search_posts", map[string]any{"query": "openai"}); err != nil {
		t.Fatal(err)
	}
	if got.JSONRPC != "2.0" || got.Method != "tools/call" || got.Params.Name != "digg_search_posts" || got.Params.Arguments["query"] != "openai" {
		t.Fatalf("request body = %+v", got)
	}
	if header.Get("Content-Type") != "application/json" || header.Get("Accept") != "application/json, text/event-stream" {
		t.Fatalf("headers = %v", header)
	}
	if header.Get("User-Agent") != "digg-pp-cli/test" {
		t.Fatalf("User-Agent = %q", header.Get("User-Agent"))
	}
	if header.Get("Mcp-Session-Id") != "" {
		t.Fatalf("a stateless call must not send a session header")
	}
}

func TestCallReadsStructuredContentFromSSE(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message\ndata: "+structuredReply+"\n\n")
	})
	raw, err := c.Call(context.Background(), "digg_search_posts", nil)
	if err != nil {
		t.Fatal(err)
	}
	if id := firstPostID(t, raw); id != "from-structured" {
		t.Fatalf("post id = %q, want the structuredContent object", id)
	}
}

func TestCallReadsPlainJSON(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, structuredReply)
	})
	raw, err := c.Call(context.Background(), "digg_search_posts", nil)
	if err != nil {
		t.Fatal(err)
	}
	if id := firstPostID(t, raw); id != "from-structured" {
		t.Fatalf("post id = %q, want the structuredContent object", id)
	}
}

func TestCallFallsBackToTextContent(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"posts\":[{\"id\":\"from-text\"}]}"}]}}`)
	})
	raw, err := c.Call(context.Background(), "digg_search_posts", nil)
	if err != nil {
		t.Fatal(err)
	}
	if id := firstPostID(t, raw); id != "from-text" {
		t.Fatalf("post id = %q, want the text content parsed as JSON", id)
	}
}

func TestCallReturnsJSONRPCError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Unknown tool: digg_nope"}}`)
	})
	_, err := c.Call(context.Background(), "digg_nope", nil)
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if e.Tool != "digg_nope" || e.Code != -32602 || !strings.Contains(err.Error(), "digg_nope") || !strings.Contains(err.Error(), "Unknown tool") {
		t.Fatalf("error = %+v (%q), want the tool name and the server's message", e, err.Error())
	}
}

func TestCallReturnsToolErrorResult(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"search is unavailable"}]}}`)
	})
	raw, err := c.Call(context.Background(), "digg_search_posts", nil)
	var e *Error
	if !errors.As(err, &e) || raw != nil {
		t.Fatalf("raw=%s err=%v, want *Error and no result", raw, err)
	}
	if !strings.Contains(err.Error(), "search is unavailable") {
		t.Fatalf("error %q should carry the tool's message", err.Error())
	}
}

func TestCallRetriesOnceAfterRetryAfter(t *testing.T) {
	var hits atomic.Int32
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, structuredReply)
	})
	start := time.Now()
	raw, err := c.Call(context.Background(), "digg_search_posts", nil)
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("server saw %d requests, want 2", hits.Load())
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("retried after %s, want the 1s Retry-After honored", elapsed)
	}
	if id := firstPostID(t, raw); id != "from-structured" {
		t.Fatalf("post id = %q", id)
	}
}

func TestCallReportsTimeout(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	})
	c.Timeout = 50 * time.Millisecond
	_, err := c.Call(context.Background(), "digg_search_posts", nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want an error saying the call timed out", err)
	}
}

// The body is what Digg's server returned for a two-character query,
// saved unmodified: an HTTP 200 whose result is flagged isError.
func TestUnwrapRecordedValidationError(t *testing.T) {
	raw, err := os.ReadFile("testdata/invalid_query_response.sse")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Unwrap("digg_search_posts", raw)
	var e *Error
	if !errors.As(err, &e) || out != nil {
		t.Fatalf("out=%s err=%v, want *Error and no result", out, err)
	}
	if e.Tool != "digg_search_posts" || !strings.Contains(e.Message, "expected string to have >=3 characters") {
		t.Fatalf("error = %+v, want Digg's validation message", e)
	}
}

// A caller with two seconds left cannot use a retry that starts in thirty:
// it gets the rate-limit error at once instead of a timeout later.
func TestCallReturnsRateLimitWhenRetryAfterOutlastsDeadline(t *testing.T) {
	var hits atomic.Int32
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	_, err := c.Call(ctx, "digg_search_posts", nil)
	elapsed := time.Since(start)
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want a rate-limit error", err)
	}
	if rl.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %s, want 30s", rl.RetryAfter)
	}
	if elapsed > time.Second {
		t.Fatalf("returned after %s, want an immediate return", elapsed)
	}
	if hits.Load() != 1 {
		t.Fatalf("server saw %d requests, want 1", hits.Load())
	}
}
